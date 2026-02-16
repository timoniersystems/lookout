package dgraph

import (
	"context"
	"time"

	"github.com/timoniersystems/lookout/pkg/logging"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func SetupAndRunDgraph() {
	manager := GetGlobalClientManager()

	if err := manager.Connect(); err != nil {
		logging.Error("Failed to connect to Dgraph: %v", err)
		panic(err)
	}

	client, err := manager.GetClient()
	if err != nil {
		logging.Error("Failed to get Dgraph client: %v", err)
		panic(err)
	}

	if err := SetupSchema(client); err != nil {
		logging.Error("Failed to setup schema: %v", err)
		panic(err)
	}
}

// SetupSchema configures the Dgraph schema for the application.
// It should be called after dropping all data to ensure schema consistency.
func SetupSchema(client *dgo.Dgraph) error {
	ctx := context.Background()
	return setupSchemaInternal(client, ctx)
}

func setupSchemaInternal(client *dgo.Dgraph, ctx context.Context) error {
	schema := `
		bomRef: string @index(exact) .
        reference: string @index(exact) .
        name: string @index(exact) .
        version: string @index(exact) .
        purl: string @index(exact) @upsert .
        dependsOn: [uid] @reverse .
		vulnerable: bool @index(bool) .
        cveID: string @index(exact) .
        dgraphURL: string @index(exact) .
        root: bool @index(bool) .

        type Component {
			bomRef
			reference
            name
            version
            purl
            dependsOn
			vulnerable
            cveID
            dgraphURL
            root
        }
    `

	op := &api.Operation{Schema: schema}
	err := client.Alter(ctx, op)
	if err != nil {
		if s, ok := status.FromError(err); ok && (s.Code() == codes.Unavailable || s.Code() == codes.Unknown) {
			logging.Warn("Server not ready, unable to alter schema. Error: %v", err)
			time.Sleep(5 * time.Second)
			return setupSchemaInternal(client, ctx)
		} else {
			logging.Error("Failed to alter schema: %v", err)
			return err
		}
	}
	logging.Info("Schema setup successful")
	return nil
}
