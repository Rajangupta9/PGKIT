package qb

import "fmt"

type joinClause struct {
	kind      JoinType
	table     string
	condition string
	lateral   bool
	sub       *Builder
	alias     string
}

// Join appends any JOIN variant.
// table is quoted as a table expression; condition is written verbatim.
// Never pass user input directly to condition.
// For CROSS JOIN pass "" as condition.
func (b *Builder) Join(kind JoinType, table, condition string) *Builder {
	if !isJoinType(kind) {
		b.errs = append(b.errs, fmt.Errorf("qb: invalid join type %q", kind))
		return b
	}
	b.joins = append(b.joins, joinClause{kind: kind, table: table, condition: condition})
	return b
}

// InnerJoin appends an INNER JOIN clause.
func (b *Builder) InnerJoin(table, condition string) *Builder {
	return b.Join(JoinInner, table, condition)
}

// LeftJoin appends a LEFT JOIN clause.
func (b *Builder) LeftJoin(table, condition string) *Builder {
	return b.Join(JoinLeft, table, condition)
}

// RightJoin appends a RIGHT JOIN clause.
func (b *Builder) RightJoin(table, condition string) *Builder {
	return b.Join(JoinRight, table, condition)
}

// FullJoin appends a FULL JOIN clause.
func (b *Builder) FullJoin(table, condition string) *Builder {
	return b.Join(JoinFull, table, condition)
}

// LateralJoin appends a LATERAL subquery join:
//
//	LEFT JOIN LATERAL (SELECT …) AS alias ON TRUE
func (b *Builder) LateralJoin(kind JoinType, sub *Builder, alias string) *Builder {
	if !isJoinType(kind) {
		b.errs = append(b.errs, fmt.Errorf("qb: invalid join type %q", kind))
		return b
	}
	if sub == nil {
		b.errs = append(b.errs, fmt.Errorf("qb: LateralJoin subquery is nil"))
		return b
	}
	b.joins = append(b.joins, joinClause{kind: kind, lateral: true, sub: sub, alias: alias})
	return b
}

func isJoinType(kind JoinType) bool {
	switch kind {
	case JoinInner, JoinLeft, JoinRight, JoinFull, JoinCross:
		return true
	default:
		return false
	}
}
