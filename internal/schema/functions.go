package schema

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

type Function struct {
	Name       string    `json:"name"`
	SchemaName string    `json:"schema_name"`
	ReturnType string    `json:"return_type"`
	Arguments  []FuncArg `json:"arguments"`
	Language   string    `json:"language"`
}

type FuncArg struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
}

func IntrospectFunctions(db *sql.DB, schemaName string) (map[string]*Function, error) {
	functions := make(map[string]*Function)

	query := `
		SELECT
			p.proname AS function_name,
			COALESCE(pg_catalog.pg_get_function_result(p.oid), 'void') AS return_type,
			p.prokind,
			l.lanname AS language,
			COALESCE(p.proargtypes::text, '{}'),
			COALESCE(p.proargnames::text, '{}')
		FROM pg_proc p
		JOIN pg_namespace n ON p.pronamespace = n.oid
		JOIN pg_language l ON p.prolang = l.oid
		WHERE n.nspname = $1
		ORDER BY p.proname;`

	rows, err := db.Query(query, schemaName)
	if err != nil {
		return nil, fmt.Errorf("query functions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var funcName, retType, lang string
		var prokind string
		var argTypesText, argNamesText string

		if err := rows.Scan(&funcName, &retType, &prokind, &lang, &argTypesText, &argNamesText); err != nil {
			return nil, fmt.Errorf("scan function %s: %w", funcName, err)
		}

		fn := &Function{
			Name:       funcName,
			SchemaName: schemaName,
			ReturnType: retType,
			Language:   lang,
		}

		fn.Arguments = parseFuncArgs(argTypesText, argNamesText)

		key := funcName
		if _, exists := functions[key]; exists {
			key = funcName + "_" + retType
		}
		functions[key] = fn
	}

	return functions, rows.Err()
}

func parseFuncArgs(argTypesText, argNamesText string) []FuncArg {
	argTypesText = strings.TrimSpace(argTypesText)
	argNamesText = strings.TrimSpace(argNamesText)

	if argTypesText == "" || argTypesText == "{}" {
		return nil
	}

	argTypesText = strings.Trim(argTypesText, "{}")

	var typeStrs []string
	if strings.Contains(argTypesText, " ") {
		typeStrs = strings.Fields(argTypesText)
	} else if strings.Contains(argTypesText, ",") {
		typeStrs = strings.Split(argTypesText, ",")
	} else {
		typeStrs = []string{argTypesText}
	}

	numInputArgs := len(typeStrs)
	var allNames []string
	if argNamesText != "" && argNamesText != "{}" {
		argNamesText = strings.Trim(argNamesText, "{}")
		allNames = strings.Split(argNamesText, ",")
	}

	nameCount := numInputArgs
	if len(allNames) < numInputArgs {
		nameCount = len(allNames)
	}

	args := make([]FuncArg, 0, numInputArgs)
	for i, typeStr := range typeStrs {
		typeStr = strings.TrimSpace(typeStr)
		if typeStr == "" {
			continue
		}

		arg := FuncArg{
			Name: fmt.Sprintf("p_%d", i),
			Type: resolveOIDType(typeStr),
		}

		if i < nameCount {
			name := strings.TrimSpace(allNames[i])
			if name != "" {
				arg.Name = name
			}
		}

		args = append(args, arg)
	}

	return args
}

func resolveOIDType(oidStr string) string {
	switch oidStr {
	case "16":
		return "boolean"
	case "17":
		return "bytea"
	case "18":
		return "char"
	case "19":
		return "name"
	case "20":
		return "bigint"
	case "21":
		return "smallint"
	case "23":
		return "integer"
	case "25":
		return "text"
	case "700":
		return "real"
	case "701":
		return "double precision"
	case "1042":
		return "character"
	case "1043":
		return "character varying"
	case "1082":
		return "date"
	case "1114":
		return "timestamp without time zone"
	case "1184":
		return "timestamp with time zone"
	case "1700":
		return "numeric"
	case "2950":
		return "uuid"
	case "3802":
		return "jsonb"
	case "114":
		return "json"
	case "142":
		return "xml"
	case "143":
		return "ARRAY"
	default:
		if strings.HasPrefix(oidStr, "1043") {
			return "character varying"
		}
		return oidStr
	}
}
