package query

import (
	"fmt"
	"strings"

	"github.com/laurentpoirierfr/rest-trans/internal/schema"
)

type Builder struct {
	Table      *schema.Table
	Params     *Params
	SchemaName string
	Store      *schema.SchemaStore
}

func NewBuilder(table *schema.Table, params *Params, schemaName string) *Builder {
	return &Builder{
		Table:      table,
		Params:     params,
		SchemaName: schemaName,
	}
}

func NewBuilderWithStore(table *schema.Table, params *Params, schemaName string, store *schema.SchemaStore) *Builder {
	return &Builder{
		Table:      table,
		Params:     params,
		SchemaName: schemaName,
		Store:      store,
	}
}

func (b *Builder) BuildSelect() (string, []interface{}, error) {
	var args []interface{}
	argIdx := 1

	query := "SELECT "

	if len(b.Params.Select) > 0 || len(b.Params.Embeds) > 0 || len(b.Params.Aggregates) > 0 {
		cols := make([]string, 0)
		for _, col := range b.Params.Select {
			if _, ok := b.Table.Columns[col]; !ok {
				return "", nil, fmt.Errorf("column '%s' not found", col)
			}
			cols = append(cols, fmt.Sprintf("%s.%s", b.qualifiedTable(), col))
		}

		for _, embed := range b.Params.Embeds {
			embedSQL := b.buildEmbedSelect(embed, &argIdx)
			cols = append(cols, embedSQL)
		}

		for _, agg := range b.Params.Aggregates {
			aggSQL := b.buildAggregateSelect(agg)
			cols = append(cols, aggSQL)
		}

		query += strings.Join(cols, ", ")
	} else {
		query += b.qualifiedTable() + ".*"
	}

	query += fmt.Sprintf(" FROM %s", b.qualifiedTable())

	joins, joinArgs, err := b.buildEmbedJoins(&argIdx)
	if err != nil {
		return "", nil, err
	}
	if joins != "" {
		query += " " + joins
		args = append(args, joinArgs...)
	}

	whereClause, whereArgs, err := b.buildWhere(&argIdx)
	if err != nil {
		return "", nil, err
	}
	if whereClause != "" {
		query += " WHERE " + whereClause
		args = append(args, whereArgs...)
	}

	if len(b.Params.Order) > 0 {
		orderBy := b.buildOrderBy()
		query += " ORDER BY " + orderBy
	}

	if b.Params.Range.Limit >= 0 {
		query += fmt.Sprintf(" LIMIT %d", b.Params.Range.Limit)
	}
	if b.Params.Range.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", b.Params.Range.Offset)
	}

	return query, args, nil
}

func (b *Builder) buildEmbedSelect(embed Embed, argIdx *int) string {
	if b.Store == nil {
		return "NULL"
	}

	refTable, ok := b.Store.GetTable("", embed.Table)
	if !ok {
		for _, sName := range b.Store.SchemaNames() {
			if refTable, ok = b.Store.GetTable(sName, embed.Table); ok {
				break
			}
		}
	}
	if !ok {
		return "NULL"
	}

	var jsonCols []string
	if len(embed.Select) == 0 {
		for _, col := range refTable.OrderedColumns() {
			jsonCols = append(jsonCols, fmt.Sprintf("'%s', %s.%s", col.Name, embed.Table, col.Name))
		}
	} else {
		for _, col := range embed.Select {
			if _, ok := refTable.Columns[col]; ok {
				jsonCols = append(jsonCols, fmt.Sprintf("'%s', %s.%s", col, embed.Table, col))
			}
		}
	}

	if len(jsonCols) == 0 {
		return "NULL"
	}

	return fmt.Sprintf("json_build_object(%s) AS %s", strings.Join(jsonCols, ", "), embed.Table)
}

func (b *Builder) buildEmbedJoins(argIdx *int) (string, []interface{}, error) {
	if b.Store == nil || len(b.Params.Embeds) == 0 {
		return "", nil, nil
	}

	var joins []string
	var args []interface{}

	for _, embed := range b.Params.Embeds {
		refTable, ok := b.Store.GetTable("", embed.Table)
		if !ok {
			for _, sName := range b.Store.SchemaNames() {
				if refTable, ok = b.Store.GetTable(sName, embed.Table); ok {
					break
				}
			}
		}
		if !ok {
			continue
		}

		var fk *schema.ForeignKey
		for _, f := range b.Table.ForeignKeys {
			if f.RefTable == embed.Table {
				fk = &f
				break
			}
		}
		if fk == nil {
			for _, f := range refTable.ForeignKeys {
				if f.RefTable == b.Table.Name {
					joinCol := fmt.Sprintf("%s.%s", embed.Table, f.RefColumn)
					onClause := fmt.Sprintf("%s.%s = %s", b.qualifiedTable(), b.Table.PK, joinCol)
					join := fmt.Sprintf("LEFT JOIN %s AS %s ON %s", refTable.QualifiedName(), embed.Table, onClause)
					joins = append(joins, join)
					break
				}
			}
			continue
		}

		joinCol := fmt.Sprintf("%s.%s", embed.Table, fk.RefColumn)
		onClause := fmt.Sprintf("%s.%s = %s", b.qualifiedTable(), fk.ColumnName, joinCol)
		join := fmt.Sprintf("LEFT JOIN %s AS %s ON %s", refTable.QualifiedName(), embed.Table, onClause)
		joins = append(joins, join)
	}

	return strings.Join(joins, " "), args, nil
}

