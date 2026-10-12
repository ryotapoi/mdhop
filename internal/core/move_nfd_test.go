package core

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// observeMoveNFD records the actual directory entries and the normalization
// behavior of the filesystem hosting this test, without skipping the contract.
func observeMoveNFD(t *testing.T, vault, actual string) {
	t.Helper()
	full := filepath.Join(vault, actual)
	entries, err := os.ReadDir(filepath.Dir(full))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Logf("disk entry: name=%q bytes=% x", entry.Name(), []byte(entry.Name()))
	}
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("NFD access %q: %v", full, err)
	}
	nfc := filepath.Join(vault, NormalizePath(actual))
	_, err = os.Stat(nfc)
	t.Logf("filesystem condition: os=%s temp=%q NFD=%q bytes=% x NFC=%q bytes=% x NFC-direct-stat=%v", runtime.GOOS, vault, actual, []byte(actual), NormalizePath(actual), []byte(NormalizePath(actual)), err)
	if runtime.GOOS == "linux" && !os.IsNotExist(err) {
		t.Fatalf("Linux regression requires distinct NFD/NFC access, got %v", err)
	}
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func assertMoveNFDIndex(t *testing.T, vault, from, to string) {
	t.Helper()
	nodes := queryNodes(t, dbPath(vault), "note")
	found := false
	for _, node := range nodes {
		if node.path != NormalizePath(node.path) {
			t.Fatalf("non-NFC DB path: %q", node.path)
		}
		if node.path == from {
			t.Fatalf("old DB path remains: %q", from)
		}
		if node.path == to {
			found = true
			if node.nodeKey != noteKey(to) {
				t.Fatalf("node key = %q", node.nodeKey)
			}
		}
	}
	if !found {
		t.Fatalf("destination %q missing from nodes: %+v", to, nodes)
	}
}

func TestMoveNFDDiskPaths(t *testing.T) {
	for _, mode := range []string{"normal", "already-moved", "template"} {
		t.Run(mode, func(t *testing.T) {
			const actual = "src/Cafe\u0301.md"
			const from = "src/Café.md"
			const to = "Résumé/deep/Café.md"
			vault := newMoveVault(t, map[string]string{actual: "[target](../Target.md)\n", "Index.md": "[[src/Café]]\n", "Target.md": "# Target\n"})
			observeMoveNFD(t, vault, actual)
			actualParent := "Re\u0301sume\u0301"
			if err := os.Mkdir(filepath.Join(vault, actualParent), 0o755); err != nil {
				t.Fatal(err)
			}
			observeMoveNFD(t, vault, actualParent)
			actualTo := actualParent + "/deep/Café.md"
			if mode == "already-moved" {
				actualTo = actualParent + "/deep/Cafe\u0301.md"
				if err := os.Mkdir(filepath.Join(vault, actualParent, "deep"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(vault, actual), filepath.Join(vault, actualTo)); err != nil {
					t.Fatal(err)
				}
				observeMoveNFD(t, vault, actualTo)
			}
			var err error
			if mode == "template" {
				result, moveErr := MoveTemplate(vault, MoveTemplateOptions{From: from, Template: "Résumé/deep/{basename}"})
				err = moveErr
				if err == nil && !reflect.DeepEqual(result.Moved, []MovedFile{{From: from, To: to}}) {
					t.Fatalf("template moves = %+v", result.Moved)
				}
			} else {
				_, err = Move(vault, MoveOptions{From: from, To: to})
			}
			if err != nil {
				t.Fatalf("NFC logical move of NFD file: %v", err)
			}
			if _, err := os.Stat(filepath.Join(vault, actual)); !os.IsNotExist(err) {
				t.Fatalf("source remains: %v", err)
			}
			if got := readVaultFile(t, vault, actualTo); got != "[target](../../Target.md)\n" {
				t.Fatalf("moved content = %q", got)
			}
			if got := readVaultFile(t, vault, "Index.md"); got != "[[Résumé/deep/Café]]\n" {
				t.Fatalf("incoming content = %q", got)
			}
			// Resolving an existing NFD parent must not create a second NFC directory.
			entries, err := os.ReadDir(vault)
			if err != nil {
				t.Fatal(err)
			}
			parents := 0
			for _, entry := range entries {
				if NormalizePath(entry.Name()) == "Résumé" {
					parents++
				}
			}
			if parents != 1 {
				t.Fatalf("destination parents = %d, want 1", parents)
			}
			assertMoveNFDIndex(t, vault, from, to)
			incoming := queryEdges(t, dbPath(vault), "Index.md")
			outgoing := queryEdges(t, dbPath(vault), to)
			if len(incoming) != 1 || incoming[0].targetKey != noteKey(to) || incoming[0].rawLink != "[[Résumé/deep/Café]]" {
				t.Fatalf("incoming edges = %+v", incoming)
			}
			if len(outgoing) != 1 || outgoing[0].targetKey != noteKey("Target.md") {
				t.Fatalf("outgoing edges = %+v", outgoing)
			}
		})
	}
}

func TestMoveNFDStale(t *testing.T) {
	for _, alreadyMoved := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "already-moved"}[alreadyMoved], func(t *testing.T) {
			const actual = "Cafe\u0301.md"
			const destination = "Re\u0301sume\u0301.md"
			vault := newMoveVault(t, map[string]string{actual: "# Original\n"})
			observeMoveNFD(t, vault, actual)
			disk := actual
			want := ErrSourceStale
			if alreadyMoved {
				if err := os.Rename(filepath.Join(vault, actual), filepath.Join(vault, destination)); err != nil {
					t.Fatal(err)
				}
				disk = destination
				want = ErrMovedFileStale
				observeMoveNFD(t, vault, disk)
			}
			before := queryNodes(t, dbPath(vault), "note")
			writeStaleTestFile(t, filepath.Join(vault, disk), []byte("# Changed\n"))
			if _, err := Move(vault, MoveOptions{From: "Café.md", To: "Résumé.md"}); !errors.Is(err, want) {
				t.Fatalf("stale error = %v, want %v", err, want)
			}
			if got := readVaultFile(t, vault, disk); got != "# Changed\n" {
				t.Fatalf("stale content changed: %q", got)
			}
			if !reflect.DeepEqual(queryNodes(t, dbPath(vault), "note"), before) {
				t.Fatal("stale rejection changed index")
			}
		})
	}
}

