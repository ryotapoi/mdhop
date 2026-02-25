package core

import (
	"os"
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

// assetBasenameKey returns the lowercase filename with extension for asset path matching.
// Example: "sub/image.png" → "image.png"
func assetBasenameKey(path string) string {
	return strings.ToLower(filepath.Base(path))
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
// Checks note basenames first, then asset basenames (separate key spaces).
func isAmbiguousBasenameLink(target string, rm *resolveMaps) bool {
	lower := strings.ToLower(target)
	// Check note namespace.
	if rm.basenameCounts[lower] > 1 {
		return !hasRootInPathSet(lower, rm.pathSet)
	}
	if rm.basenameCounts[lower] == 1 {
		return false // unique note match
	}
	// Check asset namespace.
	if rm.assetBasenameCounts[lower] > 1 {
		return !hasRootInPathSet(lower, rm.assetPathSet)
	}
	return false
}

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
