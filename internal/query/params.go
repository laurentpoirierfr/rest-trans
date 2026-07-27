package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Operator string

const (
	OpEq      Operator = "eq"
	OpNeq     Operator = "neq"
	OpGt      Operator = "gt"
	OpGte     Operator = "gte"
	OpLt      Operator = "lt"
	OpLte     Operator = "lte"
	OpLike    Operator = "like"
	OpIlike   Operator = "ilike"
	OpIn      Operator = "in"
	OpIs      Operator = "is"
	OpMatch   Operator = "match"
	OpImatch  Operator = "imatch"
	OpContains Operator = "cs"
	OpContained Operator = "cd"
	OpOverlaps Operator = "ov"
	OpAdjacent Operator = "adj"
	OpStrictlyLeft  Operator = "sl"
	OpStrictlyRight Operator = "sr"
	OpNotStrictlyLeft  Operator = "nsr"
	OpNotStrictlyRight Operator = "nsl"
)

func (o Operator) SQL() string {
	switch o {
	case OpEq:
		return "="
	case OpNeq:
		return "<>"
	case OpGt:
		return ">"
	case OpGte:
		return ">="
	case OpLt:
		return "<"
	case OpLte:
		return "<="
	case OpLike:
		return "LIKE"
	case OpIlike:
		return "ILIKE"
	case OpMatch:
		return "~"
	case OpImatch:
		return "~*"
	case OpContains:
		return "@>"
	case OpContained:
		return "<@"
	case OpOverlaps:
		return "&&"
	case OpAdjacent:
		return "-|-"
	case OpStrictlyLeft:
		return "<<"
	case OpStrictlyRight:
		return ">>"
	case OpNotStrictlyLeft:
		return "&<"
	case OpNotStrictlyRight:
		return "&>"
	default:
		return "="
	}
}

type FilterOp string

const (
	LogicalAnd FilterOp = "and"
	LogicalOr  FilterOp = "or"
	LogicalNot FilterOp = "not"
)

type OrderDirection string

const (
	Asc  OrderDirection = "asc"
	Desc OrderDirection = "desc"
)

type OrderItem struct {
	Column    string
	Direction OrderDirection
	Nulls     string
}

type Filter struct {
	Column   string
	Operator Operator
	Value    string
	Negate   bool
	Modifier string
}

type LogicalFilter struct {
	Op       FilterOp
	Filters  []Filter
	Logicals []LogicalFilter
}

type Range struct {
	Limit  int
	Offset int
}

type Embed struct {
	Table  string
	Select []string
}

type FtsFilter struct {
	Column     string
	SearchTerm string
	Negate     bool
}

type Aggregate struct {
	Func string
	Col  string
	Alias string
}

type Params struct {
	Filters    []Filter
	Logicals   []LogicalFilter
	FtsFilters []FtsFilter
	Order      []OrderItem
	Select     []string
	Embeds     []Embed
	Aggregates []Aggregate
	Range      Range
	Count      string
	Prefer     []string
}

var filterRegex = regexp.MustCompile(`^(?:(not)\.)?([a-z_][a-z0-9_]*)\.([a-z_]+)(?:\.\((.+)\))?$`)

func Parse(rawQuery string, tableColumns map[string]bool) (*Params, error) {
	params := &Params{
		Range: Range{Limit: -1, Offset: 0},
	}

	if rawQuery == "" {
		return params, nil
	}

	query := strings.TrimPrefix(rawQuery, "?")
	pairs := splitQuery(query)

	for key, value := range pairs {
		switch key {
		case "_select":
			cols, embeds, aggregates := parseSelect(value)
			params.Select = cols
			params.Embeds = embeds
			params.Aggregates = aggregates
		case "_order":
			items, err := parseOrder(value)
			if err != nil {
				return nil, err
			}
			params.Order = items
		case "_limit":
			v, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid _limit: %s", value)
			}
			params.Range.Limit = v
		case "_offset":
			v, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid _offset: %s", value)
			}
			params.Range.Offset = v
		case "_count":
			params.Count = value
		case "_or":
			lf, err := parseLogicalFilter(LogicalOr, value, tableColumns)
			if err != nil {
				return nil, err
			}
			params.Logicals = append(params.Logicals, lf)
		case "_and":
			lf, err := parseLogicalFilter(LogicalAnd, value, tableColumns)
			if err != nil {
				return nil, err
			}
			params.Logicals = append(params.Logicals, lf)
		case "_fts":
			fts, err := parseFtsFilter(value, tableColumns)
			if err != nil {
				return nil, err
			}
			params.FtsFilters = append(params.FtsFilters, fts)
		default:
			if strings.HasPrefix(key, "_not.") {
				subkey := key[5:]
				if subkey == "or" || subkey == "and" {
					op := LogicalOr
					if subkey == "and" {
						op = LogicalAnd
					}
					lf, err := parseLogicalFilter(op, value, tableColumns)
					if err != nil {
						return nil, err
					}
					lf.Op = LogicalNot
					params.Logicals = append(params.Logicals, lf)
				} else {
					filter, err := parseFilterWithPrefix(subkey, value, true, tableColumns)
					if err != nil {
						return nil, err
					}
					params.Filters = append(params.Filters, filter)
				}
			} else {
				filter, err := parseFilter(key, value, tableColumns)
				if err != nil {
					return nil, err
				}
				params.Filters = append(params.Filters, filter)
			}
		}
	}

	return params, nil
}