func TestMoveNFDDestinationRefused(t *testing.T) {
	vault := newMoveVault(t, map[string]string{"Source.md": "# Source\n"})
	// Create after build so this exercises disk protection, rather than DB protection.
	const destination = "Re\u0301sume\u0301/Cafe\u0301.md"
	writeVaultFile(t, vault, destination, "# Keep\n")
	observeMoveNFD(t, vault, destination)
	before := queryNodes(t, dbPath(vault), "note")
	if _, err := Move(vault, MoveOptions{From: "Source.md", To: "Résumé/Café.md"}); err == nil {
		t.Fatal("existing NFD destination was accepted")
	}
	if got := readVaultFile(t, vault, destination); got != "# Keep\n" {
		t.Fatalf("destination overwritten: %q", got)
	}
	if got := readVaultFile(t, vault, "Source.md"); got != "# Source\n" {
		t.Fatalf("source changed: %q", got)
	}
	if !reflect.DeepEqual(queryNodes(t, dbPath(vault), "note"), before) {
		t.Fatal("destination refusal changed index")
	}
}

func TestMoveNFDDirectory(t *testing.T) {
	const dir = "Cafe\u0301"
	vault := newMoveVault(t, map[string]string{dir + "/Re\u0301sume\u0301.md": "![image](Piqué.png)\n", dir + "/Pique\u0301.png": "image", "Index.md": "[[Café/Résumé]]\n"})
	const late = "Prive\u0301.bin"
	writeVaultFile(t, vault, dir+"/"+late, "disk-only")
	observeMoveNFD(t, vault, dir+"/Re\u0301sume\u0301.md")
	observeMoveNFD(t, vault, dir+"/"+late)
	result, err := MoveDir(vault, MoveDirOptions{FromDir: "Café", ToDir: "archive"})
	if err != nil {
		t.Fatalf("NFC directory move: %v", err)
	}
	want := []MovedFile{{From: "Café/Piqué.png", To: "archive/Piqué.png"}, {From: "Café/Résumé.md", To: "archive/Résumé.md"}, {From: "Café/Privé.bin", To: "archive/Privé.bin"}}
	slices.SortFunc(result.Moved, func(a, b MovedFile) int { return strings.Compare(a.From, b.From) })
	slices.SortFunc(want, func(a, b MovedFile) int { return strings.Compare(a.From, b.From) })
	if !reflect.DeepEqual(result.Moved, want) {
		t.Fatalf("moves = %+v, want %+v", result.Moved, want)
	}
	for rel, body := range map[string]string{"archive/Résumé.md": "![image](./Piqué.png)\n", "archive/Piqué.png": "image", "archive/Privé.bin": "disk-only", "Index.md": "[[archive/Résumé]]\n"} {
		if got := readVaultFile(t, vault, rel); got != body {
			t.Fatalf("%s = %q, want %q", rel, got, body)
		}
	}
	for _, actual := range []string{dir + "/Re\u0301sume\u0301.md", dir + "/Pique\u0301.png", dir + "/" + late} {
		if _, err := os.Stat(filepath.Join(vault, actual)); !os.IsNotExist(err) {
			t.Fatalf("old file remains %q: %v", actual, err)
		}
	}
	assertMoveNFDIndex(t, vault, "Café/Résumé.md", "archive/Résumé.md")
	edges := queryEdges(t, dbPath(vault), "archive/Résumé.md")
	if len(edges) != 1 || edges[0].targetKey != assetKey("archive/Piqué.png") {
		t.Fatalf("asset edges = %+v", edges)
	}
}

