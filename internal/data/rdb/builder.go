package rdb

import (
	"strings"
)

// QueryBuilder aids in building dynamic relational SQL queries with automatic dialect rebinding.
type QueryBuilder struct {
	dialect *Dialect
	base    string
	wheres  []string
	args    []any
	orderBy string
	limit   int
}

// NewQueryBuilder initializes a QueryBuilder with a Dialect and base SQL statement.
func NewQueryBuilder(d *Dialect, baseSQL string) *QueryBuilder {
	return &QueryBuilder{
		dialect: d,
		base:    strings.TrimSpace(baseSQL),
	}
}

// Where adds a WHERE condition. Standard '?' placeholders should be used.
func (qb *QueryBuilder) Where(cond string, args ...any) *QueryBuilder {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return qb
	}
	qb.wheres = append(qb.wheres, cond)
	qb.args = append(qb.args, args...)
	return qb
}

// WhereSearch adds a multi-column keyword search condition.
func (qb *QueryBuilder) WhereSearch(search string, columns ...string) *QueryBuilder {
	search = strings.TrimSpace(search)
	if search == "" || len(columns) == 0 {
		return qb
	}
	likePattern := "%" + search + "%"

	conds := make([]string, len(columns))
	for i, col := range columns {
		conds[i] = qb.dialect.SearchCond(col, "?")
		qb.args = append(qb.args, likePattern)
	}
	qb.wheres = append(qb.wheres, "("+strings.Join(conds, " OR ")+")")
	return qb
}

// OrderBy sets the ORDER BY clause.
func (qb *QueryBuilder) OrderBy(clause string) *QueryBuilder {
	qb.orderBy = strings.TrimSpace(clause)
	return qb
}

// Limit sets the LIMIT clause.
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.limit = limit
	return qb
}

// Build generates the final SQL string (rebound to dialect) and the argument slice.
func (qb *QueryBuilder) Build() (string, []any) {
	var sb strings.Builder
	sb.WriteString(qb.base)
	if len(qb.wheres) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(qb.wheres, " AND "))
	}
	if qb.orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(qb.orderBy)
	}
	finalArgs := make([]any, len(qb.args), len(qb.args)+1)
	copy(finalArgs, qb.args)
	if qb.limit > 0 {
		finalArgs = append(finalArgs, qb.limit)
		sb.WriteString(" LIMIT ?")
	}

	rawSQL := sb.String()
	if qb.dialect != nil {
		return qb.dialect.Rebind(rawSQL), finalArgs
	}
	return rawSQL, finalArgs
}

// SQL returns the built SQL query string.
func (qb *QueryBuilder) SQL() string {
	sql, _ := qb.Build()
	return sql
}
