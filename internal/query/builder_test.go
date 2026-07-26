package query

import (
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
