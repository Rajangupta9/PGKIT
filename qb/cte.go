package qb

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type cteClause struct {
	name      string
	query     string
	args      []any
	recursive bool
}

// With prepends a CTE. CTE args are prepended to the full argument list
// so their $N parameters come first.
//
//	b.With("recent", "SELECT id FROM orders WHERE created_at > $1", cutoff)
func (b *Builder) With(name, query string, args ...any) *Builder {
	b.ctes = append(b.ctes, cteClause{name: name, query: query, args: args})
	return b
}

// WithRecursive prepends a RECURSIVE CTE.
func (b *Builder) WithRecursive(name, query string, args ...any) *Builder {
	b.ctes = append(b.ctes, cteClause{name: name, query: query, args: args, recursive: true})
	return b
}

func (b *Builder) writeCTEs(sb *strings.Builder, startIdx int) (args []any, nextIdx int) {
	nextIdx = startIdx
	if len(b.ctes) == 0 {
		return nil, nextIdx
	}

	hasRecursive := false
	for _, c := range b.ctes {
		if c.recursive {
			hasRecursive = true
			break
		}
	}

	frags := make([]string, len(b.ctes))
	for i, c := range b.ctes {
		shifted := offsetParams(c.query, startIdx-1)
		frags[i] = fmt.Sprintf("%s AS (%s)", pgx.Identifier{c.name}.Sanitize(), shifted)
		args = append(args, c.args...)
		nextIdx += len(c.args)
	}

	if hasRecursive {
		sb.WriteString("WITH RECURSIVE ")
	} else {
		sb.WriteString("WITH ")
	}
	sb.WriteString(strings.Join(frags, ", "))
	sb.WriteString(" ")
	return args, nextIdx
}
