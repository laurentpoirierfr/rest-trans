package transaction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

const HeaderTransaction = "Authorization-Transaction"

func Middleware(manager *Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		txID := c.GetHeader(HeaderTransaction)
		if txID == "" {
			c.Next()
			return
		}

		method := c.Request.Method
		if method != http.MethodPost && method != http.MethodPut &&
			method != http.MethodPatch && method != http.MethodDelete {
			c.Next()
			return
		}

		tx, err := manager.Get(txID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code":    "PGRST310",
				"message": fmt.Sprintf("Failed to validate transaction: %v", err),
			})
			return
		}
		if tx == nil || tx.Status != StatusPending {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"code":    "PGRST311",
				"message": "Transaction not found or not pending",
			})
			return
		}

		tableName := c.Param("table")
		schemaName := c.Param("schema")
		if schemaName == "" {
			schemaName = "public"
		}
		qualifiedTable := schemaName + "." + tableName

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"code":    "PGRST312",
				"message": "Failed to read request body",
			})
			return
		}
		c.Request.Body.Close()

		var bodyData interface{}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &bodyData); err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"code":    "PGRST313",
					"message": "Invalid JSON body",
				})
				return
			}
		}

		var items []map[string]interface{}
		switch v := bodyData.(type) {
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					items = append(items, m)
				}
			}
		case map[string]interface{}:
			items = []map[string]interface{}{v}
		default:
			items = []map[string]interface{}{}
		}

		var op string
		var sqlQuery string
		var params []interface{}

		switch method {
		case http.MethodPost:
			op = "INSERT"
			for _, item := range items {
				columns, placeholders, vals := extractInsertParts(item)
				sqlQuery = BuildInsertQuery(qualifiedTable, columns, placeholders)
				params = vals
				if err := manager.Stage(txID, op, qualifiedTable, sqlQuery, params); err != nil {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
						"code":    "PGRST314",
						"message": fmt.Sprintf("Failed to stage operation: %v", err),
					})
					return
				}
			}

		case http.MethodPut, http.MethodPatch:
			op = "UPDATE"
			if len(items) > 0 {
				setClauses, placeholders, vals := extractUpdateParts(items[0])
				sqlQuery = BuildUpdateQuery(qualifiedTable, setClauses, placeholders)
				params = vals
				if err := manager.Stage(txID, op, qualifiedTable, sqlQuery, params); err != nil {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
						"code":    "PGRST315",
						"message": fmt.Sprintf("Failed to stage operation: %v", err),
					})
					return
				}
			}

		case http.MethodDelete:
			op = "DELETE"
			sqlQuery = BuildDeleteQuery(qualifiedTable)
			if pkVal := c.Query("id"); pkVal != "" {
				params = []interface{}{pkVal}
			}
			if err := manager.Stage(txID, op, qualifiedTable, sqlQuery, params); err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    "PGRST316",
					"message": fmt.Sprintf("Failed to stage operation: %v", err),
				})
				return
			}
		}

		c.JSON(http.StatusAccepted, StagedResponse{
			Status: "pending",
			Tx:     txID,
		})
		c.Abort()
	}
}

func extractInsertParts(item map[string]interface{}) (columns, placeholders []string, vals []interface{}) {
	i := 1
	for k, v := range item {
		columns = append(columns, k)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		vals = append(vals, v)
		i++
	}
	return
}

func extractUpdateParts(item map[string]interface{}) (setClauses, placeholders []string, vals []interface{}) {
	i := 1
	for k, v := range item {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, i))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		vals = append(vals, v)
		i++
	}
	return
}

func BuildBodyForStaging(c *gin.Context) *bytes.Buffer {
	var buf bytes.Buffer
	if c.Request.Body != nil {
		buf.ReadFrom(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
	}
	return &buf
}
