package schema

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

func Introspect(db *sql.DB, schemaName string) (map[string]*Table, error) {
	tables := make(map[string]*Table)

	if err := introspectTables(db, schemaName, tables); err != nil {
		return nil, err
	}

	if err := introspectViews(db, schemaName, tables); err != nil {
		return nil, err
	}

	if err := introspectPrimaryKeys(db, schemaName, tables); err != nil {
		return nil, err
	}

	if err := introspectForeignKeys(db, schemaName, tables); err != nil {
		return nil, err
	}

	if err := introspectUniqueConstraints(db, schemaName, tables); err != nil {
		return nil, err
	}

	introspectPermissions(db, schemaName, tables)

	return tables, nil
}

func introspectTables(db *sql.DB, schemaName string, tables map[string]*Table) error {
	query := `
		SELECT 
			c.table_name, 
			c.column_name, 
			c.data_type, 
			c.is_nullable,
			COALESCE(c.column_default, ''),
			c.ordinal_position,
			COALESCE(c.character_maximum_length, 0)
		FROM information_schema.columns c
		JOIN information_schema.tables t ON c.table_name = t.table_name AND c.table_schema = t.table_schema
		WHERE c.table_schema = $1 AND t.table_type = 'BASE TABLE'
		ORDER BY c.table_name, c.ordinal_position;`

	rows, err := db.Query(query, schemaName)
	if err != nil {
		return fmt.Errorf("query columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName, dataType, isNullable, defaultVal string
		var ordinalPos, charMaxLen int
		if err := rows.Scan(&tableName, &colName, &dataType, &isNullable, &defaultVal, &ordinalPos, &charMaxLen); err != nil {
			return fmt.Errorf("scan column: %w", err)
		}

		if _, exists := tables[tableName]; !exists {
			tables[tableName] = &Table{
				Name:        tableName,
				Columns:     make(map[string]Column),
				ColumnOrder: make([]string, 0),
				IsView:      false,
			}
		}

		tbl := tables[tableName]
		tbl.Columns[colName] = Column{
			Name:            colName,
			DataType:        dataType,
			IsNullable:      isNullable == "YES",
			DefaultValue:    defaultVal,
			OrdinalPos:      ordinalPos,
			CharacterMaxLen: charMaxLen,
		}
		tbl.ColumnOrder = append(tbl.ColumnOrder, colName)
	}

	return rows.Err()
}

func introspectViews(db *sql.DB, schemaName string, tables map[string]*Table) error {
	query := `
		SELECT 
			v.table_name,
			c.column_name,
			c.data_type,
			c.is_nullable,
			COALESCE(c.column_default, ''),
			c.ordinal_position,
			COALESCE(c.character_maximum_length, 0)
		FROM information_schema.views v
		JOIN information_schema.columns c 
			ON v.table_name = c.table_name AND v.table_schema = c.table_schema
		WHERE v.table_schema = $1
		ORDER BY v.table_name, c.ordinal_position;`

	rows, err := db.Query(query, schemaName)
	if err != nil {
		return fmt.Errorf("query views: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName, dataType, isNullable, defaultVal string
		var ordinalPos, charMaxLen int
		if err := rows.Scan(&tableName, &colName, &dataType, &isNullable, &defaultVal, &ordinalPos, &charMaxLen); err != nil {
			return fmt.Errorf("scan view column: %w", err)
		}

		if _, exists := tables[tableName]; !exists {
			tables[tableName] = &Table{
				Name:        tableName,
				Columns:     make(map[string]Column),
				ColumnOrder: make([]string, 0),
				IsView:      true,
			}
		}

		tbl := tables[tableName]
		tbl.IsView = true
		tbl.Columns[colName] = Column{
			Name:            colName,
			DataType:        dataType,
			IsNullable:      isNullable == "YES",
			DefaultValue:    defaultVal,
			OrdinalPos:      ordinalPos,
			CharacterMaxLen: charMaxLen,
		}
		tbl.ColumnOrder = append(tbl.ColumnOrder, colName)
	}

	return rows.Err()
}

func introspectPrimaryKeys(db *sql.DB, schemaName string, tables map[string]*Table) error {
	query := `
		SELECT kcu.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY' 
			AND tc.table_schema = $1
		ORDER BY kcu.table_name, kcu.ordinal_position;`

	rows, err := db.Query(query, schemaName)
	if err != nil {
		return fmt.Errorf("query primary keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName string
		if err := rows.Scan(&tableName, &colName); err != nil {
			return fmt.Errorf("scan pk: %w", err)
		}
		if tbl, exists := tables[tableName]; exists {
			if col, ok := tbl.Columns[colName]; ok {
				col.IsPK = true
				tbl.Columns[colName] = col
				if tbl.PK == "" {
					tbl.PK = colName
				}
			}
		}
	}

	return rows.Err()
}

func introspectForeignKeys(db *sql.DB, schemaName string, tables map[string]*Table) error {
	query := `
		SELECT 
			tc.constraint_name,
			kcu.table_name,
			kcu.column_name,
			ccu.table_name AS ref_table,
			ccu.column_name AS ref_column,
			rc.update_rule AS on_update,
			rc.delete_rule AS on_delete
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu 
			ON tc.constraint_name = ccu.constraint_name
			AND tc.table_schema = ccu.table_schema
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name
			AND tc.table_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' 
			AND tc.table_schema = $1
		ORDER BY kcu.table_name, kcu.ordinal_position;`

	rows, err := db.Query(query, schemaName)
	if err != nil {
		return fmt.Errorf("query foreign keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var constraintName, tableName, colName, refTable, refColumn, onUpdate, onDelete string
		if err := rows.Scan(&constraintName, &tableName, &colName, &refTable, &refColumn, &onUpdate, &onDelete); err != nil {
			return fmt.Errorf("scan fk: %w", err)
		}

		if tbl, exists := tables[tableName]; exists {
			fk := ForeignKey{
				ConstraintName: constraintName,
				ColumnName:     colName,
				RefTable:       refTable,
				RefColumn:      refColumn,
				OnDelete:       onDelete,
				OnUpdate:       onUpdate,
			}
			tbl.ForeignKeys = append(tbl.ForeignKeys, fk)

			if col, ok := tbl.Columns[colName]; ok {
				tbl.Columns[colName] = col
			}
		}
	}

	return rows.Err()
}

func introspectUniqueConstraints(db *sql.DB, schemaName string, tables map[string]*Table) error {
	query := `
		SELECT 
			tc.constraint_name,
			kcu.table_name,
			string_agg(kcu.column_name, ',' ORDER BY kcu.ordinal_position) AS columns
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'UNIQUE' 
			AND tc.table_schema = $1
		GROUP BY tc.constraint_name, kcu.table_name;`

	rows, err := db.Query(query, schemaName)
	if err != nil {
		return fmt.Errorf("query unique constraints: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var constraintName, tableName, columns string
		if err := rows.Scan(&constraintName, &tableName, &columns); err != nil {
			return fmt.Errorf("scan unique: %w", err)
		}

		if tbl, exists := tables[tableName]; exists {
			colNames := strings.Split(columns, ",")
			if len(colNames) == 1 {
				if col, ok := tbl.Columns[colNames[0]]; ok {
					col.IsUnique = true
					tbl.Columns[colNames[0]] = col
				}
			}
			tbl.UniqueConstraints = append(tbl.UniqueConstraints, UniqueConstraint{
				ConstraintName: constraintName,
				Columns:        colNames,
			})
		}
	}

	return rows.Err()
}

func introspectPermissions(db *sql.DB, schemaName string, tables map[string]*Table) {
	query := `
		SELECT c.relname AS table_name,
		       pg_catalog.obj_description(c.oid, 'pg_class') AS table_comment
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1
		  AND c.relkind IN ('r', 'v', 'm')
		  AND pg_catalog.obj_description(c.oid, 'pg_class') IS NOT NULL;`

	rows, err := db.Query(query, schemaName)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, comment string
		if err := rows.Scan(&tableName, &comment); err != nil {
			continue
		}
		tbl, ok := tables[tableName]
		if !ok {
			continue
		}
		allowed, denied := parseComment(comment)
		if len(allowed) > 0 {
			tbl.AllowedMethods = allowed
		}
		if len(denied) > 0 {
			tbl.DenyMethods = denied
		}
	}
}

func parseComment(comment string) (allowed, denied []string) {
	for _, line := range strings.Split(comment, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@allow ") {
			for _, p := range strings.Split(strings.TrimPrefix(line, "@allow "), ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					allowed = append(allowed, strings.ToUpper(p))
				}
			}
		} else if strings.HasPrefix(line, "@deny ") {
			for _, p := range strings.Split(strings.TrimPrefix(line, "@deny "), ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					denied = append(denied, strings.ToUpper(p))
				}
			}
		}
	}
	return
}
