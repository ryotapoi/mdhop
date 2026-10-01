package main

import (
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCommandsRejectPositionalArguments(t *testing.T) {
	commands := []struct {
		name string
		run  func([]string) error
	}{
		{"build", runBuild}, {"resolve", runResolve}, {"query", runQuery},
		{"stats", runStats}, {"diagnose", runDiagnose}, {"status", runStatus},
		{"meta-check", runMetaCheck}, {"meta-validate", runMetaValidate},
		{"delete", runDelete}, {"update", runUpdate}, {"set", runSet},
		{"add", runAdd}, {"move", runMove}, {"disambiguate", runDisambiguate},
		{"simplify", runSimplify}, {"repair", runRepair}, {"convert", runConvert},
		{"search", runSearch}, {"reachable", runReachable}, {"graph", runGraph},
		{"init-meta", runInitMeta},
	}
	for _, cmd := range commands {
		t.Run(cmd.name, func(t *testing.T) {
			err := cmd.run([]string{"--vault", filepath.Join(t.TempDir(), "missing"), "unexpected"})
			if err == nil || !strings.Contains(err.Error(), "unexpected positional argument") {
				t.Fatalf("expected positional argument error before command validation or I/O, got %v", err)
			}
		})
	}
}

func TestParseFlagsBoundaries(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"first", []string{"unexpected", "--dry-run"}, "unexpected positional argument"},
		{"middle", []string{"--file", "A.md", "unexpected", "--dry-run"}, "unexpected positional argument"},
		{"last", []string{"--dry-run", "unexpected"}, "unexpected positional argument"},
		{"separator argument", []string{"--", "unexpected"}, "unexpected positional argument"},
		{"flag values and repeats", []string{"--file", "unexpected", "--file", "--dry-run"}, ""},
		{"separator only", []string{"--dry-run", "--"}, ""},
		{"help first", []string{"--help", "unexpected"}, "help"},
		{"invalid flag first", []string{"--invalid", "unexpected"}, "flag provided but not defined"},
		{"missing value", []string{"--file"}, "flag needs an argument"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			fs.Bool("dry-run", false, "")
			var files multiString
			fs.Var(&files, "file", "")
			err := parseFlags(fs, tc.args)
			if tc.want == "help" {
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			} else if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				if tc.name == "flag values and repeats" && !reflect.DeepEqual(files, multiString{"unexpected", "--dry-run"}) {
					t.Fatalf("flag values = %v", files)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestRewriteCommandsPositionalArgumentsDoNotModifyVault(t *testing.T) {
	cases := []struct {
		name, fixture, scopeFlag string
		run                      func([]string) error
		flags                    []string
	}{
		{"convert", "vault_convert", "--file", runConvert, []string{"--to", "wikilink"}},
		{"repair", "vault_repair", "--path", runRepair, nil},
		{"simplify", "vault_simplify", "--file", runSimplify, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := setupVaultForCLI(t, tc.fixture)
			before := snapshotVaultFiles(t, vault)
			for _, tail := range [][]string{{"unexpected", "--dry-run"}, {"unexpected", tc.scopeFlag, "nonexistent.md"}} {
				args := append([]string{"--vault", vault}, tc.flags...)
				args = append(args, tail...)
				out := captureStdout(t, func() error {
					err := tc.run(args)
					if err == nil || !strings.Contains(err.Error(), "unexpected positional argument") {
						t.Fatalf("expected positional argument error, got %v", err)
					}
					return nil
				})
				if out != "" {
					t.Fatalf("unexpected success output: %q", out)
				}
				if after := snapshotVaultFiles(t, vault); !reflect.DeepEqual(before, after) {
					t.Fatal("rejected arguments modified vault files or index")
				}
			}
		})
	}
}

func snapshotVaultFiles(t *testing.T, vault string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(vault, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
