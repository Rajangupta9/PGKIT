package qb

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ─── SELECT column setters ───────────────────────────────────────────────────

// Columns appends expressions to the SELECT list.
// Plain column names are auto-quoted; expressions like "COUNT(*)", "u.email",
// "SUM(total) AS total" are written verbatim.
func (b *Builder) Columns(cols ...string) *Builder {
	b.selectCols = append(b.selectCols, cols...)
	return b
}

// Distinct adds DISTINCT to the SELECT clause.
func (b *Builder) Distinct() *Builder {
	b.distinct = true
	return b
}

// WindowCol appends a window function expression to the SELECT list.
//
// fn is the window function call (e.g. "RANK()", "SUM(total)", "LAG(price)").
// partitionBy is the PARTITION BY column (empty string = no partition).
// orderBy is the ORDER BY expression written verbatim (e.g. "total DESC").
// alias is the AS name for the column.
//
// SECURITY: fn and orderBy are written verbatim — never pass user input.
//
//	qb.New("orders").
//	    Columns("user_id", "country", "total").
//	    WindowCol("RANK()", "country", "total DESC", "rank").
//	    WindowCol("SUM(total)", "country", "created_at", "running_total")
//	→ SELECT "user_id", "country", "total",
//	         RANK() OVER (PARTITION BY "country" ORDER BY total DESC) AS "rank",
//	         SUM(total) OVER (PARTITION BY "country" ORDER BY created_at) AS "running_total"
func (b *Builder) WindowCol(fn, partitionBy, orderBy, alias string) *Builder {
	var over string
	switch {
	case partitionBy != "" && orderBy != "":
		over = fmt.Sprintf("PARTITION BY %s ORDER BY %s", quoteIdentExpr(partitionBy), orderBy)
	case partitionBy != "":
		over = fmt.Sprintf("PARTITION BY %s", quoteIdentExpr(partitionBy))
	case orderBy != "":
		over = fmt.Sprintf("ORDER BY %s", orderBy)
	}
	expr := fmt.Sprintf("%s OVER (%s) AS %s", fn, over, pgx.Identifier{alias}.Sanitize())
	b.selectCols = append(b.selectCols, expr)
	return b
}

// ─── SELECT build ────────────────────────────────────────────────────────────

// BuildSelect produces a full parameterised SELECT statement starting at $1.
func (b *Builder) BuildSelect() (sql string, args []any, err error) {
	sql, args, err = b.buildSelectFrom(1)
	if err != nil {
		return
	}
	if len(args) > MaxQueryParams {
		return "", nil, tooManyParamsErr(len(args))
	}
	return
}

// buildSelectFrom produces SELECT with parameters starting at paramStart.
// Used internally for subqueries, UNION segments, and lateral joins so that
// parameter numbers continue correctly from the outer query.
func (b *Builder) buildSelectFrom(paramStart int) (string, []any, error) {
	if err := b.firstError(); err != nil {
		return "", nil, err
	}

	var sb strings.Builder
	var args []any
	idx := paramStart

	// WITH
	cteArgs, next := b.writeCTEs(&sb, idx)
	args = append(args, cteArgs...)
	idx = next

	sb.WriteString("SELECT ")
	if b.distinct {
		sb.WriteString("DISTINCT ")
	}
	sb.WriteString(b.renderSelectCols())
	sb.WriteString(" FROM ")
	sb.WriteString(quoteTableExpr(b.table))

	// JOINs
	for _, j := range b.joins {
		switch {
		case j.lateral:
			subSQL, subArgs, err := j.sub.buildSelectFrom(idx)
			if err != nil {
				return "", nil, fmt.Errorf("qb: lateral join: %w", err)
			}
			args = append(args, subArgs...)
			idx += len(subArgs)
			fmt.Fprintf(&sb, " %s JOIN LATERAL (%s) %s ON TRUE",
				j.kind, subSQL, pgx.Identifier{j.alias}.Sanitize())
		case j.kind == JoinCross:
			fmt.Fprintf(&sb, " CROSS JOIN %s", quoteTableExpr(j.table))
		default:
			fmt.Fprintf(&sb, " %s JOIN %s ON %s", j.kind, quoteTableExpr(j.table), j.condition)
		}
	}

	// WHERE
	where, whereArgs, err := b.buildWhereFrom(idx)
	if err != nil {
		return "", nil, err
	}
	if where != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(where)
	}
	args = append(args, whereArgs...)
	idx += len(whereArgs)

	// GROUP BY
	if len(b.groupByCols) > 0 {
		quoted := make([]string, len(b.groupByCols))
		for i, c := range b.groupByCols {
			quoted[i] = quoteIdentExpr(c)
		}
		sb.WriteString(" GROUP BY ")
		sb.WriteString(strings.Join(quoted, ", "))
	}

	// HAVING
	if b.havingExpr != "" {
		expr, hArgs := injectParams(b.havingExpr, idx, b.havingArgs)
		sb.WriteString(" HAVING ")
		sb.WriteString(expr)
		args = append(args, hArgs...)
		idx += len(hArgs)
	}

	// ORDER BY
	if len(b.orders) > 0 {
		parts := make([]string, len(b.orders))
		for i, o := range b.orders {
			part := fmt.Sprintf("%s %s", quoteIdentRef(o.column), o.dir)
			if o.nulls != "" {
				part += " " + string(o.nulls)
			}
			parts[i] = part
		}
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(parts, ", "))
	}

	// LIMIT / OFFSET — written as integer literals, never user data
	if b.limitVal > 0 {
		fmt.Fprintf(&sb, " LIMIT %d", b.limitVal)
	}
	if b.offsetVal > 0 {
		fmt.Fprintf(&sb, " OFFSET %d", b.offsetVal)
	}

	// Locking
	if b.lockMode != "" {
		sb.WriteString(" ")
		sb.WriteString(string(b.lockMode))
		if b.lockWait != Wait {
			sb.WriteString(" ")
			sb.WriteString(string(b.lockWait))
		}
	}

	// UNION / UNION ALL — each segment continues parameter numbering from idx
	for _, u := range b.unions {
		subSQL, subArgs, err := u.qb.buildSelectFrom(idx)
		if err != nil {
			return "", nil, fmt.Errorf("qb: UNION: %w", err)
		}
		if u.all {
			sb.WriteString(" UNION ALL ")
		} else {
			sb.WriteString(" UNION ")
		}
		sb.WriteString(subSQL)
		args = append(args, subArgs...)
		idx += len(subArgs)
	}

	_ = idx
	return sb.String(), args, nil
}

func (b *Builder) renderSelectCols() string {
	if len(b.selectCols) == 0 {
		return "*"
	}
	parts := make([]string, len(b.selectCols))
	for i, c := range b.selectCols {
		parts[i] = quoteIdentExpr(c)
	}
	return strings.Join(parts, ", ")
}
