package qb

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// BuildUpdate produces:
//
//	[WITH …] UPDATE "table" SET col=$1,… WHERE … [RETURNING …]
func (b *Builder) BuildUpdate(data map[string]any) (sql string, args []any, err error) {
	if err = b.firstError(); err != nil {
		return
	}
	if len(data) == 0 {
		return "", nil, ErrEmptyData
	}

	var sb strings.Builder
	cteArgs, idx := b.writeCTEs(&sb, 1)
	args = append(args, cteArgs...)

	setClauses, setArgs, nextIdx := b.buildSet(data, idx)
	args = append(args, setArgs...)

	fmt.Fprintf(&sb, "UPDATE %s SET %s", quoteTableExpr(b.table), setClauses)

	where, whereArgs, err := b.buildWhereFrom(nextIdx)
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

func (b *Builder) buildSet(data map[string]any, paramStart int) (clause string, args []any, nextIdx int) {
	keys := sortedKeys(data)
	parts := make([]string, len(keys))
	idx := paramStart
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s = $%d", pgx.Identifier{k}.Sanitize(), idx)
		args = append(args, data[k])
		idx++
	}
	return strings.Join(parts, ", "), args, idx
}
