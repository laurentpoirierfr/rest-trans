package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	apierror "github.com/laurentpoirierfr/rest-trans/internal/error"
	"github.com/laurentpoirierfr/rest-trans/internal/query"
	"github.com/laurentpoirierfr/rest-trans/internal/response"
	"github.com/laurentpoirierfr/rest-trans/internal/schema"
)

type Handler struct {
	DB    *sql.DB
	Store *schema.SchemaStore
}

func NewHandler(db *sql.DB, store *schema.SchemaStore) *Handler {
	return &Handler{DB: db, Store: store}
}

func (h *Handler) resolveTable(c *gin.Context) (*schema.Table, string, string, bool) {
	tableName := c.Param("table")
	schemaName := c.Param("schema")

	if schemaName == "" {
		if p := c.GetHeader("Content-Profile"); p != "" {
			schemaName = p
		} else if p := c.GetHeader("Accept-Profile"); p != "" {
			schemaName = p
		}
	}

	table, ok := h.Store.GetTable(schemaName, tableName)
	return table, tableName, schemaName, ok
}

func (h *Handler) HandleGet(c *gin.Context) {
	table, tableName, _, ok := h.resolveTable(c)
	if !ok {
		c.JSON(http.StatusNotFound, apierror.ErrTableNotFound(tableName))
		return
	}

	params, err := h.parseParams(c, table)
	if err != nil {
		apiErr := apierror.ErrInvalidFilter(err.Error())
		c.JSON(apiErr.Status, apiErr)
		return
	}

	if rLimit, rOffset, ok := response.ParseRangeHeader(c.Request); ok {
		if params.Range.Limit < 0 {
			params.Range.Limit = rLimit
		}
		if params.Range.Offset == 0 && rOffset > 0 {
			params.Range.Offset = rOffset
		}
	}

	if params.Range.Limit < 0 {
		params.Range.Limit = 1000
	}

	countTotal := 0
	if params.HasCount() {
		countTotal, err = h.executeCount(table, params)
		if err != nil {
			c.JSON(http.StatusInternalServerError, apierror.ErrDatabase(err.Error()))
			return
		}
	}

	builder := query.NewBuilderWithStore(table, params, table.SchemaName, h.Store)
	sqlQuery, args, err := builder.BuildSelect()
	if err != nil {
		apiErr := apierror.ErrInvalidFilter(err.Error())
		c.JSON(apiErr.Status, apiErr)
		return
	}

	rows, err := h.DB.Query(sqlQuery, args...)
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

	if params.HasCount() && countTotal > 0 {
		response.SetRangeHeaders(c, params.Range.Limit, params.Range.Offset, countTotal)
	}

	accept := c.GetHeader("Accept")

	if strings.Contains(accept, "text/csv") {
		response.WriteCSV(c, rows, cols)
		return
	}

	if strings.Contains(accept, "application/vnd.pgrst.object+json") {
		h.writeSingular(c, rows, cols)
		return
	}

	response.WriteJSON(c, rows, cols, nil)
}

func (h *Handler) HandleHead(c *gin.Context) {
	table, _, _, ok := h.resolveTable(c)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}

	params, err := h.parseParams(c, table)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	countTotal, err := h.executeCount(table, params)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	if params.Range.Limit < 0 {
		params.Range.Limit = 1000
	}

	response.SetRangeHeaders(c, params.Range.Limit, params.Range.Offset, countTotal)
	c.Status(http.StatusOK)
}

