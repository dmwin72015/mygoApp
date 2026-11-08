package rdb

import (
	"strings"
	"testing"
)

func TestDialectRebind(t *testing.T) {
	pg := NewDialect("postgres")
	if !pg.IsPostgres() {
		t.Errorf("expected IsPostgres() == true")
	}
	reboundPg := pg.Rebind("SELECT * FROM tbl WHERE a = ? AND b = ?")
	if reboundPg != "SELECT * FROM tbl WHERE a = $1 AND b = $2" {
		t.Errorf("expected $1, $2, got %q", reboundPg)
	}

	my := NewDialect("mysql")
	if !my.IsMySQL() {
		t.Errorf("expected IsMySQL() == true")
	}
	reboundMy := my.Rebind("SELECT * FROM tbl WHERE a = ? AND b = ?")
	if reboundMy != "SELECT * FROM tbl WHERE a = ? AND b = ?" {
		t.Errorf("expected ?, ?, got %q", reboundMy)
	}
}

func TestQueryBuilderDialect(t *testing.T) {
	pg := NewDialect("postgres")
	qbPg := NewQueryBuilder(pg, "SELECT * FROM users")
	qbPg.Where("status = ?", "active")
	qbPg.WhereSearch("alice", "username", "display_name")
	qbPg.OrderBy("id DESC")
	qbPg.Limit(10)

	sqlPg, argsPg := qbPg.Build()
	if !strings.Contains(sqlPg, "WHERE status = $1") {
		t.Errorf("expected $1 for first where clause in pg, got %s", sqlPg)
	}
	if !strings.Contains(sqlPg, "username ILIKE $2") {
		t.Errorf("expected ILIKE $2 in pg, got %s", sqlPg)
	}
	if !strings.Contains(sqlPg, "LIMIT $4") {
		t.Errorf("expected LIMIT $4 in pg, got %s", sqlPg)
	}
	if len(argsPg) != 4 {
		t.Errorf("expected 4 args, got %d", len(argsPg))
	}

	my := NewDialect("mysql")
	qbMy := NewQueryBuilder(my, "SELECT * FROM users")
	qbMy.Where("status = ?", "active")
	qbMy.WhereSearch("alice", "username", "display_name")
	qbMy.OrderBy("id DESC")
	qbMy.Limit(10)

	sqlMy, argsMy := qbMy.Build()
	if !strings.Contains(sqlMy, "WHERE status = ?") {
		t.Errorf("expected ? in mysql, got %s", sqlMy)
	}
	if !strings.Contains(sqlMy, "username LIKE ?") {
		t.Errorf("expected LIKE ? in mysql, got %s", sqlMy)
	}
	if !strings.Contains(sqlMy, "LIMIT ?") {
		t.Errorf("expected LIMIT ? in mysql, got %s", sqlMy)
	}
	if len(argsMy) != 4 {
		t.Errorf("expected 4 args, got %d", len(argsMy))
	}
}
