package qb

import "strings"

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
	if b == nil {
		return ErrNilBuilder
	}
	if len(b.errs) > 0 {
		return b.errs[0]
	}
	if strings.TrimSpace(b.table) == "" {
		return ErrNoTable
	}
	return nil
}
