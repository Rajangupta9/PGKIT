package qb

import (
	"strings"

	"github.com/jackc/pgx/v5"
)

// QuoteIdent safely quotes a single PostgreSQL identifier.
//
//	QuoteIdent("my table") → "my table"
func QuoteIdent(name string) string {
	return pgx.Identifier{name}.Sanitize()
}

// quoteIdentExpr handles:
//  1. Raw expressions (contains ( ) * / space) → verbatim
//  2. Qualified names "alias.col" → "alias"."col"
//  3. Plain names → "name"
//
// Security note: case (1) is the only path that does not quote. Callers must
// never let user input flow into expressions that hit this branch.
func quoteIdentExpr(expr string) string {
	if strings.ContainsAny(expr, "()*/ ") {
		return expr
	}
	if dot := strings.IndexByte(expr, '.'); dot >= 0 {
		left := expr[:dot]
		right := expr[dot+1:]
		var lq string
		if strings.HasPrefix(left, `"`) && strings.HasSuffix(left, `"`) {
			lq = left
		} else {
			lq = pgx.Identifier{left}.Sanitize()
		}
		return lq + "." + quoteIdentExpr(right)
	}
	if strings.HasPrefix(expr, `"`) && strings.HasSuffix(expr, `"`) {
		return expr
	}
	return pgx.Identifier{expr}.Sanitize()
}

// quoteIdentRef quotes a column/reference path strictly. Unlike
// quoteIdentExpr, it never treats spaces, parentheses, or operators as raw SQL.
// Use this for structured APIs where the caller is expected to pass an
// identifier, not an expression.
func quoteIdentRef(ref string) string {
	parts := strings.Split(strings.TrimSpace(ref), ".")
	for i, p := range parts {
		parts[i] = pgx.Identifier{strings.TrimSpace(p)}.Sanitize()
	}
	return strings.Join(parts, ".")
}

// quoteTableExpr handles: plain, schema.table, table alias.
func quoteTableExpr(expr string) string {
	expr = strings.TrimSpace(expr)
	fields := strings.Fields(expr)
	switch {
	case len(fields) == 1:
		return quoteIdentRef(fields[0])
	case len(fields) == 2:
		return quoteIdentRef(fields[0]) + " " + pgx.Identifier{fields[1]}.Sanitize()
	case len(fields) == 3 && strings.EqualFold(fields[1], "AS"):
		return quoteIdentRef(fields[0]) + " AS " + pgx.Identifier{fields[2]}.Sanitize()
	default:
		return pgx.Identifier{expr}.Sanitize()
	}
}
