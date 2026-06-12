package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

func normalizeTextNFC(s string) string {
	return norm.NFC.String(s)
}

// NormalizePath cleans a vault-relative path: forward slashes, no leading "./".
func NormalizePath(path string) string {
	clean := filepath.ToSlash(filepath.Clean(path))
	return normalizeTextNFC(strings.TrimPrefix(clean, "./"))
}

func basename(path string) string {
	base := filepath.Base(path)
	return normalizeTextNFC(strings.TrimSuffix(base, filepath.Ext(base)))
}

// countLines returns the number of lines in content, counting the whole file
// (frontmatter included). An empty file is 0 lines; a non-empty final line
// without a trailing newline still counts as a line.
func countLines(content string) int {
	if content == "" {
		return 0
	}
	n := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") {
		n++
	}
	return n
}

func basenameKey(path string) string {
	return strings.ToLower(basename(path))
}

// assetBasenameKey returns the lowercase filename with extension for asset path matching.
// Example: "sub/image.png" → "image.png"
func assetBasenameKey(path string) string {
	return strings.ToLower(normalizeTextNFC(filepath.Base(path)))
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
	lower := strings.ToLower(normalizeTextNFC(target))
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

// ambiguousCandidates returns sorted candidate paths for an ambiguous basename link.
// Must only be called when isAmbiguousBasenameLink returns true.
// Same collection pattern as simplify.go:collectNoteBasenameFiles but for resolveMaps.
func ambiguousCandidates(target string, rm *resolveMaps) []string {
	lower := strings.ToLower(normalizeTextNFC(target))
	var paths []string
	if rm.basenameCounts[lower] > 1 {
		// Note namespace.
		for lp, actual := range rm.pathSet {
			if basenameKey(actual) == lower && strings.HasSuffix(lp, ".md") {
				paths = append(paths, actual)
			}
		}
	} else {
		// Asset namespace.
		for _, actual := range rm.assetPathSet {
			if assetBasenameKey(actual) == lower {
				paths = append(paths, actual)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

// validateParsedLinks checks path links for vault escape and ambiguous
// basenames against rm, returning the first error (same logic as build's
// inline validation). Used by add/update/move/move_dir before edge creation;
// the map-based resolveLink falls back to phantom on unresolved basenames,
// so ambiguity must be rejected here.
func validateParsedLinks(sourcePath string, links []linkOccur, rm *resolveMaps) error {
	for _, link := range links {
		if !isPathLinkType(link.linkType) {
			continue
		}
		if link.isRelative && escapesVault(sourcePath, link.target) {
			return fmt.Errorf("%w: %s in %s", ErrLinkEscapesVault, link.rawLink, sourcePath)
		}
		if !link.isRelative && !link.isBasename && pathEscapesVault(link.target) {
			return fmt.Errorf("%w: %s in %s", ErrLinkEscapesVault, link.rawLink, sourcePath)
		}
		if link.isBasename && isAmbiguousBasenameLink(link.target, rm) {
			candidates := ambiguousCandidates(link.target, rm)
			return fmt.Errorf("%w: %s in %s (candidates: %s)", ErrAmbiguousLink, link.target, sourcePath, strings.Join(candidates, ", "))
		}
	}
	return nil
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
		return NormalizePath(strings.TrimPrefix(target, "/"))
	}
	return NormalizePath(target)
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
