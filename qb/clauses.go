package qb

import (
	"fmt"
	"strings"
)

// ─── GROUP BY / HAVING ────────────────────────────────────────────────────────

// GroupBy appends columns to GROUP BY.
func (b *Builder) GroupBy(cols ...string) *Builder {
	b.groupByCols = append(b.groupByCols, cols...)
	return b
}

// Having sets a raw HAVING expression. Use ? as placeholder.
// SECURITY: expr is written verbatim; never let user input flow in.
//
//	b.Having("SUM(total) > ?", 500)
func (b *Builder) Having(expr string, args ...any) *Builder {
	b.havingExpr = expr
	b.havingArgs = args
	return b
}

// ─── ORDER BY ─────────────────────────────────────────────────────────────────

type orderClause struct {
	column string
	dir    SortDir
	nulls  NullsOrder
}

// OrderBy appends a column to ORDER BY.
//
//	b.OrderBy("created_at", qb.Desc, qb.NullsLast)
func (b *Builder) OrderBy(col string, dir SortDir, nulls ...NullsOrder) *Builder {
	if !isSortDir(dir) {
		b.errs = append(b.errs, fmt.Errorf("qb: invalid sort direction %q", dir))
		return b
	}
	o := orderClause{column: col, dir: dir}
	if len(nulls) > 0 {
		if !isNullsOrder(nulls[0]) {
			b.errs = append(b.errs, fmt.Errorf("qb: invalid nulls order %q", nulls[0]))
			return b
		}
		o.nulls = nulls[0]
	}
	b.orders = append(b.orders, o)
	return b
}

func isSortDir(dir SortDir) bool {
	return dir == Asc || dir == Desc
}

func isNullsOrder(nulls NullsOrder) bool {
	return nulls == NullsFirst || nulls == NullsLast
}

// ─── LIMIT / OFFSET ───────────────────────────────────────────────────────────

// Limit sets the maximum number of rows. 0 = no limit.
func (b *Builder) Limit(n int) *Builder { b.limitVal = n; return b }

// Offset sets the number of rows to skip. 0 = no offset.
func (b *Builder) Offset(n int) *Builder { b.offsetVal = n; return b }

// ─── Locking ──────────────────────────────────────────────────────────────────

// ForUpdate appends FOR UPDATE [NOWAIT | SKIP LOCKED].
func (b *Builder) ForUpdate(wait LockWait) *Builder {
	if !isLockWait(wait) {
		b.errs = append(b.errs, fmt.Errorf("qb: invalid lock wait %q", wait))
		return b
	}
	b.lockMode = LockForUpdate
	b.lockWait = wait
	return b
}

// ForShare appends FOR SHARE [NOWAIT | SKIP LOCKED].
func (b *Builder) ForShare(wait LockWait) *Builder {
	if !isLockWait(wait) {
		b.errs = append(b.errs, fmt.Errorf("qb: invalid lock wait %q", wait))
		return b
	}
	b.lockMode = LockForShare
	b.lockWait = wait
	return b
}

// Lock sets a custom lock mode.
func (b *Builder) Lock(mode LockMode, wait LockWait) *Builder {
	if !isLockMode(mode) {
		b.errs = append(b.errs, fmt.Errorf("qb: invalid lock mode %q", mode))
		return b
	}
	if !isLockWait(wait) {
		b.errs = append(b.errs, fmt.Errorf("qb: invalid lock wait %q", wait))
		return b
	}
	b.lockMode = mode
	b.lockWait = wait
	return b
}

func isLockMode(mode LockMode) bool {
	switch mode {
	case LockForUpdate, LockForShare, LockForNoKeyUpdate, LockForKeyShare:
		return true
	default:
		return false
	}
}

func isLockWait(wait LockWait) bool {
	switch wait {
	case Wait, NoWait, SkipLocked:
		return true
	default:
		return false
	}
}

// ─── RETURNING ────────────────────────────────────────────────────────────────

// ReturningID appends RETURNING "id". Default for BuildInsert.
func (b *Builder) ReturningID() *Builder { b.retMode = returningID; return b }

// ReturningAll appends RETURNING *.
func (b *Builder) ReturningAll() *Builder { b.retMode = returningAll; return b }

// Returning appends RETURNING col1, col2, …
func (b *Builder) Returning(cols ...string) *Builder {
	b.retMode = returningColumns
	b.retCols = cols
	return b
}

// ReturningNone suppresses any RETURNING clause, overriding the INSERT default.
func (b *Builder) ReturningNone() *Builder { b.retMode = returningNone; return b }

func (b *Builder) returningClause(def returningMode) string {
	mode := b.retMode
	if mode == returningUnset {
		mode = def // apply Build* default only when caller hasn't set anything
	}
	switch mode {
	case returningID:
		return ` RETURNING "id"`
	case returningAll:
		return " RETURNING *"
	case returningColumns:
		quoted := make([]string, len(b.retCols))
		for i, c := range b.retCols {
			quoted[i] = quoteIdentRef(c)
		}
		return " RETURNING " + strings.Join(quoted, ", ")
	default: // returningNone or any unknown — no clause
		return ""
	}
}
