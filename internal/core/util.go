package core

import (
	"os"
	"path/filepath"
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

type vaultDiskPathResolver struct {
	vaultPath       string
	normalizedPaths map[string]string
}

func newVaultDiskPathResolver(vaultPath string) *vaultDiskPathResolver {
	return &vaultDiskPathResolver{vaultPath: vaultPath}
}

func (r *vaultDiskPathResolver) existingPath(rel string) (string, error) {
	normalized := NormalizePath(rel)
	exact := filepath.Join(r.vaultPath, normalized)
	if _, err := os.Stat(exact); err == nil {
		return exact, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if r.normalizedPaths == nil {
		paths, err := collectNormalizedDiskPaths(r.vaultPath)
		if err != nil {
			return "", err
		}
		r.normalizedPaths = paths
	}
	if found, ok := r.normalizedPaths[normalized]; ok {
		return found, nil
	}
	return "", &os.PathError{Op: "stat", Path: exact, Err: os.ErrNotExist}
}

func collectNormalizedDiskPaths(vaultPath string) (map[string]string, error) {
	paths := make(map[string]string)
	err := filepath.WalkDir(vaultPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == dataDirName {
			return filepath.SkipDir
		}
		actualRel, err := filepath.Rel(vaultPath, path)
		if err != nil {
			return err
		}
		normalized := NormalizePath(actualRel)
		if _, ok := paths[normalized]; !ok {
			paths[normalized] = path
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
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
