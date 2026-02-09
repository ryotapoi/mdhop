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
