package core

import (
	"os"
	"path/filepath"
	"strings"
)

// CleanupEmptyDirs removes empty directories left after file deletion.
// It walks from each path's parent directory upward, removing empty directories
// until it reaches vaultPath or encounters a non-empty directory.
func CleanupEmptyDirs(vaultPath string, paths []string) error {
	cleaned := make(map[string]bool)
	for _, p := range paths {
		dir := filepath.Dir(filepath.Join(vaultPath, p))
		for {
			rel, err := filepath.Rel(vaultPath, dir)
			if err != nil {
				break
			}
			rel = filepath.ToSlash(rel)
			if rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
				break // reached vault root
			}
			if cleaned[dir] {
				break
			}
			err = os.Remove(dir)
			if err != nil {
				break // non-empty or permission error
			}
			cleaned[dir] = true
			dir = filepath.Dir(dir)
		}
	}
	return nil
}
