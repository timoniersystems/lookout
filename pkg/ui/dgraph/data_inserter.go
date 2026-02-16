package dgraph

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/timoniersystems/lookout/pkg/common/cyclonedx"
	"github.com/timoniersystems/lookout/pkg/logging"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
)

// validPercentEncoding matches valid percent-encoded sequences (%XX where XX are hex digits)
var validPercentEncoding = regexp.MustCompile(`%[0-9A-Fa-f]{2}`)

// looksURLEncoded checks if a string appears to contain URL percent-encoding.
// It returns true if the string contains valid percent-encoded sequences like %20, %3A, etc.
func looksURLEncoded(s string) bool {
	// Must contain at least one percent sign
	if !strings.Contains(s, "%") {
		return false
	}

	// Check if it contains valid percent-encoded sequences
	return validPercentEncoding.MatchString(s)
}

func EncodeNodeID(id string) string {
	decodedID := id

	// Only attempt to decode if it looks URL-encoded
	if looksURLEncoded(id) {
		if unescaped, err := url.QueryUnescape(id); err == nil {
			decodedID = unescaped
			logging.Debug("URL-decoded ID: %s -> %s", id, decodedID)
		} else {
			logging.Debug("Failed to decode URL-encoded ID: %v", err)
		}
	}

	return base64.URLEncoding.EncodeToString([]byte(decodedID))
}

func encodePURL(purl string) string {
	decodedPURL := purl

	// Only attempt to decode if it looks URL-encoded
	if looksURLEncoded(purl) {
		if unescaped, err := url.QueryUnescape(purl); err == nil {
			decodedPURL = unescaped
			logging.Debug("URL-decoded PURL: %s -> %s", purl, decodedPURL)
		} else {
			logging.Debug("Failed to decode URL-encoded PURL: %v", err)
		}
	}

	return base64.URLEncoding.EncodeToString([]byte(decodedPURL))
}

func decodePURL(encoded string) string {
	decoded, _ := base64.URLEncoding.DecodeString(encoded)
	return string(decoded)
}

func extractName(purl string) string {
	parts := strings.Split(purl, "@")
	if len(parts) == 2 {
		segments := strings.Split(parts[0], "/")
		return fmt.Sprintf("%s@%s", segments[len(segments)-1], parts[1])
	}
	return purl
}

func RetrieveExistingComponents(txn *dgo.Txn) (map[string]string, error) {
	query := `
    {
        components(func: has(bomRef)) {
            uid
            bomRef
        }
    }`

	resp, err := txn.Query(context.Background(), query)
	if err != nil {
		logging.Error("Failed to query existing components: %v", err)
		return nil, err
	}

	type Component struct {
		UID    string `json:"uid"`
		BomRef string `json:"bomRef"`
	}

	var result struct {
		Components []Component `json:"components"`
	}

	if err := json.Unmarshal(resp.Json, &result); err != nil {
		logging.Error("Failed to unmarshal response: %v", err)
		return nil, err
	}

	existingComponents := make(map[string]string)
	for _, comp := range result.Components {
		existingComponents[comp.BomRef] = comp.UID
	}

	return existingComponents, nil
}

func InsertComponentsAndDependencies(client *dgo.Dgraph, bom *cyclonedx.Bom) error {
	ctx := context.Background()
	txn := client.NewTxn()
	defer txn.Discard(ctx)

	existingComponents, err := RetrieveExistingComponents(txn)
	if err != nil {
		logging.Error("Failed to retrieve existing components: %v", err)
		return err
	}

	rootComponent := bom.Metadata.Component
	// fmt.Println("HERE IS THE rootComponent: ", rootComponent)  // Debug output
	rootComponent.Purl = rootComponent.Name + "@root"
	componentIDs := make(map[string]string)

	bom.Components = append(bom.Components, rootComponent)

	dgraphQuery := buildDependencyTree(bom, rootComponent.BomRef, existingComponents, componentIDs)

	// log.Printf("Prepared Dgraph Mutation Query:\n%s", dgraphQuery)  // Debug output

	mu := &api.Mutation{
		SetNquads: []byte(dgraphQuery),
		CommitNow: true,
	}

	if _, err := txn.Mutate(ctx, mu); err != nil {
		logging.Error("Failed to create components and dependencies: %v", err)
		return err
	}

	return nil
}

func buildDependencyTree(bom *cyclonedx.Bom, rootComponent string, existingComponents map[string]string, componentIDs map[string]string) string {
	var dgraphQuery strings.Builder

	for _, comp := range bom.Components {
		compID, exists := existingComponents[comp.BomRef]
		if !exists {
			compID = "_:comp_" + EncodeNodeID(comp.BomRef)
		}
		componentIDs[comp.BomRef] = compID

		// fmt.Println("The buildDependencyTree's compID is: ", compID)  // Debug output

		var encodedPurl, decodedPurl string
		if comp.Purl != "" {
			decodedPurl = decodePURL(comp.Purl)
			encodedPurl = encodePURL(decodedPurl)
		}

		name := extractName(comp.Purl)
		reference := comp.Name
		vulnerable := "false"
		root := "false"

		fmt.Fprintf(&dgraphQuery, `<%s> <reference> "%s" .
        <%s> <name> "%s" .
        <%s> <version> "%s" .
        <%s> <purl> "%s" .
        <%s> <base64PURL> "%s" .
        <%s> <decodedPURL> "%s" .
        <%s> <bomRef> "%s" .
        <%s> <vulnerable> "%s" .
        <%s> <root> "%s" .
        <%s> <dgraph.type> "Component" .
        `, compID, reference, compID, name, compID, comp.Version, compID, comp.Purl, compID, encodedPurl, compID, decodedPurl, compID, comp.BomRef, compID, vulnerable, compID, root, compID)
	}

	var traverse func(string, bool)
	traverse = func(componentRef string, isRoot bool) {
		comp := findComponentByRef(bom.Components, componentRef)
		if comp == nil {
			return
		}

		compID := componentIDs[comp.BomRef]

		root := "false"
		if isRoot {
			root = "true"
		}

		fmt.Fprintf(&dgraphQuery, `<%s> <root> "%s" .`, compID, root)
		dgraphQuery.WriteString("\n")
		for _, depRef := range bom.Dependencies {
			if depRef.Ref == componentRef {
				for _, depOnRef := range depRef.DependsOn {
					depID := componentIDs[depOnRef]
					// log.Printf("Logged depIDS: %s", depID)  // Debug output
					fmt.Fprintf(&dgraphQuery, `<%s> <dependsOn> <%s> .`, compID, depID)
					dgraphQuery.WriteString("\n")
					traverse(depOnRef, false)
				}
			}
		}
	}

	// log.Printf("Root component is: %s", rootComponent)  // Debug output
	traverse(rootComponent, true)
	return dgraphQuery.String()
}

func findComponentByRef(components []cyclonedx.Component, ref string) *cyclonedx.Component {
	for _, comp := range components {
		if comp.BomRef == ref {
			return &comp
		}
	}
	return nil
}
