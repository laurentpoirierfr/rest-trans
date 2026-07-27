package query

import (
	"strings"
	"testing"

	"github.com/laurentpoirierfr/rest-trans/internal/schema"
)

func testTable() *schema.Table {
	return &schema.Table{
		Name: "users",
		Columns: map[string]schema.Column{
			"id":    {Name: "id", DataType: "integer", IsPK: true},
			"name":  {Name: "name", DataType: "character varying"},
			"email": {Name: "email", DataType: "character varying"},
			"age":   {Name: "age", DataType: "integer"},
		},
		ColumnOrder: []string{"id", "name", "email", "age"},
		PK:          "id",
	}
}

func TestBuildSelectSimple(t *testing.T) {
	table := testTable()
	params := &Params{Range: Range{Limit: 100, Offset: 0}}

	b := NewBuilder(table, params, "")
	query, args, err := b.BuildSelect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if query == "" {
		t.Error("expected non-empty query")
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestBuildSelectWithFilter(t *testing.T) {
	table := testTable()
	params := &Params{
		Filters: []Filter{
			{Column: "name", Operator: OpEq, Value: "John"},
		},
		Range: Range{Limit: 100, Offset: 0},
	}

	b := NewBuilder(table, params, "")
	query, args, err := b.BuildSelect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
	if args[0] != "John" {
		t.Errorf("expected arg 'John', got %v", args[0])
	}
	_ = query
}

func TestBuildSelectWithOrder(t *testing.T) {
	table := testTable()
	params := &Params{
		Order: []OrderItem{
			{Column: "age", Direction: Desc},
			{Column: "name", Direction: Asc},
		},
		Range: Range{Limit: 100, Offset: 0},
	}

	b := NewBuilder(table, params, "")
	query, _, err := b.BuildSelect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if query == "" {
		t.Error("expected non-empty query")
	}
}

func TestBuildSelectWithSelectCols(t *testing.T) {
	table := testTable()
	params := &Params{
		Select: []string{"id", "name"},
		Range:  Range{Limit: 100, Offset: 0},
	}

	b := NewBuilder(table, params, "")
	query, _, err := b.BuildSelect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if query == "" {
		t.Error("expected non-empty query")
	}
}

func TestBuildCount(t *testing.T) {
	table := testTable()
	params := &Params{
		Filters: []Filter{
			{Column: "age", Operator: OpGte, Value: "18"},
		},
	}

	b := NewBuilder(table, params, "")
	query, args, err := b.BuildCount()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
	_ = query
}

func TestBuildDelete(t *testing.T) {
	table := testTable()
	params := &Params{
		Filters: []Filter{
			{Column: "id", Operator: OpEq, Value: "1"},
		},
	}

	b := NewBuilder(table, params, "")
	query, args, err := b.BuildDelete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
	_ = query
}

func TestBuildWithSchema(t *testing.T) {
	table := testTable()
	params := &Params{Range: Range{Limit: 100, Offset: 0}}

	b := NewBuilder(table, params, "private")
	query, _, err := b.BuildSelect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if query == "" {
		t.Error("expected non-empty query")
	}
}

func TestBuildUpsertSingleColumn(t *testing.T) {
	table := testTable()
	params := &Params{}
	b := NewBuilder(table, params, "")

	query, _, err := b.BuildUpsert(
		[]string{"name", "email"},
		[]string{"email"},
		1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "ON CONFLICT (email) DO UPDATE SET") {
		t.Errorf("expected ON CONFLICT (email) DO UPDATE SET, got: %s", query)
	}
	if strings.Contains(query, "EXCLUDED.email") {
		t.Errorf("should not update conflict column, got: %s", query)
	}
	if !strings.Contains(query, "EXCLUDED.name") {
		t.Errorf("should update non-conflict column, got: %s", query)
	}
}

func TestBuildUpsertMultiColumn(t *testing.T) {
	table := testTable()
	params := &Params{}
	b := NewBuilder(table, params, "")

	query, _, err := b.BuildUpsert(
		[]string{"name", "email", "age"},
		[]string{"name", "email"},
		2,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "ON CONFLICT (name, email) DO UPDATE SET") {
		t.Errorf("expected ON CONFLICT (name, email), got: %s", query)
	}
	if !strings.Contains(query, "EXCLUDED.age") {
		t.Errorf("should update non-conflict column 'age', got: %s", query)
	}
	if strings.Contains(query, "EXCLUDED.name") || strings.Contains(query, "EXCLUDED.email") {
		t.Errorf("should not update conflict columns, got: %s", query)
	}
}

func TestBuildUpsertDoNothing(t *testing.T) {
	table := testTable()
	params := &Params{}
	b := NewBuilder(table, params, "")

	query, _, err := b.BuildUpsert(
		[]string{"email"},
		[]string{"email"},
		1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "ON CONFLICT (email) DO NOTHING") {
		t.Errorf("expected ON CONFLICT (email) DO NOTHING, got: %s", query)
	}
}

func TestBuildUpsertEmptyColumns(t *testing.T) {
	table := testTable()
	params := &Params{}
	b := NewBuilder(table, params, "")

	_, _, err := b.BuildUpsert([]string{}, []string{"email"}, 1)
	if err == nil {
		t.Error("expected error for empty columns")
	}
}

func TestBuildUpsertEmptyConflictCols(t *testing.T) {
	table := testTable()
	params := &Params{}
	b := NewBuilder(table, params, "")

	_, _, err := b.BuildUpsert([]string{"name"}, []string{}, 1)
	if err == nil {
		t.Error("expected error for empty conflict columns")
	}
}

func TestBuildUpsertRowCount(t *testing.T) {
	table := testTable()
	params := &Params{}
	b := NewBuilder(table, params, "")

	query, _, err := b.BuildUpsert(
		[]string{"name", "email"},
		[]string{"email"},
		3,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "($1, $2), ($3, $4), ($5, $6)") {
		t.Errorf("expected 3 row placeholders, got: %s", query)
	}
}