func (h *Handler) HandlePost(c *gin.Context) {
	table, tableName, _, ok := h.resolveTable(c)
	if !ok {
		c.JSON(http.StatusNotFound, apierror.ErrTableNotFound(tableName))
		return
	}

	if table.IsView {
		c.JSON(http.StatusBadRequest, apierror.ErrMethodNotAllowed("Cannot insert into a view"))
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody(err.Error()))
		return
	}
	defer c.Request.Body.Close()

	if len(body) == 0 {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody("Empty request body"))
		return
	}

	prefer := parsePrefer(c.GetHeader("Prefer"))

	var rowsData []map[string]interface{}
	if body[0] == '[' {
		if err := json.Unmarshal(body, &rowsData); err != nil {
			c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody(err.Error()))
			return
		}
	} else {
		var row map[string]interface{}
		if err := json.Unmarshal(body, &row); err != nil {
			c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody(err.Error()))
			return
		}
		rowsData = []map[string]interface{}{row}
	}

	if len(rowsData) == 0 {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody("Empty array"))
		return
	}

	var columns []string
	for col := range rowsData[0] {
		if _, ok := table.Columns[col]; !ok {
			c.JSON(http.StatusBadRequest, apierror.ErrInvalidColumn(col))
			return
		}
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody("No valid columns"))
		return
	}

	argIdx := 1
	var allArgs []interface{}
	for _, row := range rowsData {
		for _, col := range columns {
			val, exists := row[col]
			if !exists {
				allArgs = append(allArgs, nil)
			} else {
				allArgs = append(allArgs, fmtVal(val))
			}
			argIdx++
		}
	}

	// Build base INSERT
	var valuePlaceholders []string
	argIdx = 1
	for range rowsData {
		rowPH := make([]string, len(columns))
		for i := range columns {
			rowPH[i] = query.Placeholder(argIdx)
			argIdx++
		}
		valuePlaceholders = append(valuePlaceholders, "("+strings.Join(rowPH, ", ")+")")
	}

	qualified := table.QualifiedName()
	sqlQuery := "INSERT INTO " + qualified + " (" + strings.Join(columns, ", ") + ") VALUES " + strings.Join(valuePlaceholders, ", ")

	if hasPrefer(prefer, "resolution=merge-duplicates") {
		// Determine conflict columns: on_conflict param > PK
		var conflictCols []string
		if onConflict := c.Query("on_conflict"); onConflict != "" {
			for _, col := range strings.Split(onConflict, ",") {
				col = strings.TrimSpace(col)
				if col == "" {
					continue
				}
				if _, ok := table.Columns[col]; !ok {
					c.JSON(http.StatusBadRequest, apierror.ErrInvalidColumn(col))
					return
				}
				conflictCols = append(conflictCols, col)
			}
		} else if table.PK != "" {
			conflictCols = []string{table.PK}
		}

		if len(conflictCols) > 0 {
			setClauses := make([]string, 0, len(columns))
			for _, col := range columns {
				isConflict := false
				for _, cc := range conflictCols {
					if col == cc {
						isConflict = true
						break
					}
				}
				if !isConflict {
					setClauses = append(setClauses, col+" = EXCLUDED."+col)
				}
			}
			conflictTarget := make([]string, len(conflictCols))
			for i, col := range conflictCols {
				conflictTarget[i] = col
			}
			if len(setClauses) > 0 {
				sqlQuery += " ON CONFLICT (" + strings.Join(conflictTarget, ", ") + ") DO UPDATE SET " + strings.Join(setClauses, ", ")
			} else {
				sqlQuery += " ON CONFLICT (" + strings.Join(conflictTarget, ", ") + ") DO NOTHING"
			}
		}
	} else if hasPrefer(prefer, "resolution=ignore-duplicates") {
		sqlQuery += " ON CONFLICT DO NOTHING"
	}

	result, err := h.DB.Exec(sqlQuery, allArgs...)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.JSON(http.StatusConflict, apierror.ErrConflict(err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, apierror.ErrDatabase(err.Error()))
		return
	}

	affected, _ := result.RowsAffected()
	response.WriteInserted(c, int(affected), prefer)
}