func parseFilter(key, value string, tableColumns map[string]bool) (Filter, error) {
	return parseFilterWithPrefix(key, value, false, tableColumns)
}

func parseFilterWithPrefix(key, value string, negate bool, tableColumns map[string]bool) (Filter, error) {
	if !tableColumns[key] {
		return Filter{}, fmt.Errorf("column '%s' not found", key)
	}

	op := OpEq
	val := value
	modifier := ""

	if strings.HasPrefix(value, "not.") {
		negate = !negate
		value = value[4:]
	}

	if parenIdx := strings.Index(value, "("); parenIdx > 0 {
		if parenEnd := strings.Index(value, ")"); parenEnd > parenIdx {
			potentialMod := value[parenIdx+1 : parenEnd]
			if potentialMod == "any" || potentialMod == "all" {
				modifier = potentialMod
				value = value[:parenIdx] + value[parenEnd+1:]
			}
		}
	}

	dotIdx := strings.IndexByte(value, '.')
	if dotIdx > 0 {
		potentialOp := value[:dotIdx]
		if isValidOperator(Operator(potentialOp)) {
			op = Operator(potentialOp)
			val = value[dotIdx+1:]
		} else {
			val = value
		}
	} else {
		val = value
	}

	return Filter{
		Column:   key,
		Operator: op,
		Value:    val,
		Negate:   negate,
		Modifier: modifier,
	}, nil
}

func parseFtsFilter(value string, tableColumns map[string]bool) (FtsFilter, error) {
	negate := false
	if strings.HasPrefix(value, "not.") {
		negate = true
		value = value[4:]
	}

	dotIdx := strings.IndexByte(value, '.')
	if dotIdx <= 0 {
		return FtsFilter{}, fmt.Errorf("invalid _fts format: %s (expected column.search_term)", value)
	}

	col := value[:dotIdx]
	searchTerm := value[dotIdx+1:]

	if !tableColumns[col] {
		return FtsFilter{}, fmt.Errorf("column '%s' not found", col)
	}

	return FtsFilter{
		Column:     col,
		SearchTerm: searchTerm,
		Negate:     negate,
	}, nil
}

func parseLogicalFilter(op FilterOp, value string, tableColumns map[string]bool) (LogicalFilter, error) {
	value = strings.Trim(value, "()")
	lf := LogicalFilter{Op: op}

	parts := splitLogicalConditions(value)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.HasPrefix(part, "not.") {
			inner := part[4:]
			innerParts := strings.SplitN(inner, ".", 2)
			if len(innerParts) == 2 && (innerParts[0] == "or" || innerParts[0] == "and") {
				subOp := LogicalOr
				if innerParts[0] == "and" {
					subOp = LogicalAnd
				}
				subLf, err := parseLogicalFilter(subOp, innerParts[1], tableColumns)
				if err != nil {
					return LogicalFilter{}, err
				}
				subLf.Op = LogicalNot
				lf.Logicals = append(lf.Logicals, subLf)
				continue
			}
		}

		filter, err := parseCondition(part, tableColumns)
		if err != nil {
			return LogicalFilter{}, err
		}
		lf.Filters = append(lf.Filters, filter)
	}

	return lf, nil
}

