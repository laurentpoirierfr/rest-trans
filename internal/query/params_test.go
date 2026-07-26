package query

import (
	"testing"
)

func TestParseSimpleEq(t *testing.T) {
	cols := map[string]bool{"name": true, "age": true, "email": true}
	params, err := Parse("name=eq.John", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(params.Filters))
	}
	f := params.Filters[0]
	if f.Column != "name" || f.Operator != OpEq || f.Value != "John" {
		t.Errorf("unexpected filter: %+v", f)
	}
}

func TestParseMultipleFilters(t *testing.T) {
	cols := map[string]bool{"name": true, "age": true}
	params, err := Parse("name=eq.John&age=gte.18", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(params.Filters))
	}
}

func TestParseNoOperator(t *testing.T) {
	cols := map[string]bool{"status": true}
	params, err := Parse("status=active", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(params.Filters))
	}
	f := params.Filters[0]
	if f.Column != "status" || f.Operator != OpEq || f.Value != "active" {
		t.Errorf("unexpected filter: %+v", f)
	}
}

func TestParseNegateFilter(t *testing.T) {
	cols := map[string]bool{"name": true}
	params, err := Parse("name=not.eq.John", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(params.Filters))
	}
	f := params.Filters[0]
	if !f.Negate {
		t.Error("expected negate to be true")
	}
}

func TestParseOr(t *testing.T) {
	cols := map[string]bool{"age": true}
	params, err := Parse("_or=(age.lt.18,age.gt.65)", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Logicals) != 1 {
		t.Fatalf("expected 1 logical filter, got %d", len(params.Logicals))
	}
	lf := params.Logicals[0]
	if lf.Op != LogicalOr {
		t.Errorf("expected OR, got %s", lf.Op)
	}
	if len(lf.Filters) != 2 {
		t.Errorf("expected 2 filters in OR, got %d", len(lf.Filters))
	}
}

func TestParseAnd(t *testing.T) {
	cols := map[string]bool{"active": true, "verified": true}
	params, err := Parse("_and=(active.is.true,verified.is.true)", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Logicals) != 1 {
		t.Fatalf("expected 1 logical filter, got %d", len(params.Logicals))
	}
	lf := params.Logicals[0]
	if lf.Op != LogicalAnd {
		t.Errorf("expected AND, got %s", lf.Op)
	}
}

func TestParseOrder(t *testing.T) {
	cols := map[string]bool{"name": true, "age": true}
	params, err := Parse("_order=age.desc,name.asc", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Order) != 2 {
		t.Fatalf("expected 2 order items, got %d", len(params.Order))
	}
	if params.Order[0].Column != "age" || params.Order[0].Direction != Desc {
		t.Errorf("expected age desc, got %+v", params.Order[0])
	}
	if params.Order[1].Column != "name" || params.Order[1].Direction != Asc {
		t.Errorf("expected name asc, got %+v", params.Order[1])
	}
}

func TestParseSelect(t *testing.T) {
	cols := map[string]bool{"id": true, "name": true, "email": true}
	params, err := Parse("_select=id,name", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Select) != 2 {
		t.Fatalf("expected 2 select cols, got %d", len(params.Select))
	}
	if params.Select[0] != "id" || params.Select[1] != "name" {
		t.Errorf("unexpected select: %v", params.Select)
	}
}

func TestParseLimitOffset(t *testing.T) {
	cols := map[string]bool{"id": true}
	params, err := Parse("_limit=20&_offset=40", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params.Range.Limit != 20 {
		t.Errorf("expected limit 20, got %d", params.Range.Limit)
	}
	if params.Range.Offset != 40 {
		t.Errorf("expected offset 40, got %d", params.Range.Offset)
	}
}

func TestParseCount(t *testing.T) {
	cols := map[string]bool{"id": true}
	params, err := Parse("_count=exact", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !params.HasCount() {
		t.Error("expected HasCount to be true")
	}
}

func TestParseInvalidColumn(t *testing.T) {
	cols := map[string]bool{"name": true}
	_, err := Parse("invalid_column=eq.test", cols)
	if err == nil {
		t.Error("expected error for invalid column")
	}
}

func TestParseInFilter(t *testing.T) {
	cols := map[string]bool{"id": true}
	params, err := Parse("id=in.(1,2,3)", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(params.Filters))
	}
	f := params.Filters[0]
	if f.Operator != OpIn {
		t.Errorf("expected IN operator, got %s", f.Operator)
	}
}

func TestParseIsFilter(t *testing.T) {
	cols := map[string]bool{"deleted_at": true}
	params, err := Parse("deleted_at=is.null", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(params.Filters))
	}
	f := params.Filters[0]
	if f.Operator != OpIs || f.Value != "null" {
		t.Errorf("expected IS null, got %+v", f)
	}
}

func TestParseEmpty(t *testing.T) {
	cols := map[string]bool{"id": true}
	params, err := Parse("", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Filters) != 0 {
		t.Errorf("expected 0 filters, got %d", len(params.Filters))
	}
}

func TestParseNotOr(t *testing.T) {
	cols := map[string]bool{"status": true}
	params, err := Parse("_not.or=(status.eq.active,status.eq.pending)", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Logicals) != 1 {
		t.Fatalf("expected 1 logical filter, got %d", len(params.Logicals))
	}
	lf := params.Logicals[0]
	if lf.Op != LogicalNot {
		t.Errorf("expected NOT, got %s", lf.Op)
	}
}

func TestParseNotAnd(t *testing.T) {
	cols := map[string]bool{"active": true, "verified": true}
	params, err := Parse("_not.and=(active.is.true,verified.is.true)", cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params.Logicals) != 1 {
		t.Fatalf("expected 1 logical filter, got %d", len(params.Logicals))
	}
	lf := params.Logicals[0]
	if lf.Op != LogicalNot {
		t.Errorf("expected NOT, got %s", lf.Op)
	}
}
