package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestRunInitMeta_WritePreservesLegacyTemp(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		name := "file"
		if symlink {
			name = "symlink"
		}
		t.Run(name, func(t *testing.T) {
			vault := t.TempDir()
			legacy := filepath.Join(vault, "mdhop.toml.tmp")
			const contents = "unrelated file\n"
			if symlink {
				target := filepath.Join(t.TempDir(), "target")
				if err := os.WriteFile(target, []byte(contents), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, legacy); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(legacy, []byte(contents), 0644); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(legacy)
			if err != nil {
				t.Fatal(err)
			}
			captureStderr(t, func() error {
				return runInitMeta([]string{"--vault", vault, "--preset", "--no-comment", "--write"})
			})
			after, err := os.Lstat(legacy)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("legacy temp was replaced or removed: %v", err)
			}
			if got := readCLIFile(t, legacy); got != contents {
				t.Fatalf("legacy temp contents = %q, want %q", got, contents)
			}
			if symlink && after.Mode()&os.ModeSymlink == 0 {
				t.Fatal("legacy temp is no longer a symlink")
			}
			if got := readCLIFile(t, filepath.Join(vault, "mdhop.toml")); !strings.Contains(got, "created = 'date'") {
				t.Fatalf("missing completed config: %s", got)
			}
			assertNoInitMetaTemps(t, vault)
		})
	}
}

func TestRunInitMeta_WriteConcurrent(t *testing.T) {
	vault := t.TempDir()
	// Both options have complete, distinct expected results. Every writer may
	// read either published result, but must publish one whole candidate.
	var candidates []string
	for _, noComment := range []bool{false, true} {
		result, err := core.InitMeta(vault, core.InitMetaOptions{Preset: true, NoComment: noComment})
		if err != nil {
			t.Fatal(err)
		}
		candidates = append(candidates, result.TOML)
	}
	const writers = 16
	start := make(chan struct{})
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		args := []string{"--vault", vault, "--preset", "--write"}
		if i%2 == 0 {
			args = append(args, "--no-comment")
		}
		go func() {
			<-start
			errs <- runInitMeta(args)
		}()
	}
	close(start)
	for i := 0; i < writers; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent write: %v", err)
		}
	}
	got := readCLIFile(t, filepath.Join(vault, "mdhop.toml"))
	if got != candidates[0] && got != candidates[1] {
		t.Fatalf("final config is not a complete candidate:\n%s", got)
	}
	assertNoInitMetaTemps(t, vault)
}

func assertNoInitMetaTemps(t *testing.T, vault string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(vault, "mdhop.toml.tmp-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("owned temp files remain: %v (glob error: %v)", matches, err)
	}
}

func TestRunInitMetaPreservesLegacyAndInvalidConfig(t *testing.T) {
	vault := t.TempDir()
	legacy := filepath.Join(vault, "mdhop.yaml")
	const old = "meta:\n  types:\n    created: string\n"
	if err := os.WriteFile(legacy, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	captureStderr(t, func() error { return runInitMeta([]string{"--vault", vault, "--preset", "--write"}) })
	if got := readCLIFile(t, legacy); got != old {
		t.Fatal("legacy YAML changed")
	}
	cfg, err := core.LoadConfig(vault)
	if err != nil || cfg.Meta.Types["created"].Name != core.MetaTypeDate {
		t.Fatalf("legacy config was used: %+v, %v", cfg, err)
	}
	path := filepath.Join(vault, "mdhop.toml")
	const invalid = "[meta.types]\ncreated = [\n"
	if err := os.WriteFile(path, []byte(invalid), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runInitMeta([]string{"--vault", vault, "--preset", "--write"}); err == nil {
		t.Fatal("expected parse error")
	}
	if got := readCLIFile(t, path); got != invalid {
		t.Fatal("invalid TOML changed")
	}
	if got := readCLIFile(t, legacy); got != old {
		t.Fatal("legacy YAML changed on failure")
	}
	assertNoInitMetaTemps(t, vault)
}