func TestMoveNFDRollback(t *testing.T) {
	for _, failure := range []string{"database", "already-moved-database", "partial-rename"} {
		t.Run(failure, func(t *testing.T) {
			const actual = "Cafe\u0301/Re\u0301sume\u0301.md"
			originals := map[string]string{actual: "[target](../Target.md)\n", "Index.md": "[[Café/Résumé]]\n", "Target.md": "# Target\n"}
			if failure == "partial-rename" {
				originals["Cafe\u0301/Z.bin"] = "late file"
			}
			vault := t.TempDir()
			mtimes := map[string]time.Time{}
			for rel, body := range originals {
				writeVaultFile(t, vault, rel, body)
				full := filepath.Join(vault, rel)
				if err := os.Chmod(full, 0o600); err != nil {
					t.Fatal(err)
				}
				mtimes[rel] = setRollbackTestMtime(t, full)
			}
			if _, err := Build(vault); err != nil {
				t.Fatal(err)
			}
			observeMoveNFD(t, vault, actual)
			before := queryNodes(t, dbPath(vault), "note")
			beforeIncoming := queryEdges(t, dbPath(vault), "Index.md")
			beforeOutgoing := queryEdges(t, dbPath(vault), "Café/Résumé.md")
			diskActual := actual
			if failure == "already-moved-database" {
				diskActual = "deep/sub/Re\u0301sume\u0301.md"
				if err := os.MkdirAll(filepath.Join(vault, "deep/sub"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(vault, actual), filepath.Join(vault, diskActual)); err != nil {
					t.Fatal(err)
				}
				observeMoveNFD(t, vault, diskActual)
			}
			primary := errors.New("injected NFD rollback failure")
			oldExec, oldRename := rewriteTxExec, moveRename
			t.Cleanup(func() { rewriteTxExec = oldExec; moveRename = oldRename })
			successfulRename := false
			if failure == "partial-rename" {
				moveRename = func(from, to string) error {
					if strings.HasPrefix(to, filepath.Join(vault, "deep")+string(os.PathSeparator)) {
						if successfulRename {
							return primary
						}
						err := oldRename(from, to)
						if err == nil {
							successfulRename = true
						}
						return err
					}
					return oldRename(from, to)
				}
			} else {
				rewriteTxExec = func(dbExecer, string, ...any) (sql.Result, error) { return nil, primary }
			}
			var err error
			if failure == "partial-rename" {
				_, err = MoveDir(vault, MoveDirOptions{FromDir: "Café", ToDir: "deep/sub"})
			} else {
				_, err = Move(vault, MoveOptions{From: "Café/Résumé.md", To: "deep/sub/Résumé.md"})
			}
			if !errors.Is(err, primary) {
				t.Fatalf("error = %v, want injected failure", err)
			}
			if failure == "partial-rename" && !successfulRename {
				t.Fatal("failure did not follow a completed rename")
			}
			for rel, body := range originals {
				diskRel := rel
				if rel == actual {
					diskRel = diskActual
				}
				if got := readVaultFile(t, vault, diskRel); got != body {
					t.Fatalf("restored %q = %q, want %q", diskRel, got, body)
				}
				info, err := os.Stat(filepath.Join(vault, diskRel))
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0o600 || !info.ModTime().Equal(mtimes[rel]) {
					t.Fatalf("attributes not restored for %q: mode=%v mtime=%v want=%v", diskRel, info.Mode(), info.ModTime(), mtimes[rel])
				}
			}
			if !reflect.DeepEqual(queryNodes(t, dbPath(vault), "note"), before) || !reflect.DeepEqual(queryEdges(t, dbPath(vault), "Index.md"), beforeIncoming) || !reflect.DeepEqual(queryEdges(t, dbPath(vault), "Café/Résumé.md"), beforeOutgoing) {
				t.Fatal("rollback changed index")
			}
			observeMoveNFD(t, vault, diskActual)
			entries, err := os.ReadDir(filepath.Dir(filepath.Join(vault, diskActual)))
			if err != nil {
				t.Fatal(err)
			}
			foundOriginalName := false
			for _, entry := range entries {
				if entry.Name() == filepath.Base(diskActual) {
					foundOriginalName = true
				}
			}
			if !foundOriginalName {
				t.Fatalf("rollback lost original disk spelling %q", diskActual)
			}
			if failure != "already-moved-database" {
				if _, err := os.Stat(filepath.Join(vault, "deep/sub/Résumé.md")); !os.IsNotExist(err) {
					t.Fatalf("destination survived rollback: %v", err)
				}
			}
		})
	}
}
