package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestMigrateCLI(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("A"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--db", "unused"}, {"--config", "unused"}, {"positional"}} {
		if err := runMigrate(append([]string{"--vault", vault}, args...)); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	paths, err := core.Paths(vault)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.DB); !os.IsNotExist(err) {
		t.Fatal("invalid flags created index")
	}
	if err := runMigrate([]string{"--help"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help: %v", err)
	}
	if err := runMigrate([]string{"--vault", vault}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Status(vault); err != nil {
		t.Fatal(err)
	}
}
