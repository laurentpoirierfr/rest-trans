package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/laurentpoirierfr/rest-trans/internal/notification"
)

func HandleSSE(hub *notification.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		tableName := c.Param("table")
		schemaName := c.Param("schema")

		if schemaName == "" {
			schemaName = "public"
		}

		channel := "rest_" + schemaName + "_" + tableName

		client := hub.Subscribe(channel)
		defer hub.Unsubscribe(channel, client)

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
			return
		}

		fmt.Fprintf(c.Writer, "event: connected\ndata: {\"channel\":\"%s\"}\n\n", channel)
		flusher.Flush()

		c.Done()

		for {
			select {
			case <-c.Request.Context().Done():
				return
			case <-client.Done():
				return
			case event := <-client.C:
				fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", tableName, event.Payload)
				flusher.Flush()
			}
		}
	}
}
