package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNFDDiskPathsContentAndChecks(t *testing.T) {
	vault := t.TempDir()
	dir := "Cafe\u0301"
	actual := filepath.Join(vault, dir, "Re\u0301sume\u0301.md")
	if err := os.MkdirAll(filepath.Dir(actual), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nfolder: /Café/\n---\n# Present\n[[Target]]\n"
	if err := os.WriteFile(actual, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Target.md"), []byte("[[Café/Résumé#Present]]\n[[Café/Résumé#Absent]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	result, err := Query(vault, EntrySpec{File: "Café/Résumé.md"}, QueryOptions{Fields: []string{"head"}, IncludeHead: 2})
	if err != nil || !reflect.DeepEqual(result.Head, []string{"# Present", "[[Target]]"}) {
		t.Fatalf("head = %+v, %v", result, err)
	}
	result, err = Query(vault, EntrySpec{File: "Target.md"}, QueryOptions{Fields: []string{"snippet"}, IncludeSnippet: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Snippets) != 1 || result.Snippets[0].SourcePath != "Café/Résumé.md" || !reflect.DeepEqual(result.Snippets[0].Lines, []string{"# Present", "[[Target]]"}) {
		t.Fatalf("snippets = %+v", result.Snippets)
	}
	diag, err := Diagnose(vault, DiagnoseOptions{Fields: []string{"anchors"}})
	if err != nil || len(diag.BrokenAnchors) != 1 || diag.BrokenAnchors[0].Fragment != "Absent" {
		t.Fatalf("anchors = %+v, %v", diag, err)
	}
	check, err := MetaCheck(vault, MetaCheckOptions{Keys: []string{"folder"}, Kind: MetaKindPath})
	if err != nil || len(check.Issues) != 0 {
		t.Fatalf("meta check = %+v, %v", check, err)
	}
	writeStaleTestFile(t, actual, []byte(content+"changed\n"))
	_, err = Query(vault, EntrySpec{File: "Café/Résumé.md"}, QueryOptions{Fields: []string{"head"}, IncludeHead: 1})
	if !errors.Is(err, ErrSourceStale) {
		t.Fatalf("head stale error = %v", err)
	}
	_, err = Query(vault, EntrySpec{File: "Target.md"}, QueryOptions{Fields: []string{"snippet"}, IncludeSnippet: 1})
	if !errors.Is(err, ErrSourceStale) {
		t.Fatalf("snippet stale error = %v", err)
	}
	if err := os.Remove(actual); err != nil {
		t.Fatal(err)
	}
	_, err = Query(vault, EntrySpec{File: "Café/Résumé.md"}, QueryOptions{Fields: []string{"head"}, IncludeHead: 1})
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("missing head error = %v", err)
	}
	diag, err = Diagnose(vault, DiagnoseOptions{Fields: []string{"anchors"}})
	if err != nil || len(diag.BrokenAnchors) != 2 {
		t.Fatalf("missing anchors = %+v, %v", diag, err)
	}
}

func TestDeleteNFDDiskPaths(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			vault := t.TempDir()
			dir := "Cafe\u0301"
			actualDir := filepath.Join(vault, dir)
			if err := os.Mkdir(actualDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"Re\u0301sume\u0301.md", "Unused.md"} {
				if err := os.WriteFile(filepath.Join(actualDir, name), []byte("# Note\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(vault, "Index.md"), []byte("[[Café/Résumé]]\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Build(vault); err != nil {
				t.Fatal(err)
			}
			input := "Café/Résumé.md"
			if directory {
				input = "Café"
			}
			beforeNotes, beforeEdges := countNotes(t, dbPath(vault)), countEdges(t, dbPath(vault))
			if _, err := Delete(vault, DeleteOptions{Files: []string{input}}); err == nil || !strings.Contains(err.Error(), "file still exists on disk") {
				t.Fatalf("delete without rm error = %v", err)
			}
			if countNotes(t, dbPath(vault)) != beforeNotes || countEdges(t, dbPath(vault)) != beforeEdges {
				t.Fatal("rejected delete changed DB")
			}
			if directory {
				if err := os.WriteFile(filepath.Join(actualDir, "late.bin"), []byte("asset"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			result, err := Delete(vault, DeleteOptions{Files: []string{input}, RemoveFiles: true})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Phantomed, []string{"Café/Résumé.md"}) {
				t.Fatalf("phantomed = %v", result.Phantomed)
			}
			if _, err := os.Lstat(filepath.Join(actualDir, "Re\u0301sume\u0301.md")); !os.IsNotExist(err) {
				t.Fatalf("NFD file remains: %v", err)
			}
			if directory {
				if !reflect.DeepEqual(result.Deleted, []string{"Café/Unused.md"}) {
					t.Fatalf("deleted = %v", result.Deleted)
				}
				if _, err := os.Lstat(actualDir); !os.IsNotExist(err) {
					t.Fatalf("NFD directory remains: %v", err)
				}
			}
		})
	}
}

func TestDeleteNFDTerminalSymlink(t *testing.T) {
	for _, dangling := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing", true: "dangling"}[dangling], func(t *testing.T) {
			vault := t.TempDir()
			actual := filepath.Join(vault, "Cafe\u0301.md")
			if err := os.WriteFile(actual, []byte("note\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Build(vault); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(actual); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(t.TempDir(), "outside.md")
			if !dangling {
				if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(outside, actual); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if _, err := Delete(vault, DeleteOptions{Files: []string{"Café.md"}, RemoveFiles: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(actual); !os.IsNotExist(err) {
				t.Fatalf("symlink remains: %v", err)
			}
			if !dangling {
				if got, err := os.ReadFile(outside); err != nil || string(got) != "outside\n" {
					t.Fatalf("outside changed: %q, %v", got, err)
				}
			}
		})
	}
}

func TestDeleteNFDAncestorSymlinkEscapesVault(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "Cafe\u0301")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "A.md"), []byte("inside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "A.md")
	if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, dir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := Delete(vault, DeleteOptions{Files: []string{"Café/"}, RemoveFiles: true})
	if err == nil || !strings.Contains(err.Error(), "path escapes vault") {
		t.Fatalf("delete error = %v", err)
	}
	if got, err := os.ReadFile(outsideFile); err != nil || string(got) != "outside\n" {
		t.Fatalf("outside changed: %q, %v", got, err)
	}
	if countNotes(t, dbPath(vault)) != 1 {
		t.Fatal("rejected delete changed DB")
	}
}
