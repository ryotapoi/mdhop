package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

// indexFileMatcher belongs to one scan or pre-mutation validation phase.
// Do not reuse it after directory mutations: parent aliases may have changed.
type indexFileMatcher struct {
	vaultPath string
	locations []Locations
	index     string
	parents   map[string]string
}

func newIndexFileMatcher(vaultPath string, locations []Locations) *indexFileMatcher {
	return &indexFileMatcher{vaultPath: vaultPath, locations: locations, parents: make(map[string]string)}
}

// resourcePath resolves parent aliases without following the final entry,
// which build replaces and directory mutations must preserve.
func (m *indexFileMatcher) resourcePath(path string) string {
	absolute, _ := filepath.Abs(path)
	dir := filepath.Dir(absolute)
	parent, ok := m.parents[dir]
	if !ok {
		parent = dir
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			parent = resolved
		}
		m.parents[dir] = parent
	}
	return NormalizePath(filepath.Join(parent, filepath.Base(absolute)))
}

func (m *indexFileMatcher) matches(path string) (bool, error) {
	// Preserve empty-scan behavior: no placement resolution is needed until
	// there is an entry to compare.
	if m.index == "" {
		dbp, err := resolveDBPath(m.vaultPath, m.locations...)
		if err != nil {
			return false, err
		}
		m.index = m.resourcePath(dbp)
	}
	absolute := m.resourcePath(path)
	return absolute == m.index || absolute == m.index+"-journal" || absolute == m.index+"-wal" || absolute == m.index+"-shm" || strings.HasPrefix(absolute, m.index+".tmp-"), nil
}

func (m *indexFileMatcher) filter(files []string) ([]string, error) {
	result := make([]string, 0, len(files))
	for _, file := range files {
		excluded, err := m.matches(filepath.Join(m.vaultPath, file))
		if err != nil {
			return nil, err
		}
		if !excluded {
			result = append(result, file)
		}
	}
	return result, nil
}

func filterIndexFiles(vaultPath string, files []string, locations []Locations) ([]string, error) {
	return newIndexFileMatcher(vaultPath, locations).filter(files)
}

// reject keeps the selected index out of registered file operations.
func (m *indexFileMatcher) reject(path string, diskPaths *vaultDiskPathResolver) error {
	diskPath, err := deleteDiskPath(diskPaths, path)
	if err != nil {
		return err
	}
	protected, err := m.matches(diskPath)
	if err != nil {
		return err
	}
	if protected {
		return fmt.Errorf("selected index resource cannot be registered or mutated: %s", path)
	}
	return nil
}
