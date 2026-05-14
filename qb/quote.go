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

// quoteTableExpr handles: plain, schema.table, table alias.
func quoteTableExpr(expr string) string {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, `"`) || strings.HasPrefix(expr, "(") {
		return expr
	}
	if strings.Contains(expr, ".") {
		dotIdx := strings.Index(expr, ".")
		schema := expr[:dotIdx]
		rest := expr[dotIdx+1:]
		if spaceIdx := strings.Index(rest, " "); spaceIdx >= 0 {
			table := rest[:spaceIdx]
			alias := strings.TrimSpace(rest[spaceIdx+1:])
			return pgx.Identifier{schema}.Sanitize() + "." +
				pgx.Identifier{table}.Sanitize() + " " +
				pgx.Identifier{alias}.Sanitize()
		}
		return pgx.Identifier{schema}.Sanitize() + "." + pgx.Identifier{rest}.Sanitize()
	}
	if spaceIdx := strings.Index(expr, " "); spaceIdx >= 0 {
		table := expr[:spaceIdx]
		alias := strings.TrimSpace(expr[spaceIdx+1:])
		if !strings.Contains(alias, " ") {
			return pgx.Identifier{table}.Sanitize() + " " + pgx.Identifier{alias}.Sanitize()
		}
		return expr
	}
	return pgx.Identifier{expr}.Sanitize()
}
