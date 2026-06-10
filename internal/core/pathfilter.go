package core

import (
	"fmt"
	"strings"
)

// validateGlobPatterns checks that none of the patterns use unsupported character classes.
func validateGlobPatterns(patterns []string) error {
	for _, p := range patterns {
		if strings.Contains(p, "[") {
			return fmt.Errorf("unsupported glob pattern (character class): %s", p)
		}
	}
	return nil
}

// pathIncludeSQL generates a SQL fragment for path inclusion filtering.
// Returns ("", nil) for empty patterns.
func pathIncludeSQL(alias string, patterns []string) (string, []any) {
	if len(patterns) == 0 {
		return "", nil
	}
	var parts []string
	var args []any
	for _, p := range patterns {
		parts = append(parts, alias+" GLOB ?")
		args = append(args, p)
	}
	return " AND (" + strings.Join(parts, " OR ") + ")", args
}

// PathExcludeSQL returns a SQL fragment and args for excluding paths.
// alias is the column expression for path (e.g. "n.path").
func (ef *ExcludeFilter) PathExcludeSQL(alias string) (string, []any) {
	if ef == nil || len(ef.PathGlobs) == 0 {
		return "", nil
	}
	var parts []string
	var args []any
	for _, g := range ef.PathGlobs {
		parts = append(parts, alias+" GLOB ?")
		args = append(args, g)
	}
	// path IS NULL protects phantom/tag nodes (NOT (NULL GLOB ?) → NULL → false in WHERE).
	return fmt.Sprintf(" AND (%s IS NULL OR NOT (%s))", alias, strings.Join(parts, " OR ")), args
}

// globMatch implements SQLite GLOB semantics in Go.
// '*' matches any sequence of characters (including '/').
// '?' matches exactly one character.
// '[' is treated as a literal character (character classes not supported).
func globMatch(pattern, s string) bool {
	return globMatchImpl([]rune(pattern), []rune(s))
}

func globMatchImpl(pattern, s []rune) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// Skip consecutive '*'.
			for len(pattern) > 0 && pattern[0] == '*' {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true
			}
			// Try matching the rest of the pattern at every position.
			for i := 0; i <= len(s); i++ {
				if globMatchImpl(pattern, s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(s) == 0 {
				return false
			}
			pattern = pattern[1:]
			s = s[1:]
		default:
			if len(s) == 0 || pattern[0] != s[0] {
				return false
			}
			pattern = pattern[1:]
			s = s[1:]
		}
	}
	return len(s) == 0
}
