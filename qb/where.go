package qb

import (
	"fmt"
	"strings"
)

// Where appends a single Condition ANDed into the WHERE clause.
func (b *Builder) Where(c Condition) *Builder {
	if err := c.Validate(); err != nil {
		b.errs = append(b.errs, err)
		return b
	}
	b.groups = append(b.groups, condGroup{cond: &c})
	return b
}

// WhereGroup appends a condGroup (single or OR-group) ANDed into WHERE.
//
//	b.WhereGroup(qb.OrGroup(
//	    qb.Where("status", qb.OpEq, "active"),
//	    qb.Where("status", qb.OpEq, "pending"),
//	))
//	→ WHERE ("status" = $1 OR "status" = $2)
func (b *Builder) WhereGroup(g condGroup) *Builder {
	for _, c := range g.group {
		if err := c.Validate(); err != nil {
			b.errs = append(b.errs, err)
			return b
		}
	}
	b.groups = append(b.groups, g)
	return b
}

func (b *Builder) buildWhereFrom(startIdx int) (clause string, args []any, err error) {
	if len(b.groups) == 0 {
		return "", nil, nil
	}

	parts := make([]string, 0, len(b.groups))
	idx := startIdx

	for _, g := range b.groups {
		if g.isOr {
			orParts := make([]string, 0, len(g.group))
			for _, cond := range g.group {
				frag, condArgs, condErr := b.renderCond(cond, idx)
				if condErr != nil {
					return "", nil, condErr
				}
				orParts = append(orParts, frag)
				args = append(args, condArgs...)
				idx += len(condArgs)
			}
			parts = append(parts, "("+strings.Join(orParts, " OR ")+")")
		} else {
			frag, condArgs, condErr := b.renderCond(*g.cond, idx)
			if condErr != nil {
				return "", nil, condErr
			}
			parts = append(parts, frag)
			args = append(args, condArgs...)
			idx += len(condArgs)
		}
	}

	return strings.Join(parts, " AND "), args, nil
}

func (b *Builder) renderCond(cond Condition, idx int) (frag string, args []any, err error) {
	col := quoteIdentRef(cond.Column)

	switch cond.Operator {
	case OpExists:
		subSQL, subArgs, subErr := cond.Sub.buildSelectFrom(idx)
		if subErr != nil {
			return "", nil, fmt.Errorf("qb: EXISTS subquery: %w", subErr)
		}
		return fmt.Sprintf("EXISTS (%s)", subSQL), subArgs, nil

	case OpNotExists:
		subSQL, subArgs, subErr := cond.Sub.buildSelectFrom(idx)
		if subErr != nil {
			return "", nil, fmt.Errorf("qb: NOT EXISTS subquery: %w", subErr)
		}
		return fmt.Sprintf("NOT EXISTS (%s)", subSQL), subArgs, nil

	case OpSubquery:
		op := cond.Value.(Operator)
		subSQL, subArgs, subErr := cond.Sub.buildSelectFrom(idx)
		if subErr != nil {
			return "", nil, fmt.Errorf("qb: subquery condition: %w", subErr)
		}
		return fmt.Sprintf("%s %s (%s)", col, string(op), subSQL), subArgs, nil

	case OpRaw:
		raw := cond.Value.(rawExpr)
		expr, rawArgs := injectParams(raw.expr, idx, raw.args)
		return expr, rawArgs, nil

	case OpIsNull:
		return fmt.Sprintf("%s IS NULL", col), nil, nil

	case OpNotNull:
		return fmt.Sprintf("%s IS NOT NULL", col), nil, nil

	case OpIn, OpNotIn:
		items, sliceErr := toAnySlice(cond.Value)
		if sliceErr != nil {
			return "", nil, fmt.Errorf("qb: %s %q: %w", cond.Operator, cond.Column, sliceErr)
		}
		if len(items) == 0 {
			return "", nil, fmt.Errorf("qb: %s %q: slice must not be empty", cond.Operator, cond.Column)
		}
		placeholders := make([]string, len(items))
		for i, v := range items {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, v)
			idx++
		}
		return fmt.Sprintf("%s %s (%s)", col, string(cond.Operator), strings.Join(placeholders, ", ")), args, nil

	case OpBetween, OpNotBetween:
		pair, betErr := toAnySlice(cond.Value)
		if betErr != nil || len(pair) != 2 {
			return "", nil, fmt.Errorf("qb: %s %q requires a 2-element slice", cond.Operator, cond.Column)
		}
		frag = fmt.Sprintf("%s %s $%d AND $%d", col, string(cond.Operator), idx, idx+1)
		return frag, pair, nil

	case OpAny, OpAll:
		// col = ANY($n) or col = ALL($n) — value bound as a PG array
		return fmt.Sprintf("%s %s($%d)", col, string(cond.Operator), idx), []any{cond.Value}, nil

	default:
		// scalar: =, <>, <, <=, >, >=, LIKE, ILIKE, @@, ?, ?&, ?|, @>, <@, &&
		return fmt.Sprintf("%s %s $%d", col, string(cond.Operator), idx), []any{cond.Value}, nil
	}
}
