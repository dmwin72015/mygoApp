package postgres

import (
	"reflect"
	"testing"
)

func TestQueryBuilderBasic(t *testing.T) {
	qb := NewQueryBuilder("SELECT id, name FROM students")
	sql, args := qb.Build()
	if sql != "SELECT id, name FROM students" {
		t.Errorf("expected base query, got %q", sql)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestQueryBuilderWhereAndLimit(t *testing.T) {
	qb := NewQueryBuilder("SELECT * FROM students")
	qb.Where("active = ?", true)
	qb.Where("balance_cents > ?", 1000)
	qb.OrderBy("id DESC")
	qb.Limit(20)

	sql, args := qb.Build()
	expectedSQL := "SELECT * FROM students WHERE active = $1 AND balance_cents > $2 ORDER BY id DESC LIMIT $3"
	if sql != expectedSQL {
		t.Fatalf("expected SQL %q, got %q", expectedSQL, sql)
	}
	expectedArgs := []any{true, 1000, 20}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, args)
	}

	// Verify idempotency
	sql2, args2 := qb.Build()
	if sql != sql2 {
		t.Errorf("idempotency check failed: %q != %q", sql, sql2)
	}
	if !reflect.DeepEqual(args, args2) {
		t.Errorf("idempotency check failed: %v != %v", args, args2)
	}
}

func TestQueryBuilderWhereSearch(t *testing.T) {
	qb := NewQueryBuilder("SELECT id, name, phone, code FROM teachers")
	qb.WhereSearch("alice", "name", "phone", "code")
	qb.OrderBy("id DESC")
	qb.Limit(10)

	sql, args := qb.Build()
	expectedSQL := "SELECT id, name, phone, code FROM teachers WHERE (name ILIKE $1 OR phone ILIKE $1 OR code ILIKE $1) ORDER BY id DESC LIMIT $2"
	if sql != expectedSQL {
		t.Fatalf("expected SQL %q, got %q", expectedSQL, sql)
	}
	expectedArgs := []any{"%alice%", 10}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, args)
	}
}

func TestQueryBuilderEmptySearchNoOp(t *testing.T) {
	qb := NewQueryBuilder("SELECT * FROM classrooms")
	qb.WhereSearch("   ", "name", "location")
	sql, args := qb.Build()
	if sql != "SELECT * FROM classrooms" {
		t.Errorf("expected no WHERE clause for blank search, got %q", sql)
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %v", args)
	}
}
