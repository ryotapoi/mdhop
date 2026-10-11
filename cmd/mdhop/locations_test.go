package main

import (
	"errors"
	"github.com/ryotapoi/mdhop/internal/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandsSelectConfigAndAcceptDB(t *testing.T) {
	// This matrix protects every CLI's location flag parsing and config selection.
	commands := map[string]func([]string) error{
		"build": runBuild, "resolve": runResolve, "inspect": runInspect, "query": runQuery,
		"stats": runStats, "diagnose": runDiagnose, "status": runStatus, "meta-check": runMetaCheck,
		"meta-validate": runMetaValidate, "delete": runDelete, "update": runUpdate, "set": runSet,
		"add": runAdd, "move": runMove, "disambiguate": runDisambiguate, "simplify": runSimplify,
		"repair": runRepair, "convert": runConvert, "search": runSearch, "reachable": runReachable,
		"graph": runGraph, "init-meta": runInitMeta,
	}
	vault := t.TempDir()
	config := filepath.Join(t.TempDir(), "chosen.toml")
	if err := os.WriteFile(config, []byte("invalid = ["), 0600); err != nil {
		t.Fatal(err)
	}
	for name, run := range commands {
		t.Run(name, func(t *testing.T) {
			args := []string{"--vault", vault, "--db", filepath.Join(t.TempDir(), "external.sqlite"), "--config", config}
			if name == "query" {
				args = append(args, "--no-config-hide", "--no-config-via")
			}
			if name == "search" {
				args = append(args, "--no-exclude")
			}
			err := run(args)
			if err == nil || !strings.Contains(err.Error(), config) {
				t.Fatalf("selected configuration not validated: %v", err)
			}
		})
	}
}

func TestInitMetaWritesSelectedConfig(t *testing.T) {
	vault := t.TempDir()
	config := filepath.Join(t.TempDir(), "chosen.toml")
	original := []byte("[build]\nexclude_paths=['private/**']\n")
	if err := os.WriteFile(config, original, 0600); err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(vault, "mdhop.toml")
	if err := os.WriteFile(defaultPath, []byte("broken = ["), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runInitMeta([]string{"--vault", vault, "--config", config, "--db", filepath.Join(vault, "never.sqlite"), "--preset", "--write"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(config)
	if !strings.Contains(string(got), "private/**") || !strings.Contains(string(got), "[meta.types]") {
		t.Fatalf("selected config=%s", got)
	}
	unchanged, _ := os.ReadFile(defaultPath)
	if string(unchanged) != "broken = [" {
		t.Fatal("default config changed")
	}
	if _, err := os.Stat(filepath.Join(vault, "never.sqlite")); !os.IsNotExist(err) {
		t.Fatal("init-meta created DB")
	}
}

func TestCommandsUseSelectedDB(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("A\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// A usable default index means dropping --db would produce a different result.
	if err := runBuild([]string{"--vault", vault}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		run  func([]string) error
		args []string
	}{
		{"resolve", runResolve, []string{"--from", "A.md", "--link", "[[A]]"}},
		{"inspect", runInspect, []string{"--file", "A.md"}},
		{"query", runQuery, []string{"--file", "A.md"}},
		{"stats", runStats, nil}, {"diagnose", runDiagnose, nil}, {"status", runStatus, nil},
		{"meta-check", runMetaCheck, []string{"--key", "related"}}, {"meta-validate", runMetaValidate, []string{"--require", "rank"}},
		{"delete", runDelete, []string{"--file", "A.md"}}, {"update", runUpdate, []string{"--file", "A.md"}},
		{"set", runSet, []string{"--file", "A.md", "--key", "rank", "--value", "2"}},
		{"add", runAdd, []string{"--file", "New.md"}},
		{"move", runMove, []string{"--from", "A.md", "--to", "B.md"}},
		{"move directory", runMove, []string{"--from", "sub/", "--to", "dest/"}},
		{"move template", runMove, []string{"--from", "A.md", "--to-template", "{basename}.md"}},
		{"disambiguate", runDisambiguate, []string{"--name", "A"}},
		{"search", runSearch, nil}, {"reachable", runReachable, []string{"--from", "A.md"}}, {"graph", runGraph, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"--vault", vault, "--db", filepath.Join(t.TempDir(), "missing.sqlite")}, tc.args...)
			err := tc.run(args)
			if !errors.Is(err, core.ErrIndexNotFound) {
				t.Fatalf("selected DB not used: %v", err)
			}
		})
	}
}
