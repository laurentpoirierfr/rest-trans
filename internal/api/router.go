package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/laurentpoirierfr/rest-trans/internal/config"
	"github.com/laurentpoirierfr/rest-trans/internal/docs"
	"github.com/laurentpoirierfr/rest-trans/internal/openapi"
	"github.com/laurentpoirierfr/rest-trans/internal/rpc"
	"github.com/laurentpoirierfr/rest-trans/internal/schema"
	"github.com/laurentpoirierfr/rest-trans/internal/transaction"
)

func NewRouter(db *sql.DB, store *schema.SchemaStore, schemas []string, cfg *config.Config, txManager *transaction.Manager) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(loggingMiddleware())
	r.Use(corsMiddleware())
	r.Use(methodGuard(store, cfg))
	if txManager != nil {
		r.Use(transaction.Middleware(txManager))
	}

	h := NewHandler(db, store)
	rpcH := rpc.NewHandler(db, store)

	r.GET("/info", func(c *gin.Context) {
		tables := make(map[string][]string)
		for _, sName := range store.SchemaNames() {
			tablesBySchema := store.TablesBySchema(sName)
			names := make([]string, 0, len(tablesBySchema))
			for name := range tablesBySchema {
				names = append(names, name)
			}
			tables[sName] = names
		}
		c.JSON(http.StatusOK, gin.H{
			"version": "1.0.0",
			"schema":  schemas,
			"tables":  tables,
		})
	})

	r.GET("/openapi.json", func(c *gin.Context) {
		spec := openapi.Generate(store, cfg)
		c.JSON(http.StatusOK, spec)
	})

	docs.RegisterRoutes(r)

	r.POST("/:schema/rpc/:function", rpcH.HandleRPC)

	r.GET("/:schema/:table", h.HandleGet)
	r.HEAD("/:schema/:table", h.HandleHead)
	r.POST("/:schema/:table", h.HandlePost)
	r.PUT("/:schema/:table", h.HandlePut)
	r.PATCH("/:schema/:table", h.HandlePatch)
	r.DELETE("/:schema/:table", h.HandleDelete)
	r.OPTIONS("/:schema/:table", h.HandleOptions)

	if txManager != nil {
		txH := transaction.NewHandler(txManager)
		r.POST("/:schema/transactions", txH.Start)
		r.GET("/:schema/transactions", txH.List)
		r.GET("/:schema/transactions/:txID", txH.Get)
		r.POST("/:schema/transactions/:txID/commit", txH.Commit)
		r.POST("/:schema/transactions/:txID/rollback", txH.Rollback)
	}

	return r
}

func methodGuard(store *schema.SchemaStore, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tableName := c.Param("table")
		if tableName == "" {
			c.Next()
			return
		}

		schemaName := c.Param("schema")
		method := c.Request.Method

		if t, ok := store.GetTable(schemaName, tableName); ok {
			if !t.IsMethodAllowed(method) {
				c.AbortWithStatusJSON(405, gin.H{
					"code":    "PGRST102",
					"message": fmt.Sprintf("Method %s is not allowed for %s.%s", method, schemaName, tableName),
					"details": "Check @allow/@deny comment on table or config permissions",
				})
				return
			}
		}

		if !cfg.IsMethodAllowed(tableName, method) {
			c.AbortWithStatusJSON(405, gin.H{
				"code":    "PGRST102",
				"message": fmt.Sprintf("Method %s is not allowed for %s.%s (config)", method, schemaName, tableName),
			})
			return
		}

		c.Next()
	}
}

func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		log.Printf("[%s] %s %s -> %d",
			c.Request.Method,
			c.Request.URL.Path,
			c.Request.URL.RawQuery,
			c.Writer.Status(),
		)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, HEAD, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, Prefer, Range, Accept-Profile, Content-Profile, Authorization-Transaction")
		c.Header("Access-Control-Expose-Headers", "Content-Range, Range-Unit, X-Total-Count, Content-Profile")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func containsSubstr(s, substr string) bool {
	return strings.Contains(s, substr)
}