func (h *Handler) HandlePatch(c *gin.Context) {
	table, tableName, _, ok := h.resolveTable(c)
	if !ok {
		c.JSON(http.StatusNotFound, apierror.ErrTableNotFound(tableName))
		return
	}

	if table.IsView {
		c.JSON(http.StatusBadRequest, apierror.ErrMethodNotAllowed("Cannot update a view"))
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody(err.Error()))
		return
	}
	defer c.Request.Body.Close()

	if len(body) == 0 {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody("Empty request body"))
		return
	}

	var updateData map[string]interface{}
	if err := json.Unmarshal(body, &updateData); err != nil {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody(err.Error()))
		return
	}

	prefer := parsePrefer(c.GetHeader("Prefer"))

	params, err := h.parseParams(c, table)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.ErrInvalidFilter(err.Error()))
		return
	}

	var setCols []string
	var setArgs []interface{}
	argIdx := 1

	for col, val := range updateData {
		if _, ok := table.Columns[col]; !ok {
			c.JSON(http.StatusBadRequest, apierror.ErrInvalidColumn(col))
			return
		}
		setCols = append(setCols, col+" = "+query.Placeholder(argIdx))
		setArgs = append(setArgs, fmtVal(val))
		argIdx++
	}

	if len(setCols) == 0 {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody("No valid columns to update"))
		return
	}

	whereClause, whereArgs, err := buildWhereFromParams(params, &argIdx)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.ErrInvalidFilter(err.Error()))
		return
	}

	allArgs := append(setArgs, whereArgs...)

	qualified := table.QualifiedName()
	sqlQuery := "UPDATE " + qualified + " SET " + strings.Join(setCols, ", ")
	if whereClause != "" {
		sqlQuery += " WHERE " + whereClause
	}

	result, err := h.DB.Exec(sqlQuery, allArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.ErrDatabase(err.Error()))
		return
	}

	affected, _ := result.RowsAffected()
	response.WriteUpdated(c, int(affected), prefer)
}

func (h *Handler) HandlePut(c *gin.Context) {
	table, tableName, _, ok := h.resolveTable(c)
	if !ok {
		c.JSON(http.StatusNotFound, apierror.ErrTableNotFound(tableName))
		return
	}

	if table.IsView {
		c.JSON(http.StatusBadRequest, apierror.ErrMethodNotAllowed("Cannot upsert into a view"))
		return
	}

	if table.PK == "" {
		c.JSON(http.StatusBadRequest, apierror.ErrMethodNotAllowed("Table has no primary key for upsert"))
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody(err.Error()))
		return
	}
	defer c.Request.Body.Close()

	if len(body) == 0 {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody("Empty request body"))
		return
	}

	prefer := parsePrefer(c.GetHeader("Prefer"))

	var rowData map[string]interface{}
	if err := json.Unmarshal(body, &rowData); err != nil {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody(err.Error()))
		return
	}

	var columns []string
	for col := range rowData {
		if _, ok := table.Columns[col]; !ok {
			c.JSON(http.StatusBadRequest, apierror.ErrInvalidColumn(col))
			return
		}
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		c.JSON(http.StatusUnprocessableEntity, apierror.ErrInvalidBody("No valid columns"))
		return
	}

	argIdx := 1
	var allArgs []interface{}
	for _, col := range columns {
		allArgs = append(allArgs, fmtVal(rowData[col]))
		argIdx++
	}

	qualified := table.QualifiedName()
	sqlQuery := "INSERT INTO " + qualified + " (" + strings.Join(columns, ", ") + ") VALUES ("
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = query.Placeholder(i + 1)
	}
	sqlQuery += strings.Join(placeholders, ", ") + ")"

	sqlQuery += " ON CONFLICT (" + table.PK + ") DO UPDATE SET "
	setClauses := make([]string, 0, len(columns))
	for _, col := range columns {
		if col == table.PK {
			continue
		}
		setClauses = append(setClauses, col+" = EXCLUDED."+col)
	}
	if len(setClauses) > 0 {
		sqlQuery += strings.Join(setClauses, ", ")
	} else {
		sqlQuery += "DO NOTHING"
	}

	if hasPrefer(prefer, "return=representation") {
		sqlQuery += " RETURNING *"

		var returnCols []string
		for _, col := range table.OrderedColumns() {
			returnCols = append(returnCols, col.Name)
		}

		rows, err := h.DB.Query(sqlQuery, allArgs...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, apierror.ErrDatabase(err.Error()))
			return
		}
		defer rows.Close()

		if rows.Next() {
			values := make([]interface{}, len(returnCols))
			valuePtrs := make([]interface{}, len(returnCols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			if err := rows.Scan(valuePtrs...); err != nil {
				c.JSON(http.StatusInternalServerError, apierror.ErrDatabase(err.Error()))
				return
			}
			row := make(map[string]interface{}, len(returnCols))
			for i, col := range returnCols {
				row[col] = toInterface(values[i])
			}
			response.WriteSingularJSON(c, row)
			return
		}
		c.Status(http.StatusCreated)
		return
	}

	result, err := h.DB.Exec(sqlQuery, allArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.ErrDatabase(err.Error()))
		return
	}

	affected, _ := result.RowsAffected()
	response.WriteInserted(c, int(affected), prefer)
}

