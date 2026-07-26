package schema

import "strings"

type Column struct {
	Name            string `json:"name"`
	DataType        string `json:"data_type"`
	IsNullable      bool   `json:"is_nullable"`
	IsPK            bool   `json:"is_pk"`
	IsUnique        bool   `json:"is_unique"`
	DefaultValue    string `json:"default_value,omitempty"`
	OrdinalPos      int    `json:"ordinal_position"`
	CharacterMaxLen int    `json:"character_maximum_length,omitempty"`
}

type ForeignKey struct {
	ConstraintName string `json:"constraint_name"`
	ColumnName     string `json:"column_name"`
	RefTable       string `json:"ref_table"`
	RefColumn      string `json:"ref_column"`
	OnDelete       string `json:"on_delete"`
	OnUpdate       string `json:"on_update"`
}

type Table struct {
	Name           string            `json:"name"`
	SchemaName     string            `json:"schema_name"`
	Columns        map[string]Column `json:"columns"`
	ColumnOrder    []string          `json:"column_order"`
	PK             string            `json:"pk"`
	IsView         bool              `json:"is_view"`
	ForeignKeys    []ForeignKey      `json:"foreign_keys,omitempty"`
	AllowedMethods []string          `json:"allowed_methods,omitempty"`
	DenyMethods    []string          `json:"deny_methods,omitempty"`
}

func (t *Table) QualifiedName() string {
	if t.SchemaName != "" && t.SchemaName != "public" {
		return t.SchemaName + "." + t.Name
	}
	return t.Name
}

func (t *Table) OrderedColumns() []Column {
	cols := make([]Column, 0, len(t.ColumnOrder))
	for _, name := range t.ColumnOrder {
		if c, ok := t.Columns[name]; ok {
			cols = append(cols, c)
		}
	}
	return cols
}

func (t *Table) IsMethodAllowed(method string) bool {
	for _, m := range t.DenyMethods {
		if strings.EqualFold(m, method) {
			return false
		}
	}
	if t.AllowedMethods == nil {
		return true
	}
	for _, m := range t.AllowedMethods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

type SchemaStore struct {
	schemas       map[string]map[string]*Table
	functions     map[string]map[string]*Function
	defaultSchema string
}

func NewSchemaStore(defSchema string) *SchemaStore {
	return &SchemaStore{
		schemas:       make(map[string]map[string]*Table),
		functions:     make(map[string]map[string]*Function),
		defaultSchema: defSchema,
	}
}

func (ss *SchemaStore) AddTable(schemaName string, table *Table) {
	table.SchemaName = schemaName
	if _, ok := ss.schemas[schemaName]; !ok {
		ss.schemas[schemaName] = make(map[string]*Table)
	}
	ss.schemas[schemaName][table.Name] = table
}

func (ss *SchemaStore) GetTable(schemaName, tableName string) (*Table, bool) {
	if schemaName == "" {
		schemaName = ss.defaultSchema
	}
	schema, ok := ss.schemas[schemaName]
	if !ok {
		return nil, false
	}
	t, ok := schema[tableName]
	return t, ok
}

func (ss *SchemaStore) AllTables() map[string]*Table {
	result := make(map[string]*Table)
	for _, tables := range ss.schemas {
		for name, table := range tables {
			result[name] = table
		}
	}
	return result
}

func (ss *SchemaStore) TablesBySchema(schemaName string) map[string]*Table {
	if schemaName == "" {
		schemaName = ss.defaultSchema
	}
	return ss.schemas[schemaName]
}

func (ss *SchemaStore) DefaultSchema() string {
	return ss.defaultSchema
}

func (ss *SchemaStore) SchemaNames() []string {
	names := make([]string, 0, len(ss.schemas))
	for name := range ss.schemas {
		names = append(names, name)
	}
	return names
}

func (ss *SchemaStore) HasTable(schemaName, tableName string) bool {
	_, ok := ss.GetTable(schemaName, tableName)
	return ok
}

func (ss *SchemaStore) AddFunction(schemaName string, fn *Function) {
	if _, ok := ss.functions[schemaName]; !ok {
		ss.functions[schemaName] = make(map[string]*Function)
	}
	ss.functions[schemaName][fn.Name] = fn
}

func (ss *SchemaStore) GetFunction(schemaName, funcName string) (*Function, bool) {
	if schemaName == "" {
		schemaName = ss.defaultSchema
	}
	schema, ok := ss.functions[schemaName]
	if !ok {
		return nil, false
	}
	fn, ok := schema[funcName]
	return fn, ok
}

func (ss *SchemaStore) AllFunctions() map[string]*Function {
	result := make(map[string]*Function)
	for _, fns := range ss.functions {
		for name, fn := range fns {
			result[name] = fn
		}
	}
	return result
}

func (ss *SchemaStore) FunctionsBySchema(schemaName string) map[string]*Function {
	if schemaName == "" {
		schemaName = ss.defaultSchema
	}
	return ss.functions[schemaName]
}
