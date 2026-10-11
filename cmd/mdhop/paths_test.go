package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestPathsOutputAndNoSideEffects(t *testing.T) {
	vault := t.TempDir()
	cache := filepath.Join(t.TempDir(), "missing-cache")
	t.Setenv("XDG_CACHE_HOME", cache)
	out := captureStdout(t, func() error { return runPaths([]string{"--vault", vault, "--format", "json"}) })
	var paths core.EffectivePaths
	if err := json.Unmarshal([]byte(out), &paths); err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(paths.Vault) || !filepath.IsAbs(paths.Config) || !filepath.IsAbs(paths.DB) {
		t.Fatalf("paths=%+v", paths)
	}
	if paths.Config != filepath.Join(vault, "mdhop.toml") {
		t.Fatalf("config=%s", paths.Config)
	}
	text := captureStdout(t, func() error { return runPaths([]string{"--vault", vault}) })
	if text != "vault: "+paths.Vault+"\nconfig: "+paths.Config+"\ndb: "+paths.DB+"\n" {
		t.Fatalf("text=%s", text)
	}
	explicit := captureStdout(t, func() error {
		return runPaths([]string{"--vault", vault, "--db", "chosen.sqlite", "--config", "chosen.toml", "--format", "json"})
	})
	if err := json.Unmarshal([]byte(explicit), &paths); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if paths.DB != filepath.Join(cwd, "chosen.sqlite") || paths.Config != filepath.Join(cwd, "chosen.toml") {
		t.Fatalf("explicit=%+v", paths)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("cache created: %v", err)
	}
	entries, err := os.ReadDir(vault)
	if err != nil || len(entries) != 0 {
		t.Fatalf("vault changed: %v %v", entries, err)
	}
}

func TestPathsRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"--format", "yaml"}, {"--unknown"}, {"positional"}} {
		if err := runPaths(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if err := runPaths([]string{"--vault", filepath.Join(t.TempDir(), "missing")}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing vault: %v", err)
	}
}
