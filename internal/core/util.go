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

// resolveToVaultRelative resolves a relative or absolute path link target
// to a vault-relative path. For basename links, the target is returned as-is.
func resolveToVaultRelative(sourcePath string, lo linkOccur) string {
	target := lo.target
	if lo.isRelative {
		return NormalizePath(filepath.Join(filepath.Dir(sourcePath), target))
	}
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(target, "/")
	}
	return target
}

// noteResolveMaps holds in-memory lookup maps for note link resolution (scan-mode).
type noteResolveMaps struct {
	basenameCounts     map[string]int
	basenameToPath     map[string]string // lower basename → path (count==1 only)
	rootBasenameToPath map[string]string // lower basename → root path
	pathSetLower       map[string]string // lower path → actual path
}

// buildNoteResolveMaps builds note resolve maps from a list of vault-relative .md file paths.
func buildNoteResolveMaps(files []string) noteResolveMaps {
	counts := countBasenames(files)
	btp := make(map[string]string)
	rbtp := make(map[string]string)
	for _, rel := range files {
		bk := basenameKey(rel)
		if counts[bk] == 1 {
			btp[bk] = rel
		}
		if isRootFile(rel) {
			rbtp[bk] = rel
		}
	}
	ps := make(map[string]string)
	for _, rel := range files {
		ps[strings.ToLower(rel)] = rel
		noExt := strings.TrimSuffix(rel, filepath.Ext(rel))
		ps[strings.ToLower(noExt)] = rel
	}
	return noteResolveMaps{
		basenameCounts:     counts,
		basenameToPath:     btp,
		rootBasenameToPath: rbtp,
		pathSetLower:       ps,
	}
}

// assetResolveMaps holds in-memory lookup maps for asset link resolution (scan-mode).
type assetResolveMaps struct {
	basenameCounts     map[string]int
	basenameToPath     map[string]string // lower asset basename → path (count==1 only)
	rootBasenameToPath map[string]string // lower asset basename → root path
	pathSetLower       map[string]string // lower path → actual path
}

// buildAssetResolveMaps builds asset resolve maps from a list of vault-relative asset file paths.
func buildAssetResolveMaps(assetFiles []string) assetResolveMaps {
	counts := countAssetBasenames(assetFiles)
	btp := make(map[string]string)
	rbtp := make(map[string]string)
	for _, rel := range assetFiles {
		abk := assetBasenameKey(rel)
		if counts[abk] == 1 {
			btp[abk] = rel
		}
		if isRootFile(rel) {
			rbtp[abk] = rel
		}
	}
	ps := make(map[string]string)
	for _, rel := range assetFiles {
		ps[strings.ToLower(rel)] = rel
	}
	return assetResolveMaps{
		basenameCounts:     counts,
		basenameToPath:     btp,
		rootBasenameToPath: rbtp,
		pathSetLower:       ps,
	}
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
