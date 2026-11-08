package postgres

import (
	"fmt"
	"strings"
)

// QueryBuilder aids in building dynamic PostgreSQL queries with positional parameters ($1, $2, ...).
type QueryBuilder struct {
	base    string
	wheres  []string
	args    []any
	orderBy string
	limit   int
}

// NewQueryBuilder initializes a QueryBuilder with the given base SQL statement.
func NewQueryBuilder(baseSQL string) *QueryBuilder {
	return &QueryBuilder{
		base: strings.TrimSpace(baseSQL),
	}
}

// Where adds a WHERE condition. Any '?' placeholder is replaced with the corresponding '$N' index.
func (qb *QueryBuilder) Where(cond string, args ...any) *QueryBuilder {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return qb
	}
	var sb strings.Builder
	argIdx := 0
	for _, r := range cond {
		if r == '?' && argIdx < len(args) {
			qb.args = append(qb.args, args[argIdx])
			sb.WriteString(fmt.Sprintf("$%d", len(qb.args)))
			argIdx++
		} else {
			sb.WriteRune(r)
		}
	}
	qb.wheres = append(qb.wheres, sb.String())
	return qb
}

// WhereSearch adds a multi-column ILIKE condition matching search keyword.
// If search is empty or no columns are provided, it is a no-op.
func (qb *QueryBuilder) WhereSearch(search string, columns ...string) *QueryBuilder {
	search = strings.TrimSpace(search)
	if search == "" || len(columns) == 0 {
		return qb
	}
	qb.args = append(qb.args, "%"+search+"%")
	placeholder := fmt.Sprintf("$%d", len(qb.args))

	conds := make([]string, len(columns))
	for i, col := range columns {
		conds[i] = fmt.Sprintf("%s ILIKE %s", col, placeholder)
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

// Build generates the complete SQL string and corresponding argument slice.
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
		sb.WriteString(fmt.Sprintf(" LIMIT $%d", len(finalArgs)))
	}
	return sb.String(), finalArgs
}

// SQL returns the built SQL query string.
func (qb *QueryBuilder) SQL() string {
	sql, _ := qb.Build()
	return sql
}

// Args returns the arguments for the built query.
func (qb *QueryBuilder) Args() []any {
	_, args := qb.Build()
	return args
}