func (h *Handler) HandleDelete(c *gin.Context) {
	table, tableName, _, ok := h.resolveTable(c)
	if !ok {
		c.JSON(http.StatusNotFound, apierror.ErrTableNotFound(tableName))
		return
	}

	if table.IsView {
		c.JSON(http.StatusBadRequest, apierror.ErrMethodNotAllowed("Cannot delete from a view"))
		return
	}

	params, err := h.parseParams(c, table)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.ErrInvalidFilter(err.Error()))
		return
	}

	prefer := parsePrefer(c.GetHeader("Prefer"))

	builder := query.NewBuilder(table, params, table.SchemaName)
	sqlQuery, args, err := builder.BuildDelete()
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.ErrInvalidBody(err.Error()))
		return
	}

	result, err := h.DB.Exec(sqlQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.ErrDatabase(err.Error()))
		return
	}

	affected, _ := result.RowsAffected()
	response.WriteDeleted(c, int(affected), prefer)
}

func (h *Handler) HandleOptions(c *gin.Context) {
	table, _, _, ok := h.resolveTable(c)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}

	info := map[string]interface{}{
		"table_name":  table.Name,
		"schema":      table.SchemaName,
		"is_view":     table.IsView,
		"primary_key": table.PK,
	}

	colList := make([]map[string]interface{}, 0, len(table.Columns))
	for _, col := range table.OrderedColumns() {
		colList = append(colList, map[string]interface{}{
			"name":       col.Name,
			"data_type":  col.DataType,
			"nullable":   col.IsNullable,
			"is_pk":      col.IsPK,
			"is_unique":  col.IsUnique,
			"default":    col.DefaultValue,
		})
	}
	info["columns"] = colList

	fkList := make([]map[string]interface{}, 0, len(table.ForeignKeys))
	for _, fk := range table.ForeignKeys {
		fkList = append(fkList, map[string]interface{}{
			"constraint_name": fk.ConstraintName,
			"column":          fk.ColumnName,
			"ref_table":       fk.RefTable,
			"ref_column":      fk.RefColumn,
			"on_delete":       fk.OnDelete,
			"on_update":       fk.OnUpdate,
		})
	}
	info["foreign_keys"] = fkList

	c.Header("Allow", "GET, HEAD, POST, PATCH, DELETE, OPTIONS")
	c.JSON(http.StatusOK, info)
}

func (h *Handler) writeSingular(c *gin.Context, rows *sql.Rows, cols []string) {
	if !rows.Next() {
		c.Status(http.StatusNotFound)
		return
	}
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
		row[col] = toInterface(values[i])
	}
	response.WriteSingularJSON(c, row)
}

func (h *Handler) parseParams(c *gin.Context, table *schema.Table) (*query.Params, error) {
	tableColumns := make(map[string]bool)
	for col := range table.Columns {
		tableColumns[col] = true
	}
	return query.Parse(c.Request.URL.RawQuery, tableColumns)
}

func (h *Handler) executeCount(table *schema.Table, params *query.Params) (int, error) {
	countBuilder := query.NewBuilder(table, params, "")
	sqlQuery, args, err := countBuilder.BuildCount()
	if err != nil {
		return 0, err
	}
	var count int
	if err := h.DB.QueryRow(sqlQuery, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func parsePrefer(header string) []string {
	if header == "" {
		return nil
	}
	var result []string
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func hasPrefer(prefer []string, value string) bool {
	for _, p := range prefer {
		if strings.Contains(p, value) {
			return true
		}
	}
	return false
}

func fmtVal(val interface{}) interface{} {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return v
	case bool:
		return v
	case map[string]interface{}, []interface{}:
		b, _ := json.Marshal(v)
		return string(b)
	default:
		return v
	}
}

func toInterface(val interface{}) interface{} {
	if val == nil {
		return nil
	}
	if b, ok := val.([]byte); ok {
		var jsonVal interface{}
		if err := json.Unmarshal(b, &jsonVal); err == nil {
			return jsonVal
		}
		return string(b)
	}
	return val
}

func buildWhereFromParams(params *query.Params, argIdx *int) (string, []interface{}, error) {
	b := query.NewBuilder(nil, params, "")
	return b.BuildWherePublic(argIdx)
}
