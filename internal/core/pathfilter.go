package core

import (
	"fmt"
	"strings"
)

// validateGlobPatterns checks that none of the patterns use unsupported character classes.
func validateGlobPatterns(patterns []string) error {
	for _, p := range patterns {
		if strings.Contains(NormalizePath(p), "[") {
			return fmt.Errorf("unsupported glob pattern (character class): %s", p)
		}
	}
	return nil
}

// globOrSQL builds "alias GLOB ? OR alias GLOB ? ..." and its args.
func globOrSQL(alias string, patterns []string) (string, []any) {
	var parts []string
	var args []any
	for _, p := range patterns {
		parts = append(parts, alias+" GLOB ?")
		args = append(args, NormalizePath(p))
	}
	return strings.Join(parts, " OR "), args
}

// pathIncludeSQL generates a SQL fragment for path inclusion filtering.
// Returns ("", nil) for empty patterns.
func pathIncludeSQL(alias string, patterns []string) (string, []any) {
	if len(patterns) == 0 {
		return "", nil
	}
	globs, args := globOrSQL(alias, patterns)
	return " AND (" + globs + ")", args
}

// pathIncludeNullSafeSQL generates a SQL fragment for path inclusion filtering
// that keeps NULL-path nodes (phantom/tag), mirroring the NULL protection of
// PathExcludeSQL. Returns ("", nil) for empty patterns.
func pathIncludeNullSafeSQL(alias string, patterns []string) (string, []any) {
	if len(patterns) == 0 {
		return "", nil
	}
	globs, args := globOrSQL(alias, patterns)
	return " AND (" + alias + " IS NULL OR " + globs + ")", args
}

// PathExcludeSQL returns a SQL fragment and args for excluding paths.
// alias is the column expression for path (e.g. "n.path").
func (ef *ExcludeFilter) PathExcludeSQL(alias string) (string, []any) {
	if ef == nil || len(ef.PathGlobs) == 0 {
		return "", nil
	}
	globs, args := globOrSQL(alias, ef.PathGlobs)
	// path IS NULL protects phantom/tag nodes (NOT (NULL GLOB ?) → NULL → false in WHERE).
	return fmt.Sprintf(" AND (%s IS NULL OR NOT (%s))", alias, globs), args
}

// globMatch implements SQLite GLOB semantics in Go.
// '*' matches any sequence of characters (including '/').
// '?' matches exactly one character.
// '[' is treated as a literal character (character classes not supported).
func globMatch(pattern, s string) bool {
	return globMatchImpl([]rune(pattern), []rune(s))
}

func pathMatchesFilters(path string, include, exclude []string) bool {
	path = NormalizePath(path)
	if len(include) > 0 {
		matched := false
		for _, pattern := range include {
			if globMatch(NormalizePath(pattern), path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, pattern := range exclude {
		if globMatch(NormalizePath(pattern), path) {
			return false
		}
	}
	return true
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
