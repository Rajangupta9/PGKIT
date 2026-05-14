package qb

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// OnConflict sets the ON CONFLICT clause for INSERT (upsert).
// The clause is written verbatim — never let user input flow in.
//
//	b.OnConflict("(email) DO UPDATE SET name = EXCLUDED.name, updated_at = NOW()")
//	b.OnConflict("(id) DO NOTHING")
func (b *Builder) OnConflict(clause string) *Builder {
	b.onConflict = clause
	return b
}

// BuildInsert produces:
//
//	[WITH …] INSERT INTO "table" (cols) VALUES ($1,…) [ON CONFLICT …] RETURNING "id"
//
// Default RETURNING is "id". Override with ReturningAll() / Returning(…) / ReturningNone().
func (b *Builder) BuildInsert(data map[string]any) (sql string, args []any, err error) {
	if err = b.firstError(); err != nil {
		return
	}
	if len(data) == 0 {
		return "", nil, ErrEmptyData
	}

	var sb strings.Builder
	cteArgs, idx := b.writeCTEs(&sb, 1)
	args = append(args, cteArgs...)

	cols, vals, params := b.flattenData(data, idx)
	args = append(args, vals...)

	fmt.Fprintf(&sb, "INSERT INTO %s (%s) VALUES (%s)", quoteTableExpr(b.table), cols, params)

	if b.onConflict != "" {
		sb.WriteString(" ON CONFLICT ")
		sb.WriteString(b.onConflict)
	}
	sb.WriteString(b.returningClause(returningID))

	if len(args) > MaxQueryParams {
		return "", nil, tooManyParamsErr(len(args))
	}
	return sb.String(), args, nil
}

// BuildInsertBatch inserts multiple rows in one statement.
//
//	INSERT INTO "table" (col1, col2) VALUES ($1,$2), ($3,$4), … RETURNING "id"
//
// All rows must have identical key sets (first row defines the column list).
func (b *Builder) BuildInsertBatch(rows []map[string]any) (sql string, args []any, err error) {
	if err = b.firstError(); err != nil {
		return
	}
	if len(rows) == 0 {
		return "", nil, ErrEmptyRows
	}

	var sb strings.Builder
	cteArgs, idx := b.writeCTEs(&sb, 1)
	args = append(args, cteArgs...)

	keys := sortedKeys(rows[0])
	if len(keys) == 0 {
		return "", nil, fmt.Errorf("qb: batch row 0 has no columns")
	}
	quotedCols := make([]string, len(keys))
	for i, k := range keys {
		quotedCols[i] = pgx.Identifier{k}.Sanitize()
	}
	fmt.Fprintf(&sb, "INSERT INTO %s (%s) VALUES ", quoteTableExpr(b.table), strings.Join(quotedCols, ", "))

	rowFrags := make([]string, len(rows))
	for r, row := range rows {
		placeholders := make([]string, len(keys))
		for i, k := range keys {
			val, ok := row[k]
			if !ok {
				return "", nil, fmt.Errorf("qb: batch row %d missing column %q", r, k)
			}
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, val)
			idx++
		}
		rowFrags[r] = "(" + strings.Join(placeholders, ", ") + ")"
	}
	sb.WriteString(strings.Join(rowFrags, ", "))

	if b.onConflict != "" {
		sb.WriteString(" ON CONFLICT ")
		sb.WriteString(b.onConflict)
	}
	sb.WriteString(b.returningClause(returningID))

	if len(args) > MaxQueryParams {
		return "", nil, tooManyParamsErr(len(args))
	}
	return sb.String(), args, nil
}

func (b *Builder) flattenData(data map[string]any, startIdx int) (cols string, vals []any, params string) {
	keys := sortedKeys(data)
	quotedCols := make([]string, len(keys))
	placeholders := make([]string, len(keys))
	for i, k := range keys {
		quotedCols[i] = pgx.Identifier{k}.Sanitize()
		vals = append(vals, data[k])
		placeholders[i] = fmt.Sprintf("$%d", startIdx+i)
	}
	cols = strings.Join(quotedCols, ", ")
	params = strings.Join(placeholders, ", ")
	return
}
