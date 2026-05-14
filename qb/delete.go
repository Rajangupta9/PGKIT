package qb

import (
	"fmt"
	"strings"
)

// BuildDelete produces:
//
//	[WITH …] DELETE FROM "table" WHERE … [RETURNING …]
func (b *Builder) BuildDelete() (sql string, args []any, err error) {
	if err = b.firstError(); err != nil {
		return
	}

	var sb strings.Builder
	cteArgs, idx := b.writeCTEs(&sb, 1)
	args = append(args, cteArgs...)

	fmt.Fprintf(&sb, "DELETE FROM %s", quoteTableExpr(b.table))

	where, whereArgs, err := b.buildWhereFrom(idx)
	if err != nil {
		return "", nil, err
	}
	if where != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(where)
	}
	args = append(args, whereArgs...)
	sb.WriteString(b.returningClause(returningUnset))

	if len(args) > MaxQueryParams {
		return "", nil, tooManyParamsErr(len(args))
	}
	return sb.String(), args, nil
}
