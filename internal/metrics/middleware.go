package metrics

import (
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/laurentpoirierfr/rest-trans/internal/schema"
)

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		method := c.Request.Method

		InflightRequests.WithLabelValues(method).Inc()

		c.Next()

		InflightRequests.WithLabelValues(method).Dec()

		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		RequestsTotal.WithLabelValues(method, path, status).Inc()
		RequestDuration.WithLabelValues(method, path).Observe(duration)

		if c.Writer.Status() >= 400 {
			ErrorsTotal.WithLabelValues(method, path, status).Inc()
		}
	}
}

func LivenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func ReadinessHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "down",
				"error":  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func StreamsHandler(store *schema.SchemaStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := make(map[string][]string)
		for _, schemaName := range store.SchemaNames() {
			tables := store.TablesBySchema(schemaName)
			names := make([]string, 0, len(tables))
			for name := range tables {
				names = append(names, name)
			}
			sort.Strings(names)
			result[schemaName] = names
		}
		c.JSON(http.StatusOK, result)
	}
}