func (b *Builder) BuildCount() (string, []interface{}, error) {
	var args []interface{}
	argIdx := 1

	query := fmt.Sprintf("SELECT count(*) FROM %s", b.qualifiedTable())

	whereClause, whereArgs, err := b.buildWhere(&argIdx)
	if err != nil {
		return "", nil, err
	}
	if whereClause != "" {
		query += " WHERE " + whereClause
		args = append(args, whereArgs...)
	}

	return query, args, nil
}

func (b *Builder) BuildInsert(columns []string, rowCount int) (string, []interface{}, error) {
	if len(columns) == 0 || rowCount == 0 {
		return "", nil, fmt.Errorf("empty insert")
	}

	placeholders := make([]string, 0, len(columns))
	cols := make([]string, 0, len(columns))

	for _, col := range columns {
		cols = append(cols, col)
	}

	argIdx := 1
	var args []interface{}

	for row := 0; row < rowCount; row++ {
		rowPlaceholders := make([]string, len(columns))
		for i := range columns {
			rowPlaceholders[i] = fmt.Sprintf("$%d", argIdx)
			argIdx++
		}
		placeholders = append(placeholders, "("+strings.Join(rowPlaceholders, ", ")+")")
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES %s",
		b.qualifiedTable(),
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)

	return query, args, nil
}

func (b *Builder) BuildUpsert(columns []string, conflictCols []string, rowCount int) (string, []interface{}, error) {
	if len(columns) == 0 || rowCount == 0 {
		return "", nil, fmt.Errorf("empty upsert")
	}
	if len(conflictCols) == 0 {
		return "", nil, fmt.Errorf("empty conflict columns")
	}

	argIdx := 1
	var args []interface{}

	placeholders := make([]string, 0, len(columns))

	for row := 0; row < rowCount; row++ {
		rowPlaceholders := make([]string, len(columns))
		for i := range columns {
			rowPlaceholders[i] = fmt.Sprintf("$%d", argIdx)
			argIdx++
		}
		placeholders = append(placeholders, "("+strings.Join(rowPlaceholders, ", ")+")")
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES %s",
		b.qualifiedTable(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	conflictTarget := make([]string, len(conflictCols))
	for i, col := range conflictCols {
		conflictTarget[i] = col
	}

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

	var onConflict string
	if len(setClauses) > 0 {
		onConflict = fmt.Sprintf(
			" ON CONFLICT (%s) DO UPDATE SET %s",
			strings.Join(conflictTarget, ", "),
			strings.Join(setClauses, ", "),
		)
	} else {
		onConflict = fmt.Sprintf(" ON CONFLICT (%s) DO NOTHING", strings.Join(conflictTarget, ", "))
	}

	return query + onConflict, args, nil
}

func (b *Builder) BuildUpdate(columns []string) (string, []interface{}, error) {
	if len(columns) == 0 {
		return "", nil, fmt.Errorf("empty update")
	}

	argIdx := 1
	var args []interface{}

	setClauses := make([]string, len(columns))
	for i, col := range columns {
		setClauses[i] = fmt.Sprintf("%s = $%d", col, argIdx)
		argIdx++
	}

	query := fmt.Sprintf("UPDATE %s SET %s", b.qualifiedTable(), strings.Join(setClauses, ", "))

	whereClause, whereArgs, err := b.buildWhere(&argIdx)
	if err != nil {
		return "", nil, err
	}
	if whereClause != "" {
		query += " WHERE " + whereClause
		args = append(args, whereArgs...)
	}

	return query, args, nil
}

func (b *Builder) BuildDelete() (string, []interface{}, error) {
	argIdx := 1
	query := fmt.Sprintf("DELETE FROM %s", b.qualifiedTable())

	whereClause, whereArgs, err := b.buildWhere(&argIdx)
	if err != nil {
		return "", nil, err
	}
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	return query, whereArgs, nil
}

func (b *Builder) qualifiedTable() string {
	if b.SchemaName != "" && b.SchemaName != "public" {
		return b.SchemaName + "." + b.Table.Name
	}
	return b.Table.Name
}

func (b *Builder) buildWhere(argIdx *int) (string, []interface{}, error) {
	var conditions []string
	var args []interface{}

	for _, filter := range b.Params.Filters {
		condition, conditionArgs, err := b.buildFilterCondition(filter, argIdx)
		if err != nil {
			return "", nil, err
		}
		conditions = append(conditions, condition)
		args = append(args, conditionArgs...)
	}

	for _, logical := range b.Params.Logicals {
		condition, conditionArgs, err := b.buildLogicalCondition(logical, argIdx)
		if err != nil {
			return "", nil, err
		}
		conditions = append(conditions, condition)
		args = append(args, conditionArgs...)
	}

	if len(conditions) == 0 {
		return "", nil, nil
	}

	return strings.Join(conditions, " AND "), args, nil
}

func (b *Builder) buildFilterCondition(filter Filter, argIdx *int) (string, []interface{}, error) {
	var args []interface{}
	col := filter.Column

	switch filter.Operator {
	case OpIs:
		val := strings.ToLower(filter.Value)
		if val == "null" {
			cond := fmt.Sprintf("%s IS NULL", col)
			if filter.Negate {
				cond = fmt.Sprintf("%s IS NOT NULL", col)
			}
			return cond, nil, nil
		}
		if val == "true" || val == "false" {
			cond := fmt.Sprintf("%s IS %s", col, strings.ToUpper(val))
			if filter.Negate {
				cond = fmt.Sprintf("%s IS NOT %s", col, strings.ToUpper(val))
			}
			return cond, nil, nil
		}
		return "", nil, fmt.Errorf("invalid IS value: %s (use true, false, or null)", filter.Value)

	case OpIn:
		values := parseInValues(filter.Value)
		placeholders := make([]string, len(values))
		for i, v := range values {
			placeholders[i] = fmt.Sprintf("$%d", *argIdx)
			args = append(args, v)
			*argIdx++
		}
		cond := fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", "))
		if filter.Negate {
			cond = fmt.Sprintf("NOT (%s)", cond)
		}
		return cond, args, nil

	default:
		sqlOp := filter.Operator.SQL()
		if filter.Modifier != "" {
			sqlOp = sqlOp + " " + strings.ToUpper(filter.Modifier)
		}
		cond := fmt.Sprintf("%s %s $%d", col, sqlOp, *argIdx)
		args = append(args, filter.Value)
		*argIdx++

		if filter.Negate {
			cond = fmt.Sprintf("NOT (%s)", cond)
		}
		return cond, args, nil
	}
}

