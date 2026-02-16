package dgraph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/timoniersystems/lookout/pkg/logging"

	"github.com/dgraph-io/dgo/v210/protos/api"
)

type PurlData struct {
	Purl      string
	CveID     string
	DgraphURL string
}

func QueryAndUpdatePurl(cvePurlMap map[string]string) error {
	client := DgraphClient()
	ctx := context.Background()

	for cveID, purl := range cvePurlMap {
		query := `
			query PurlQuery($purl: string) {
				purlData(func: eq(purl, $purl)) {
					uid
					purl
				}
			}
		`

		vars := map[string]string{"$purl": purl}

		txn := client.NewTxn()
		defer txn.Discard(ctx)

		resp, err := txn.QueryWithVars(ctx, query, vars)
		if err != nil {
			return fmt.Errorf("query failed: %w", err)
		}

		type PurlQueryResponse struct {
			PurlData []struct {
				UID  string `json:"uid"`
				Purl string `json:"purl"`
			} `json:"purlData"`
		}

		var purlQueryResp PurlQueryResponse
		err = json.Unmarshal(resp.Json, &purlQueryResp)
		if err != nil {
			return fmt.Errorf("unmarshal query response failed: %w", err)
		}

		if len(purlQueryResp.PurlData) == 0 {
			logging.Debug("No purl found for %s", purl)
			continue
		}

		dgraphURL, err := GenerateQueryURL(purl)
		if err != nil {
			return fmt.Errorf("failed to generate dgraphURL for %s: %w", purl, err)
		}

		mu := &api.Mutation{
			CommitNow: true,
		}

		updateData := fmt.Sprintf(`
			{
				"uid": "%s",
				"cveID": "%s",
				"dgraphURL": "%s",
				"vulnerable": true
			}
		`, purlQueryResp.PurlData[0].UID, cveID, dgraphURL)

		mu.SetJson = []byte(updateData)

		_, err = txn.Mutate(ctx, mu)
		if err != nil {
			return fmt.Errorf("mutation failed: %w", err)
		}

		logging.Info("Purl data for %s updated successfully", purl)
	}

	return nil
}
