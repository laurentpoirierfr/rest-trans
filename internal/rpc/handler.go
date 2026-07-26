package rpc

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	apierror "github.com/laurentpoirierfr/rest-trans/internal/error"
	"github.com/laurentpoirierfr/rest-trans/internal/schema"
)

type Handler struct {
	DB    *sql.DB
	Store *schema.SchemaStore
}

func NewHandler(db *sql.DB, store *schema.SchemaStore) *Handler {
	return &Handler{DB: db, Store: store}
}

func (h *Handler) HandleRPC(c *gin.Context) {
	funcName := c.Param("function")

	schemaName := c.Param("schema")
	if schemaName == "" {
		if p := c.GetHeader("Content-Profile"); p != "" {
			schemaName = p
		} else if p := c.GetHeader("Accept-Profile"); p != "" {
			schemaName = p
		}
	}
	if schemaName == "" {
		schemaName = h.Store.DefaultSchema()
	}

	fn, ok := h.Store.GetFunction(schemaName, funcName)
	if !ok {
		c.JSON(http.StatusNotFound, apierror.Error{
			Code:    "PGRST205",
			Message: fmt.Sprintf("Function '%s' not found", funcName),
			Status:  http.StatusNotFound,
		})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody(err.Error()))
		return
	}
	defer c.Request.Body.Close()

	var args map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &args); err != nil {
			c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody(err.Error()))
			return
		}
	}

	callArgs := make([]interface{}, 0, len(fn.Arguments))
	placeholders := make([]string, 0, len(fn.Arguments))

	for i, arg := range fn.Arguments {
		if val, exists := args[arg.Name]; exists {
			callArgs = append(callArgs, val)
		} else {
			callArgs = append(callArgs, nil)
		}
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	qualified := funcName
	if schemaName != "" && schemaName != "public" {
		qualified = schemaName + "." + funcName
	}

	sqlQuery := fmt.Sprintf("SELECT * FROM %s(%s)", qualified, strings.Join(placeholders, ", "))

	rows, err := h.DB.Query(sqlQuery, callArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.ErrDatabase(err.Error()))
		return
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.ErrDatabase(err.Error()))
		return
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			c.JSON(http.StatusInternalServerError, apierror.ErrDatabase(err.Error()))
			return
		}

		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				var jsonVal interface{}
				if err := json.Unmarshal(b, &jsonVal); err == nil {
					row[col] = jsonVal
				} else {
					row[col] = string(b)
				}
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if len(results) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	c.JSON(http.StatusOK, results)
}
