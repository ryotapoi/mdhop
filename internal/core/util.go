package core

import (
	"fmt"
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
	vaultPath string
}

func newVaultDiskPathResolver(vaultPath string) *vaultDiskPathResolver {
	return &vaultDiskPathResolver{vaultPath: vaultPath}
}

// existingPath resolves an existing directory entry. Callers that read its
// target use Stat or ReadFile, so a dangling terminal symlink retains its spelling.
func (r *vaultDiskPathResolver) existingPath(rel string) (string, error) {
	path, err := r.resolvePath(rel, false)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(path); err != nil {
		return "", err
	}
	return path, nil
}

// movePath preserves directory-entry spelling and appends missing destination
// components. A terminal symlink is an entry to rename, rather than its target.
func (r *vaultDiskPathResolver) movePath(rel string) (string, error) {
	return r.resolvePath(rel, true)
}

// resolvePath follows only the requested components. ReadDir follows ancestor
// symlinks using normal filesystem semantics, including targets outside the vault.
func (r *vaultDiskPathResolver) resolvePath(rel string, allowMissing bool) (string, error) {
	normalized := NormalizePath(rel)
	if pathEscapesVault(normalized) {
		return "", fmt.Errorf("path escapes vault: %s", rel)
	}
	current := r.vaultPath
	if normalized == "." {
		return current, nil
	}
	components := strings.Split(normalized, "/")
	for i, component := range components {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", err
		}
		name := ""
		for _, entry := range entries {
			if normalizeTextNFC(entry.Name()) != component {
				continue
			}
			if name != "" {
				return "", fmt.Errorf("ambiguous normalized disk path: %s", filepath.Join(current, component))
			}
			name = entry.Name()
		}
		if name == "" {
			exact := filepath.Join(current, component)
			// Preserve casing aliases already supported by the host filesystem.
			if _, err := os.Lstat(exact); err == nil {
				name = component
			} else if !os.IsNotExist(err) {
				return "", err
			} else if allowMissing {
				return filepath.Join(append([]string{current}, components[i:]...)...), nil
			} else {
				return "", err
			}
		}
		current = filepath.Join(current, name)
	}
	return current, nil
}

// writablePath preserves disk spelling and checks the logical vault boundary.
func (r *vaultDiskPathResolver) writablePath(rel string) (string, error) {
	path, err := r.existingPath(rel)
	if err != nil {
		return "", err
	}
	if err := validateVaultWritePath(r.vaultPath, path); err != nil {
		return "", err
	}
	return path, nil
}

// validateVaultWritePath checks the logical vault boundary and lets ordinary
// filesystem operations follow symlinks. Dangling ancestors remain errors.
func validateVaultWritePath(vaultPath, path string) error {
	root, err := filepath.Abs(vaultPath)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if pathEscapesVault(filepath.ToSlash(rel)) {
		return fmt.Errorf("write path escapes vault: %s", path)
	}
	ancestor := path
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			_, err = filepath.EvalSymlinks(ancestor)
			return err
		}
		if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return err
		}
		ancestor = parent
	}
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
