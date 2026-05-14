// Package qb is a standalone PostgreSQL query builder.
// It has no dependency on any database driver beyond pgx (used only for
// identifier quoting). Import it without importing pool or connection code.
//
//	import "github.com/rajangupta9/pgkit/qb"
//
//	sql, args, err := qb.New("students").
//	    Columns("id", "name").
//	    Where(qb.Where("school_id", qb.OpEq, schoolID)).
//	    Where(qb.WhereExists(
//	        qb.New("classes").Where(qb.Where("id", qb.OpEq, classID)),
//	    )).
//	    OrderBy("name", qb.Asc, qb.NullsLast).
//	    Limit(20).
//	    BuildSelect()
//
// File layout (each concern lives in its own file for easy debugging):
//
//	types.go    — enum-like constants (JoinType, SortDir, LockMode, …)
//	builder.go  — Builder struct, New, Clone, lifecycle (this file)
//	quote.go    — identifier sanitization (SECURITY-CRITICAL)
//	params.go   — placeholder helpers ($N shifting, ? injection)
//	select.go   — Columns/Distinct/WindowCol + BuildSelect
//	insert.go   — OnConflict + BuildInsert + BuildInsertBatch
//	update.go   — BuildUpdate
//	delete.go   — BuildDelete
//	where.go    — Where, WhereGroup, condition rendering
//	join.go     — INNER/LEFT/RIGHT/FULL/LATERAL joins
//	cte.go      — With, WithRecursive
//	union.go    — Union, UnionAll
//	clauses.go  — GROUP BY, ORDER BY, LIMIT, OFFSET, locking, RETURNING setters
//	condition.go— Operator constants + Condition constructors
package qb

// Builder constructs parameterised PostgreSQL queries.
// All methods return the same pointer for chaining.
// Errors accumulate and are returned on the first Build* call.
type Builder struct {
	table string

	selectCols []string
	distinct   bool

	joins []joinClause

	groups []condGroup

	groupByCols []string
	havingExpr  string
	havingArgs  []any

	orders []orderClause

	limitVal  int
	offsetVal int

	unions []unionClause

	onConflict string

	retMode returningMode
	retCols []string

	ctes []cteClause

	lockMode LockMode
	lockWait LockWait

	errs []error
}

// New creates a Builder targeting table.
func New(table string) *Builder {
	return &Builder{table: table}
}

// Clone returns a deep copy of the builder. Safe to modify without affecting
// the original.
func (b *Builder) Clone() *Builder {
	if b == nil {
		return nil
	}
	nb := *b
	nb.selectCols = append([]string(nil), b.selectCols...)
	nb.joins = append([]joinClause(nil), b.joins...)
	nb.groups = append([]condGroup(nil), b.groups...)
	nb.groupByCols = append([]string(nil), b.groupByCols...)
	nb.orders = append([]orderClause(nil), b.orders...)
	nb.unions = append([]unionClause(nil), b.unions...)
	nb.retCols = append([]string(nil), b.retCols...)
	nb.ctes = append([]cteClause(nil), b.ctes...)
	nb.errs = append([]error(nil), b.errs...)
	return &nb
}

// HasReturning reports whether the builder has an explicit RETURNING clause.
func (b *Builder) HasReturning() bool {
	return b.retMode != returningUnset && b.retMode != returningNone
}

func (b *Builder) firstError() error {
	if len(b.errs) > 0 {
		return b.errs[0]
	}
	return nil
}
