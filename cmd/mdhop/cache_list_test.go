package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestCacheSizeUnits(t *testing.T) {
	for _, tc := range []struct {
		size int64
		want string
	}{{0, "0 B"}, {512, "512 B"}, {1023, "1023 B"}, {1024, "1.0 KiB"}, {1536, "1.5 KiB"}, {120 << 20, "120.0 MiB"}, {1 << 30, "1.0 GiB"}, {1 << 60, "1.0 EiB"}} {
		if got := formatCacheSize(tc.size); got != tc.want {
			t.Errorf("%d = %s, want %s", tc.size, got, tc.want)
		}
	}
}

func TestCacheListFormatsAndWriterErrors(t *testing.T) {
	vault := "/original/vault"
	entries := []core.CacheEntry{{CachePath: "/cache/a", SizeBytes: 512, Vault: &vault}, {CachePath: "/cache/b", SizeBytes: 1536}}
	var out bytes.Buffer
	if err := printCacheListText(&out, entries); err != nil {
		t.Fatal(err)
	}
	want := "/cache/a\nsize: 512 B  original: /original/vault\n\n/cache/b\nsize: 1.5 KiB  original: unknown\n"
	if out.String() != want {
		t.Fatalf("text=%q", out.String())
	}
	if err := printCacheListText(cacheFailWriter{}, entries); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("writer error=%v", err)
	}
	if err := encodeJSON(cacheFailWriter{}, entries); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("JSON writer error=%v", err)
	}
}

type cacheFailWriter struct{}

func (cacheFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCacheListCLIReadOnlyAndJSON(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	root := filepath.Join(cache, "mdhop", "vaults")
	folders := []string{filepath.Join(root, strings.Repeat("a", 64)), filepath.Join(root, strings.Repeat("b", 64))}
	for _, dir := range folders {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.sqlite"), []byte("not a DB"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(folders[0], "origin.json"), []byte(`{"vault":"/removed/vault"}`), 0600); err != nil {
		t.Fatal(err)
	}
	type state struct {
		mode     os.FileMode
		mtime    int64
		contents string
	}
	snapshot := func() map[string]state {
		got := map[string]state{}
		err := filepath.WalkDir(cache, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			s := state{mode: info.Mode(), mtime: info.ModTime().UnixNano()}
			if !d.IsDir() {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				s.contents = string(data)
			}
			got[path] = s
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	before := snapshot()
	expected, err := core.ListCaches()
	if err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() error { return runCacheList([]string{"--format", "json"}) })
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("JSON=%s", out)
	}
	for i, entry := range entries {
		if len(entry) != 3 {
			t.Fatalf("keys=%v", entry)
		}
		var path string
		var size int64
		if err := json.Unmarshal(entry["cache_path"], &path); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(entry["size_bytes"], &size); err != nil {
			t.Fatal(err)
		}
		if path != folders[i] || size != expected[i].SizeBytes {
			t.Fatalf("entry=%v", entry)
		}
	}
	if string(entries[1]["vault"]) != "null" || string(entries[0]["vault"]) != `"/removed/vault"` {
		t.Fatalf("origins=%v", entries)
	}
	text := captureStdout(t, func() error { return runCacheList(nil) })
	if !strings.Contains(text, folders[0]+"\nsize: ") || !strings.Contains(text, "  original: /removed/vault\n\n"+folders[1]) || !strings.HasSuffix(text, "  original: unknown\n") {
		t.Fatalf("text=%q", text)
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("listing changed filesystem: before=%v after=%v", before, after)
	}
}

func TestCacheListCLIEmptyAndRejectedArguments(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	t.Setenv("XDG_CACHE_HOME", missing)
	if out := captureStdout(t, func() error { return runCacheList(nil) }); out != "" {
		t.Fatalf("empty text=%q", out)
	}
	if out := captureStdout(t, func() error { return runCacheList([]string{"--format", "json"}) }); strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty JSON=%q", out)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("created cache root")
	}
	for _, args := range [][]string{{"--format", "yaml"}, {"positional"}, {"--vault", "."}, {"--db", "index.sqlite"}, {"--config", "mdhop.toml"}} {
		if err := runCacheList(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	help := captureStderr(t, func() error {
		err := runCacheList([]string{"--help"})
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("help=%v", err)
		}
		return nil
	})
	for _, contract := range []string{"ascending cache_path", "512 bytes", "hardlinks", "unknown", "origin.json"} {
		if !strings.Contains(help, contract) {
			t.Errorf("help missing %q", contract)
		}
	}
	if err := runBuild([]string{"--db", ""}); err == nil || !strings.Contains(err.Error(), "nonempty") {
		t.Fatalf("empty explicit DB=%v", err)
	}
}
