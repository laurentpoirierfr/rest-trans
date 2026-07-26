package openapi

import (
	"strings"

	"github.com/laurentpoirierfr/rest-trans/internal/config"
	"github.com/laurentpoirierfr/rest-trans/internal/schema"
)

type Spec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       Info                   `json:"info"`
	Servers    []Server               `json:"servers"`
	Paths      map[string]PathItem    `json:"paths"`
	Components Components             `json:"components"`
}

type Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type Server struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type PathItem map[string]Operation

type Operation struct {
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	OperationID string              `json:"operationId"`
	Tags        []string            `json:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

type Parameter struct {
	Name     string  `json:"name"`
	In       string  `json:"in"`
	Required bool    `json:"required"`
	Schema   Schema  `json:"schema"`
}

type Schema struct {
	Type       string                `json:"type,omitempty"`
	Ref        string                `json:"$ref,omitempty"`
	Properties map[string]Schema     `json:"properties,omitempty"`
	Items      *Schema               `json:"items,omitempty"`
	Nullable   bool                  `json:"nullable,omitempty"`
	Default    interface{}           `json:"default,omitempty"`
	Required   []string              `json:"required,omitempty"`
	OneOf      []Schema              `json:"oneOf,omitempty"`
}

type RequestBody struct {
	Required bool                `json:"required"`
	Content  map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema Schema `json:"schema"`
}

type Response struct {
	Description string `json:"description"`
}

type Components struct {
	Schemas map[string]interface{} `json:"schemas"`
}

func Generate(store *schema.SchemaStore, cfg *config.Config) *Spec {
	spec := &Spec{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:       "rest-trans",
			Description: "Auto-generated REST API from PostgreSQL schema",
			Version:     "1.0.0",
		},
		Servers: []Server{
			{URL: "/", Description: "Default server"},
		},
		Paths:      make(map[string]PathItem),
		Components: Components{Schemas: make(map[string]interface{})},
	}

	for _, sName := range store.SchemaNames() {
		tables := store.TablesBySchema(sName)
		for tableName, table := range tables {
			path := "/" + sName + "/" + tableName
			item := make(PathItem)

			if table.IsMethodAllowed("GET") && cfg.IsMethodAllowed(tableName, "GET") {
				item["get"] = buildGETOp(table, sName)
			}
			if table.IsMethodAllowed("HEAD") && cfg.IsMethodAllowed(tableName, "HEAD") {
				item["head"] = buildHEADOp(table, sName)
			}
			if table.IsMethodAllowed("POST") && cfg.IsMethodAllowed(tableName, "POST") {
				item["post"] = buildPOSTOp(table, sName)
			}
			if table.IsMethodAllowed("PUT") && cfg.IsMethodAllowed(tableName, "PUT") {
				item["put"] = buildPUTOp(table, sName)
			}
			if table.IsMethodAllowed("PATCH") && cfg.IsMethodAllowed(tableName, "PATCH") {
				item["patch"] = buildPATCHOp(table, sName)
			}
			if table.IsMethodAllowed("DELETE") && cfg.IsMethodAllowed(tableName, "DELETE") {
				item["delete"] = buildDELETEOp(table, sName)
			}
			item["options"] = buildOPTIONSOps(table, sName)

			if len(item) > 0 {
				spec.Paths[path] = item
				spec.Components.Schemas[tableName] = buildSchema(table)
			}
		}

		funcs := store.FunctionsBySchema(sName)
		for funcName, fn := range funcs {
			if !cfg.IsRPCEnabled(funcName) {
				continue
			}
			path := "/" + sName + "/rpc/" + funcName
			item := make(PathItem)
			item["post"] = buildRPCOp(fn, sName)
			spec.Paths[path] = item
		}
	}

	return spec
}

func schemaParam(sName string) Parameter {
	return Parameter{
		Name:     "schema",
		In:       "path",
		Required: true,
		Schema:   Schema{Type: "string", Default: sName},
	}
}

func buildGETOp(table *schema.Table, sName string) Operation {
	params := []Parameter{schemaParam(sName)}
	params = append(params, filterParams(table)...)
	params = append(params, Parameter{
		Name: "select",
		In:   "query",
		Schema: Schema{Type: "string"},
	})
	params = append(params, Parameter{
		Name: "order",
		In:   "query",
		Schema: Schema{Type: "string"},
	})
	params = append(params, paginationParams()...)
	params = append(params, countParam())

	return Operation{
		Summary:     "List " + table.Name,
		Description: "Retrieve all rows from " + table.Name,
		OperationID: "list" + capitalize(table.Name),
		Tags:        []string{table.Name},
		Parameters:  params,
		Responses: map[string]Response{
			"200": {Description: "Successful response"},
		},
	}
}

func buildHEADOp(table *schema.Table, sName string) Operation {
	params := []Parameter{schemaParam(sName)}
	params = append(params, filterParams(table)...)
	params = append(params, countParam())

	return Operation{
		Summary:     "Head " + table.Name,
		Description: "Retrieve headers for " + table.Name,
		OperationID: "head" + capitalize(table.Name),
		Tags:        []string{table.Name},
		Parameters:  params,
		Responses: map[string]Response{
			"200": {Description: "Successful response"},
		},
	}
}

func buildPOSTOp(table *schema.Table, sName string) Operation {
	schemaRef := Schema{Ref: "#/components/schemas/" + table.Name}

	return Operation{
		Summary:     "Insert into " + table.Name,
		Description: "Insert one or more rows into " + table.Name,
		OperationID: "insert" + capitalize(table.Name),
		Tags:        []string{table.Name},
		Parameters:  []Parameter{schemaParam(sName)},
		RequestBody: &RequestBody{
			Required: true,
			Content: map[string]MediaType{
				"application/json": {
					Schema: Schema{
						OneOf: []Schema{
							schemaRef,
							Schema{Type: "array", Items: &schemaRef},
						},
					},
				},
			},
		},
		Responses: map[string]Response{
			"201": {Description: "Created"},
		},
	}
}

func buildPUTOp(table *schema.Table, sName string) Operation {
	schemaRef := Schema{Ref: "#/components/schemas/" + table.Name}

	return Operation{
		Summary:     "Upsert " + table.Name,
		Description: "Insert or update a single row in " + table.Name,
		OperationID: "upsert" + capitalize(table.Name),
		Tags:        []string{table.Name},
		Parameters:  []Parameter{schemaParam(sName)},
		RequestBody: &RequestBody{
			Required: true,
			Content: map[string]MediaType{
				"application/json": {Schema: schemaRef},
			},
		},
		Responses: map[string]Response{
			"200": {Description: "Upserted"},
			"201": {Description: "Created"},
		},
	}
}

func buildPATCHOp(table *schema.Table, sName string) Operation {
	params := []Parameter{schemaParam(sName)}
	params = append(params, filterParams(table)...)

	return Operation{
		Summary:     "Update " + table.Name,
		Description: "Update rows in " + table.Name,
		OperationID: "update" + capitalize(table.Name),
		Tags:        []string{table.Name},
		Parameters:  params,
		RequestBody: &RequestBody{
			Required: true,
			Content: map[string]MediaType{
				"application/json": {Schema: Schema{Type: "object"}},
			},
		},
		Responses: map[string]Response{
			"204": {Description: "No Content"},
		},
	}
}

func buildDELETEOp(table *schema.Table, sName string) Operation {
	params := []Parameter{schemaParam(sName)}
	params = append(params, filterParams(table)...)

	return Operation{
		Summary:     "Delete from " + table.Name,
		Description: "Delete rows from " + table.Name,
		OperationID: "delete" + capitalize(table.Name),
		Tags:        []string{table.Name},
		Parameters:  params,
		Responses: map[string]Response{
			"204": {Description: "No Content"},
		},
	}
}

func buildOPTIONSOps(table *schema.Table, sName string) Operation {
	return Operation{
		Summary:     "Options for " + table.Name,
		Description: "Retrieve metadata for " + table.Name,
		OperationID: "options" + capitalize(table.Name),
		Tags:        []string{table.Name},
		Parameters:  []Parameter{schemaParam(sName)},
		Responses: map[string]Response{
			"200": {Description: "Successful response"},
		},
	}
}

func buildRPCOp(fn *schema.Function, sName string) Operation {
	properties := make(map[string]Schema)
	var required []string

	for _, arg := range fn.Arguments {
		prop := Schema{Type: pgTypeToOpenAPI(arg.Type)}
		if arg.Nullable {
			prop.Nullable = true
		}
		if arg.Default != "" {
			prop.Default = arg.Default
		} else {
			required = append(required, arg.Name)
		}
		properties[arg.Name] = prop
	}

	reqSchema := Schema{
		Type:       "object",
		Properties: properties,
	}
	if len(required) > 0 {
		reqSchema.Required = required
	}

	retSchema := Schema{Type: "array"}
	if fn.ReturnType != "void" && fn.ReturnType != "record" {
		retSchema.Items = &Schema{Type: pgTypeToOpenAPI(fn.ReturnType)}
	} else {
		retSchema.Items = &Schema{Type: "object"}
	}

	return Operation{
		Summary:     "Call " + fn.Name,
		Description: "Execute stored procedure " + fn.Name + " (language: " + fn.Language + ")",
		OperationID: "rpc" + capitalize(fn.Name),
		Tags:        []string{"RPC"},
		Parameters:  []Parameter{schemaParam(sName)},
		RequestBody: &RequestBody{
			Required: true,
			Content: map[string]MediaType{
				"application/json": {Schema: reqSchema},
			},
		},
		Responses: map[string]Response{
			"200": {
				Description: "Successful response",
			},
		},
	}
}

func filterParams(table *schema.Table) []Parameter {
	var params []Parameter
	for _, col := range table.OrderedColumns() {
		params = append(params, Parameter{
			Name: col.Name,
			In:   "query",
			Schema: Schema{Type: pgTypeToOpenAPI(col.DataType)},
		})
	}
	return params
}

func paginationParams() []Parameter {
	return []Parameter{
		{Name: "limit", In: "query", Schema: Schema{Type: "integer"}},
		{Name: "offset", In: "query", Schema: Schema{Type: "integer"}},
	}
}

func countParam() Parameter {
	return Parameter{
		Name: "count",
		In:   "query",
		Schema: Schema{Type: "string"},
	}
}

func buildSchema(table *schema.Table) map[string]interface{} {
	props := make(map[string]interface{})
	var required []string

	for _, col := range table.OrderedColumns() {
		colSchema := map[string]interface{}{
			"type": pgTypeToOpenAPI(col.DataType),
		}
		if col.IsNullable {
			colSchema["nullable"] = true
		}
		if col.DefaultValue != "" {
			colSchema["default"] = col.DefaultValue
		}
		props[col.Name] = colSchema

		if !col.IsNullable && col.DefaultValue == "" {
			required = append(required, col.Name)
		}
	}

	result := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

func pgTypeToOpenAPI(pgType string) string {
	switch strings.ToLower(pgType) {
	case "integer", "bigint", "smallint", "serial", "bigserial", "int4", "int8", "int2":
		return "integer"
	case "real", "double precision", "numeric", "decimal", "float4", "float8":
		return "number"
	case "boolean", "bool":
		return "boolean"
	case "json", "jsonb":
		return "object"
	case "array":
		return "array"
	case "date", "timestamp", "timestamptz", "timestamp without time zone", "timestamp with time zone":
		return "string"
	case "uuid":
		return "string"
	case "bytea":
		return "string"
	default:
		return "string"
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
