package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	cache, err := os.MkdirTemp("", "mdhop-core-cache-*")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("XDG_CACHE_HOME", cache); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(cache)
	os.Exit(code)
}

// dbPath keeps existing integration assertions on the selected index.
func dbPath(vault string, locations ...Locations) string {
	path, err := resolveDBPath(vault, locations...)
	if err != nil {
		panic(err)
	}
	return path
}

func ensureDataDir(vault string) (string, error) {
	dir := filepath.Dir(dbPath(vault))
	return dir, os.MkdirAll(dir, 0755)
}
