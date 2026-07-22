package services

import "strings"

// titleSortExpr returns the SQL expression we use to order a text column
// alphabetically. Applied in order:
//
//  1. Strip punctuation (keep letters, digits, whitespace) so a leading
//     `"` or `'` doesn't push a title to the top, and `Wall-E` sorts
//     near `Walle` rather than near `Wal*` symbols.
//  2. Trim — punctuation followed by a space (`" Hello`) would otherwise
//     leave a leading space that throws off ordering.
//  3. Strip a leading English article (the/a/an) so "The Matrix" sorts
//     with "Matrix" and "A Beautiful Mind" sorts under B.
//  4. Lowercase for case-insensitive ordering.
//
// `col` is interpolated into a SQL fragment with no escaping, so callers
// must pass a literal column name, never user input. This file is the
// only producer of these expressions and they're all hard-coded.
func titleSortExpr(col string) string {
	return `LOWER(REGEXP_REPLACE(TRIM(REGEXP_REPLACE(` + col + `, '[^[:alnum:][:space:]]', '', 'g')), '^(the|a|an)\s+', '', 'i'))`
}

// sortClause maps a user-facing field name to a SQL expression via the
// supplied whitelist, appends a safe direction, and returns the result as
// an ORDER BY expression suitable to hand to GORM's Order().
//
// `field` is the user-supplied sort key (e.g. "title", "year"). If it's not
// in `whitelist`, the fallback expression is used so an unknown value
// never makes the request fail.
//
// Whitelist values are SQL expressions, not just column names — text
// columns are typically wrapped in LOWER() so alphabetical sort is
// case-insensitive. The whitelist keeps the SQL safe from injection: the
// only strings that ever reach the database come from this package's own
// maps and the matching fallback literal.
//
// `order` is "asc" or "desc" (case-insensitive); anything else defaults to
// ASC.
func sortClause(field, order string, whitelist map[string]string, fallback string) string {
	col, ok := whitelist[strings.ToLower(strings.TrimSpace(field))]
	if !ok {
		col = fallback
	}
	dir := "ASC"
	if strings.EqualFold(strings.TrimSpace(order), "desc") {
		dir = "DESC"
	}
	return col + " " + dir
}