func (b *Builder) buildLogicalCondition(lf LogicalFilter, argIdx *int) (string, []interface{}, error) {
	var conditions []string
	var args []interface{}

	for _, filter := range lf.Filters {
		condition, conditionArgs, err := b.buildFilterCondition(filter, argIdx)
		if err != nil {
			return "", nil, err
		}
		conditions = append(conditions, condition)
		args = append(args, conditionArgs...)
	}

	for _, subLogical := range lf.Logicals {
		condition, conditionArgs, err := b.buildLogicalCondition(subLogical, argIdx)
		if err != nil {
			return "", nil, err
		}
		conditions = append(conditions, condition)
		args = append(args, conditionArgs...)
	}

	if len(conditions) == 0 {
		return "", nil, nil
	}

	joiner := " AND "
	if lf.Op == LogicalOr {
		joiner = " OR "
	}

	result := "(" + strings.Join(conditions, joiner) + ")"

	if lf.Op == LogicalNot {
		result = "NOT " + result
	}

	return result, args, nil
}

func (b *Builder) buildOrderBy() string {
	var parts []string
	for _, item := range b.Params.Order {
		part := fmt.Sprintf("%s %s", item.Column, strings.ToUpper(string(item.Direction)))
		if item.Nulls != "" {
			part += " " + strings.ToUpper(item.Nulls)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func (b *Builder) buildAggregateSelect(agg Aggregate) string {
	col := agg.Col
	if col != "*" {
		col = b.qualifiedTable() + "." + col
	}
	return fmt.Sprintf("%s(%s) AS %s", strings.ToUpper(agg.Func), col, agg.Alias)
}

func Placeholder(idx int) string {
	return fmt.Sprintf("$%d", idx)
}

func (b *Builder) BuildWherePublic(argIdx *int) (string, []interface{}, error) {
	return b.buildWhere(argIdx)
}

func parseInValues(value string) []string {
	value = strings.Trim(value, "()")
	var values []string
	depth := 0
	current := strings.Builder{}

	for _, ch := range value {
		switch ch {
		case '"':
			depth++
		case ',':
			if depth == 0 {
				values = append(values, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		values = append(values, strings.TrimSpace(current.String()))
	}

	return values
}
