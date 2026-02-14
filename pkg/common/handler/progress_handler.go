package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"lookout/pkg/common/progress"
	"net/http"

	"github.com/labstack/echo/v4"
)

// ProgressSSE streams progress updates via Server-Sent Events
func ProgressSSE(c echo.Context) error {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Session ID required",
		})
	}

	tracker := progress.GetTracker(sessionID)
	if tracker == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Session not found",
		})
	}

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	c.Response().WriteHeader(http.StatusOK)

	// Send updates as they come
	for update := range tracker.Updates {
		data, err := json.Marshal(update)
		if err != nil {
			log.Printf("Failed to marshal progress update: %v", err)
			continue
		}

		// SSE format: "data: <json>\n\n"
		if _, err := fmt.Fprintf(c.Response().Writer, "data: %s\n\n", data); err != nil {
			log.Printf("Failed to write SSE data: %v", err)
			break
		}

		c.Response().Flush()

		// If it's a complete or error message, we're done
		if update.Type == "complete" || update.Type == "error" {
			break
		}
	}

	return nil
}