func parseCondition(cond string, tableColumns map[string]bool) (Filter, error) {
	negate := false
	if strings.HasPrefix(cond, "not.") {
		negate = true
		cond = cond[4:]
	}

	parts := strings.SplitN(cond, ".", 3)
	if len(parts) < 2 {
		return Filter{}, fmt.Errorf("invalid condition: %s", cond)
	}

	col := parts[0]
	if !tableColumns[col] {
		return Filter{}, fmt.Errorf("column '%s' not found", col)
	}

	op := OpEq
	val := ""
	if len(parts) == 3 {
		op = Operator(parts[1])
		val = parts[2]
		if !isValidOperator(op) {
			return Filter{}, fmt.Errorf("invalid operator '%s'", op)
		}
	} else if len(parts) == 2 {
		op = OpEq
		val = parts[1]
	}

	return Filter{
		Column:   col,
		Operator: op,
		Value:    val,
		Negate:   negate,
	}, nil
}

func splitLogicalConditions(s string) []string {
	var parts []string
	depth := 0
	current := strings.Builder{}

	for _, ch := range s {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

var aggregateFuncs = map[string]bool{
	"count": true, "sum": true, "avg": true, "min": true, "max": true,
	"count_distinct": true, "array_agg": true, "string_agg": true,
}

func parseSelect(value string) ([]string, []Embed, []Aggregate) {
	var cols []string
	var embeds []Embed
	var aggregates []Aggregate

	parts := splitSelectParts(value)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.HasSuffix(part, ")") {
			parenIdx := strings.IndexByte(part, '(')
			if parenIdx > 0 {
				funcName := strings.ToLower(part[:parenIdx])
				inner := part[parenIdx+1 : len(part)-1]
				inner = strings.TrimSpace(inner)

				if aggregateFuncs[funcName] {
					col := inner
					alias := funcName + "_" + col
					if col == "*" {
						alias = funcName
					}
					aggregates = append(aggregates, Aggregate{
						Func:  funcName,
						Col:   col,
						Alias: alias,
					})
					continue
				}

				tableName := part[:parenIdx]
				var embedCols []string
				if inner != "" && inner != "*" {
					embedCols = strings.Split(inner, ",")
					for i := range embedCols {
						embedCols[i] = strings.TrimSpace(embedCols[i])
					}
				}
				embeds = append(embeds, Embed{Table: tableName, Select: embedCols})
				continue
			}
		}

		cols = append(cols, part)
	}

	return cols, embeds, aggregates
}

func splitSelectParts(s string) []string {
	var parts []string
	depth := 0
	current := strings.Builder{}

	for _, ch := range s {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func parseOrder(value string) ([]OrderItem, error) {
	var items []OrderItem
	parts := strings.Split(value, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		item := OrderItem{
			Direction: Asc,
			Nulls:     "nulls first",
		}

		chunks := strings.Split(part, ".")
		item.Column = chunks[0]

		for _, chunk := range chunks[1:] {
			switch strings.ToLower(chunk) {
			case "asc":
				item.Direction = Asc
			case "desc":
				item.Direction = Desc
			case "nullsfirst":
				item.Nulls = "nulls first"
			case "nullslast":
				item.Nulls = "nullslast"
			}
		}

		items = append(items, item)
	}

	return items, nil
}

func isValidOperator(op Operator) bool {
	switch op {
	case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte,
		OpLike, OpIlike, OpIn, OpIs,
		OpMatch, OpImatch,
		OpContains, OpContained, OpOverlaps, OpAdjacent,
		OpStrictlyLeft, OpStrictlyRight, OpNotStrictlyLeft, OpNotStrictlyRight:
		return true
	default:
		return false
	}
}

func splitQuery(query string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(query, "&")

	for _, pair := range pairs {
		idx := strings.IndexByte(pair, '=')
		if idx < 0 {
			continue
		}
		key := pair[:idx]
		val := pair[idx+1:]
		val = strings.ReplaceAll(val, "+", " ")
		val = strings.ReplaceAll(val, "%2C", ",")
		val = strings.ReplaceAll(val, "%22", "\"")

		if existing, ok := result[key]; ok {
			result[key] = existing + "&" + val
		} else {
			result[key] = val
		}
	}

	return result
}

func (p *Params) HasCount() bool {
	return p.Count == "exact" || p.Count == "planned" || p.Count == "estimated"
}

func (p *Params) HasRange() bool {
	return p.Range.Limit >= 0 || p.Range.Offset > 0
}
