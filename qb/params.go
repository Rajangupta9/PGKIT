package qb

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var paramRe = regexp.MustCompile(`\$(\d+)`)

// offsetParams shifts all $N placeholders in sql by offset.
// e.g. offset=2 turns $1 → $3, $2 → $4.
func offsetParams(sql string, offset int) string {
	if offset <= 0 {
		return sql
	}
	return paramRe.ReplaceAllStringFunc(sql, func(m string) string {
		n, _ := strconv.Atoi(m[1:])
		return fmt.Sprintf("$%d", n+offset)
	})
}

// injectParams replaces ? placeholders in expr with $N starting at startIdx.
func injectParams(expr string, startIdx int, args []any) (string, []any) {
	for i := range args {
		expr = strings.Replace(expr, "?", fmt.Sprintf("$%d", startIdx+i), 1)
	}
	return expr, args
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
