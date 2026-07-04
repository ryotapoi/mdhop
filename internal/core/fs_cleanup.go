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

// HasNonMDFiles checks whether the given directory (vault-relative) contains
// any non-.md files on disk. Hidden files/directories (starting with ".") are
// ignored. Returns the first non-.md path found (vault-relative), or "" if none.
func HasNonMDFiles(vaultPath, dirPrefix string) (string, error) {
	absDir := filepath.Join(vaultPath, dirPrefix)
	var found string
	err := filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		// Skip hidden files/directories.
		name := info.Name()
		if strings.HasPrefix(name, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			rel, _ := filepath.Rel(vaultPath, path)
			found = filepath.ToSlash(rel)
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return found, nil
}
