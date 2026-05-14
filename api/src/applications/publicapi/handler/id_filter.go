package handler

import "strings"

// buildInClause returns "IN (?, ?, ?)" with the placeholder count
// matching `n`. Returns "IN (NULL)" when n == 0 so a deny-all list
// produces a well-formed query that returns zero rows.
func buildInClause(n int) string {
	if n == 0 {
		return "IN (NULL)"
	}
	return "IN (" + strings.Repeat("?,", n-1) + "?)"
}

// stringArgs turns a []string into a []interface{} suitable for
// splatting into sql.Exec/Query.
func stringArgs(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
