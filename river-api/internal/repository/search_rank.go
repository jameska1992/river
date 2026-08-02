package repository

import "strings"

// simThreshold is the pg_trgm similarity cutoff for fuzzy matching. It mirrors
// pg_trgm's own default (0.3): high enough to reject unrelated titles, low
// enough to tolerate typos like "inceptoin" matching "inception".
const simThreshold = "0.3"

// fuzzyMatch returns a SQL predicate (and its args) that is true when col
// contains q as a substring OR is trigram-similar to q above simThreshold. col
// must be a trusted column expression (e.g. "LOWER(title)"), never user input;
// q is the already-lowercased query.
func fuzzyMatch(col, q string) (string, []any) {
	return col + " LIKE ? OR similarity(" + col + ", ?) > " + simThreshold,
		[]any{"%" + q + "%", q}
}

// fuzzyMatchAny ORs fuzzyMatch across several columns, so a row matches when any
// column matches. Used for audiobooks, which match on title or author.
func fuzzyMatchAny(cols []string, q string) (string, []any) {
	parts := make([]string, len(cols))
	args := make([]any, 0, len(cols)*2)
	for i, c := range cols {
		p, a := fuzzyMatch(c, q)
		parts[i] = "(" + p + ")"
		args = append(args, a...)
	}
	return strings.Join(parts, " OR "), args
}

// relevanceOrder returns a SQL ORDER BY expression (and its args) ranking rows
// by how well col matches q: exact match first, then prefix, then word-start,
// then any substring, then everything else; ties broken by trigram similarity
// (closest first) and finally by sortCol alphabetically.
//
// This is what fixes the "searching X returns everything with an x in it but
// not the film X" bug: the exact title lands in bucket 0 regardless of how many
// incidental substring matches sort ahead of it alphabetically.
func relevanceOrder(col, sortCol, q string) (string, []any) {
	expr := "CASE" +
		" WHEN " + col + " = ? THEN 0" +
		" WHEN " + col + " LIKE ? THEN 1" +
		" WHEN " + col + " LIKE ? THEN 2" +
		" WHEN " + col + " LIKE ? THEN 3" +
		" ELSE 4 END, similarity(" + col + ", ?) DESC, " + sortCol
	return expr, []any{q, q + "%", "% " + q + "%", "%" + q + "%", q}
}
