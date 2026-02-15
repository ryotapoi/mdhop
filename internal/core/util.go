package core

import (
	"path/filepath"
	"strings"
)

func normalizePath(path string) string {
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
