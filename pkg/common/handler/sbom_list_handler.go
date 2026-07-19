package handler

import (
	"net/http"

	"github.com/timoniersystems/lookout/pkg/logging"
	"github.com/timoniersystems/lookout/pkg/ui/dgraph"

	"github.com/labstack/echo/v4"
)

// ListSBOMs enumerates the images whose SBOMs have been ingested into the
// persistent store (lookout#56). It backs the transwarp -> Lookout feed
// verification (eagle-valley#1049): a client can confirm every image the pipeline
// pushed is present, keyed by name + digest. Read-only; degrades gracefully when
// Dgraph is unavailable rather than 500ing the whole endpoint.
func ListSBOMs(c echo.Context) error {
	client, err := dgraph.GetGlobalClientManager().GetClient()
	if err != nil {
		logging.Warn("ListSBOMs: Dgraph unavailable: %v", err)
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"error": "SBOM store unavailable",
		})
	}

	images, err := dgraph.ListImages(client)
	if err != nil {
		logging.Error("ListSBOMs: failed to list images: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to list SBOMs",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"count":  len(images),
		"images": images,
	})
}
