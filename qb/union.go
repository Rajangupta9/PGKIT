package qb

import "fmt"

type unionClause struct {
	all bool
	qb  *Builder
}

// Union appends UNION (deduplicating) with another builder.
func (b *Builder) Union(other *Builder) *Builder {
	if other == nil {
		b.errs = append(b.errs, fmt.Errorf("qb: Union builder is nil"))
		return b
	}
	b.unions = append(b.unions, unionClause{all: false, qb: other})
	return b
}

// UnionAll appends UNION ALL (no deduplication).
func (b *Builder) UnionAll(other *Builder) *Builder {
	if other == nil {
		b.errs = append(b.errs, fmt.Errorf("qb: UnionAll builder is nil"))
		return b
	}
	b.unions = append(b.unions, unionClause{all: true, qb: other})
	return b
}
