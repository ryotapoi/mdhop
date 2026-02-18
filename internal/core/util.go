package core

import (
	"path/filepath"
	"strings"
)

// NormalizePath cleans a vault-relative path: forward slashes, no leading "./".
func NormalizePath(path string) string {
	clean := filepath.ToSlash(filepath.Clean(path))
	return strings.TrimPrefix(clean, "./")
}

func basename(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func basenameKey(path string) string {
	return strings.ToLower(basename(path))
}

// isRootFile returns true if the path has no directory component (root-level file).
func isRootFile(path string) bool {
	return !strings.Contains(path, "/")
}

// hasRootInPathSet checks if pathSet contains a root-level file for the given basename key.
func hasRootInPathSet(bk string, pathSet map[string]string) bool {
	p, ok := pathSet[bk]
	return ok && isRootFile(p)
}

// isAmbiguousBasenameLink checks if a basename link is ambiguous.
// Returns true if the basename has multiple files AND there is no root-level file.
// When a root-level file exists, the basename link resolves to it (root-priority rule).
func isAmbiguousBasenameLink(target string, basenameCounts map[string]int, pathSet map[string]string) bool {
	lower := strings.ToLower(target)
	if basenameCounts[lower] <= 1 {
		return false
	}
	return !hasRootInPathSet(lower, pathSet)
}

// isFieldActive returns true if the field is requested (or if fields is empty, meaning all).
func isFieldActive(field string, fields []string) bool {
	if len(fields) == 0 {
		return true
	}
	for _, f := range fields {
		if f == field {
			return true
		}
	}
	return false
}
