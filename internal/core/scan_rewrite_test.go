package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanExcludesSelectedMarkdownIndex(t *testing.T) {
	for _, operation := range []string{"convert", "repair", "simplify", "disambiguate"} {
		t.Run(operation, func(t *testing.T) {
			vault := t.TempDir()
			location := Locations{DBPath: filepath.Join(vault, "generated", "index.md")}
			if err := os.MkdirAll(filepath.Join(vault, "notes"), 0700); err != nil {
				t.Fatal(err)
			}
			source, want := "[[index]]\n", "[[notes/index]]\n"
			switch operation {
			case "convert":
				source, want = "[index](notes/index.md)\n", "[[notes/index]]\n"
			case "repair":
				source, want = "[[missing/index]]\n", "[[index]]\n"
			case "simplify":
				source, want = "[[notes/index]]\n", "[[index]]\n"
			}
			for name, content := range map[string]string{"A.md": source, "notes/index.md": "# Index\n"} {
				if err := os.WriteFile(filepath.Join(vault, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Build(vault, location); err != nil {
				t.Fatal(err)
			}
			// Store links on separate lines so treating SQLite bytes as Markdown would
			// produce rewrites, including a conversion that would corrupt the index.
			payload := "\n[index](notes/index.md)\n[[missing/index]]\n[[notes/index]]\n[[index]]\n"
			db, err := openDBAt(location.DBPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("INSERT INTO meta(node_id, key, value, line) SELECT id, 'scan_payload', ?, 1 FROM nodes WHERE path = 'A.md'", payload); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(location.DBPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(before, []byte(payload)) {
				t.Fatal("fixture does not contain rewriteable links")
			}
			var rewritten []RewrittenLink
			switch operation {
			case "convert":
				result, err := Convert(vault, ConvertOptions{ToFormat: "wikilink"}, location)
				if err != nil {
					t.Fatal(err)
				}
				rewritten = result.Rewritten
				_, err = Convert(vault, ConvertOptions{ToFormat: "wikilink", Files: []string{"generated/index.md"}}, location)
				if err == nil || !strings.Contains(err.Error(), "file not found or excluded") {
					t.Fatalf("index file error: %v", err)
				}
			case "repair":
				result, err := Repair(vault, RepairOptions{}, location)
				if err != nil {
					t.Fatal(err)
				}
				rewritten = result.Rewritten
			case "simplify":
				result, err := Simplify(vault, SimplifyOptions{}, location)
				if err != nil {
					t.Fatal(err)
				}
				rewritten = result.Rewritten
				if len(result.Skipped) != 0 {
					t.Fatalf("unexpected ambiguity: %+v", result.Skipped)
				}
				_, err = Simplify(vault, SimplifyOptions{Files: []string{"generated/index.md"}}, location)
				if !errors.Is(err, ErrFileNotFound) {
					t.Fatalf("index file error: %v", err)
				}
			case "disambiguate":
				result, err := DisambiguateScan(vault, DisambiguateOptions{Name: "index"}, location)
				if err != nil {
					t.Fatal(err)
				}
				rewritten = result.Rewritten
				_, err = DisambiguateScan(vault, DisambiguateOptions{Name: "index", Files: []string{"generated/index.md"}}, location)
				if !errors.Is(err, ErrFileNotFound) {
					t.Fatalf("index file error: %v", err)
				}
			}
			if len(rewritten) != 1 || rewritten[0].File != "A.md" {
				t.Fatalf("rewritten=%+v", rewritten)
			}
			content, err := os.ReadFile(filepath.Join(vault, "A.md"))
			if err != nil || string(content) != want {
				t.Fatalf("source=%q err=%v, want %q", content, err, want)
			}
			after, err := os.ReadFile(location.DBPath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("scan changed index bytes: %v", err)
			}
			db, err = openDBAt(location.DBPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var stored, integrity string
			if err := db.QueryRow("SELECT value FROM meta WHERE key = 'scan_payload'").Scan(&stored); err != nil {
				t.Fatal(err)
			}
			if stored != payload {
				t.Fatalf("index payload=%q", stored)
			}
			if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
				t.Fatalf("integrity=%q err=%v", integrity, err)
			}
		})
	}
}
