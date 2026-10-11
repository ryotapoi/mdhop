package core

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func buildTestWrite(t *testing.T, vault, path, content string) {
	t.Helper()
	full := filepath.Join(vault, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Compare the observable index, retaining every edge/meta attribute and node
// timestamps while excluding generated numeric identities.
func buildSemantics(t *testing.T, path string) []string {
	t.Helper()
	db := openTestDB(t, path)
	defer db.Close()
	queries := []string{
		`SELECT node_key,type,name,path,exists_flag,mtime,lines FROM nodes ORDER BY node_key`,
		`SELECT s.node_key,t.node_key,e.link_type,e.raw_link,e.frontmatter_key,e.reference_target,e.in_table,e.subpath,e.line_start,e.line_end FROM edges e JOIN nodes s ON s.id=e.source_id JOIN nodes t ON t.id=e.target_id ORDER BY s.node_key,t.node_key,e.link_type,e.raw_link,e.frontmatter_key,e.reference_target,e.in_table,e.subpath,e.line_start,e.line_end`,
		`SELECT n.node_key,m.key,m.value,m.line,m.sort_value,m.value_type FROM meta m JOIN nodes n ON n.id=m.node_id ORDER BY n.node_key,m.key,m.value,m.line,m.sort_value,m.value_type`,
	}
	var result []string
	for _, query := range queries {
		rows, err := db.Query(query)
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			dest := make([]any, len(columns))
			for i := range values {
				dest[i] = &values[i]
			}
			if err := rows.Scan(dest...); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			result = append(result, string(encoded))
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	return result
}

func assertBuildMatchesFull(t *testing.T, vault string) {
	t.Helper()
	auto, err := Build(vault)
	if err != nil {
		t.Fatal(err)
	}
	got := buildSemantics(t, dbPath(vault))
	fullPath := filepath.Join(t.TempDir(), "full.sqlite")
	full, err := BuildFull(vault, Locations{DBPath: fullPath})
	if err != nil {
		t.Fatal(err)
	}
	want := buildSemantics(t, fullPath)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("auto/full mismatch\nauto: %v\nfull: %v", got, want)
	}
	if !reflect.DeepEqual(auto.Warnings, full.Warnings) {
		t.Fatalf("warnings = %v, want %v", auto.Warnings, full.Warnings)
	}
	assertNoBuildTemps(t, vault)
}

func TestBuildIncrementalSemanticMatrix(t *testing.T) {
	vault := t.TempDir()
	buildTestWrite(t, vault, "mdhop.toml", "[meta]\nlink_keys = ['related']\n[meta.types]\nrank = 'number'\n")
	buildTestWrite(t, vault, "src/A.md", "---\nrelated: Target\nrank: bad\n---\n[[Target]] [[/future/Next]] [[#Self]]\n[path](../future/Next.md) [ref][next] ![image](../picture.png) #old\n[next]: ../future/Next.md#Heading\n")
	buildTestWrite(t, vault, "one/Target.md", "# Target\n")
	assertBuildMatchesFull(t, vault)
	assertBuildMatchesFull(t, vault)
	buildTestWrite(t, vault, "future/Next.md", "# Next\n")
	buildTestWrite(t, vault, "picture.png", "asset")
	assertBuildMatchesFull(t, vault)
	buildTestWrite(t, vault, "src/A.md", "---\nrelated: Target\nrank: 12\n---\n[[Target]] [ref][next] #new\n[next]: ../future/Next.md#Heading\n")
	assertBuildMatchesFull(t, vault)
	buildTestWrite(t, vault, "Target.md", "root priority\n")
	assertBuildMatchesFull(t, vault)
	buildTestWrite(t, vault, "two/Target.md", "second\n")
	assertBuildMatchesFull(t, vault)
	if err := os.Remove(filepath.Join(vault, "Target.md")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguity error = %v", err)
	}
	after, err := os.ReadFile(dbPath(vault))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed auto build changed DB", err)
	}
	if _, err := BuildFull(vault); err == nil {
		t.Fatal("full accepted ambiguity")
	}
	assertNoBuildTemps(t, vault)
	if err := os.Remove(filepath.Join(vault, "two/Target.md")); err != nil {
		t.Fatal(err)
	}
	assertBuildMatchesFull(t, vault)
	if err := os.Rename(filepath.Join(vault, "one/Target.md"), filepath.Join(vault, "one/Moved.md")); err != nil {
		t.Fatal(err)
	}
	assertBuildMatchesFull(t, vault)
	for _, path := range []string{"one/Moved.md", "future/Next.md", "picture.png"} {
		if err := os.Remove(filepath.Join(vault, path)); err != nil {
			t.Fatal(err)
		}
	}
	assertBuildMatchesFull(t, vault)
}

func TestBuildIncrementalSignatureAndFullRecovery(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "A.md")
	buildTestWrite(t, vault, "A.md", "#old\n")
	stamp := time.Unix(1700000000, 100)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	assertBuildMatchesFull(t, vault)
	buildTestWrite(t, vault, "A.md", "#new\n")
	next := stamp.Add(time.Nanosecond)
	if err := os.Chtimes(path, next, next); err != nil {
		t.Fatal(err)
	}
	assertBuildMatchesFull(t, vault)
	buildTestWrite(t, vault, "A.md", "#longer\n")
	if err := os.Chtimes(path, next, next); err != nil {
		t.Fatal(err)
	}
	assertBuildMatchesFull(t, vault)
	// This documented boundary intentionally keeps the old index until --full.
	before := buildSemantics(t, dbPath(vault))
	buildTestWrite(t, vault, "A.md", "#change\n")
	if err := os.Chtimes(path, next, next); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	if got := buildSemantics(t, dbPath(vault)); !reflect.DeepEqual(got, before) {
		t.Fatal("same signature unexpectedly changed index")
	}
	if _, err := BuildFull(vault); err != nil {
		t.Fatal(err)
	}
	if got := buildSemantics(t, dbPath(vault)); reflect.DeepEqual(got, before) {
		t.Fatal("full did not recover same-signature edit")
	}
}

func TestBuildIncrementalFallbacks(t *testing.T) {
	for _, invalidate := range []string{"state", "file", "links", "version", "update", "delete", "config", "root"} {
		t.Run(invalidate, func(t *testing.T) {
			vault := t.TempDir()
			buildTestWrite(t, vault, "A.md", "[[B]] #old\n")
			buildTestWrite(t, vault, "B.md", "# B\n")
			assertBuildMatchesFull(t, vault)
			switch invalidate {
			case "update":
				if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
					t.Fatal(err)
				}
			case "delete":
				if err := os.Remove(filepath.Join(vault, "A.md")); err != nil {
					t.Fatal(err)
				}
				if _, err := Delete(vault, DeleteOptions{Files: []string{"A.md"}}); err != nil {
					t.Fatal(err)
				}
				buildTestWrite(t, vault, "A.md", "[[B]] #old\n")
			case "config":
				buildTestWrite(t, vault, "mdhop.toml", "[build]\nexclude_paths = ['B.md']\n")
			case "root":
				other := t.TempDir()
				buildTestWrite(t, other, "A.md", "[[B]] #root\n")
				buildTestWrite(t, other, "B.md", "# B\n")
				if _, err := Build(other, Locations{DBPath: dbPath(vault)}); err != nil {
					t.Fatal(err)
				}
				assertBuildMatchesFull(t, vault)
				return
			default:
				db := openTestDB(t, dbPath(vault))
				statements := map[string]string{"state": "DELETE FROM build_state", "file": "DELETE FROM build_files WHERE path='A.md'", "links": "DELETE FROM build_links WHERE source_path='A.md'", "version": "PRAGMA user_version=1"}
				if _, err := db.Exec(statements[invalidate]); err != nil {
					t.Fatal(err)
				}
				db.Close()
			}
			// Keep the same signature, making the result proof of a full fallback.
			info, err := os.Stat(filepath.Join(vault, "A.md"))
			if err != nil {
				t.Fatal(err)
			}
			buildTestWrite(t, vault, "A.md", "[[B]] #new\n")
			if err := os.Chtimes(filepath.Join(vault, "A.md"), info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
			assertBuildMatchesFull(t, vault)
		})
	}
}

func TestBuildInputChangesPreserveIndex(t *testing.T) {
	vault := t.TempDir()
	buildTestWrite(t, vault, "A.md", "#old\n")
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareBuild(vault)
	if err != nil {
		t.Fatal(err)
	}
	buildTestWrite(t, vault, "A.md", "#changed\n")
	if _, err := buildPrepared(vault, prepared); err == nil || !strings.Contains(err.Error(), "changed during build") {
		t.Fatalf("error = %v", err)
	}
	after, err := os.ReadFile(dbPath(vault))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("index changed", err)
	}
	if _, err := readBuildNote(prepared.inputs, "A.md"); err == nil {
		t.Fatal("read accepted changed signature")
	}
	assertNoBuildTemps(t, vault)
}

func TestBuildIncrementalReaderGenerations(t *testing.T) {
	vault := t.TempDir()
	buildTestWrite(t, vault, "A.md", "#old\n")
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	old := openTestDB(t, dbPath(vault))
	defer old.Close()
	var name string
	if err := old.QueryRow("SELECT name FROM nodes WHERE type='tag'").Scan(&name); err != nil {
		t.Fatal(err)
	}
	buildTestWrite(t, vault, "A.md", "#new\nextra\n")
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	if err := old.QueryRow("SELECT name FROM nodes WHERE type='tag'").Scan(&name); err != nil || name != "#old" {
		t.Fatalf("old generation = %q, %v", name, err)
	}
	current := openTestDB(t, dbPath(vault))
	defer current.Close()
	if err := current.QueryRow("SELECT name FROM nodes WHERE type='tag'").Scan(&name); err != nil || name != "#new" {
		t.Fatalf("new generation = %q, %v", name, err)
	}
}

func TestBuildFingerprintAndDependencies(t *testing.T) {
	if got := (buildSignature{Mtime: -1}).seconds(); got != -1 {
		t.Fatalf("pre-epoch seconds = %d", got)
	}
	cfg := &Config{Build: BuildConfig{ExcludePaths: []string{"b", "a"}}, Meta: MetaConfig{LinkKeys: []string{"y", "x"}}}
	first, err := buildFingerprint("/vault", cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Build.ExcludePaths = []string{"a", "b"}
	cfg.Meta.LinkKeys = []string{"x", "y"}
	cfg.Exclude.Paths = []string{"query-only"}
	second, err := buildFingerprint("/vault", cfg)
	if err != nil || first != second {
		t.Fatal("effective fingerprint is unstable", err)
	}
	cfg.Meta.Types = map[string]MetaTypeInfo{"rank": {Name: MetaTypeNumber}}
	third, _ := buildFingerprint("/vault", cfg)
	if first == third {
		t.Fatal("meta type ignored")
	}
	cases := []struct {
		link linkOccur
		want string
	}{
		{linkOccur{target: "#tag", linkType: LinkTypeTag, isBasename: true}, ""},
		{linkOccur{subpath: "#Self", linkType: LinkTypeWikilink, isBasename: true}, ""},
		{linkOccur{target: "Target", linkType: LinkTypeFrontmatterPath, isBasename: true}, "basename:target"},
		{linkOccur{target: "../Target.md", linkType: LinkTypeMarkdownReference, isRelative: true}, "path:target.md"},
	}
	for _, tc := range cases {
		got, err := buildDependency("sub/A.md", tc.link)
		if err != nil || got != tc.want {
			t.Fatalf("dependency = %q, %v; want %q", got, err, tc.want)
		}
	}
	old := newResolveMaps([]string{"one/T.md"}, nil)
	for _, next := range []*resolveMaps{newResolveMaps([]string{"one/T.md", "T.md"}, nil), newResolveMaps([]string{"one/T.md", "two/T.md"}, nil), newResolveMaps(nil, nil)} {
		if dependencyResolution("basename:t", old) == dependencyResolution("basename:t", next) {
			t.Fatal("changed basename resolution missed")
		}
	}
}

func TestBuildReusesUnreadableUnchangedNote(t *testing.T) {
	vault := t.TempDir()
	buildTestWrite(t, vault, "A.md", "[[B]]\n")
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "A.md")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0o600)
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("filesystem/user does not enforce file read permissions")
	}
	if _, err := Build(vault); err != nil {
		t.Fatal("unchanged build read note", err)
	}
	buildTestWrite(t, vault, "B.md", "# B\n")
	if _, err := Build(vault); err != nil {
		t.Fatal("dependency reevaluation read unchanged note", err)
	}
	db := openTestDB(t, dbPath(vault))
	var target string
	err := db.QueryRow("SELECT t.node_key FROM edges e JOIN nodes t ON t.id=e.target_id").Scan(&target)
	db.Close()
	if err != nil || target != noteKey("B.md") {
		t.Fatalf("target = %q, error %v", target, err)
	}
	before, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildFull(vault); err == nil {
		t.Fatal("full build did not read inaccessible note")
	}
	after, err := os.ReadFile(dbPath(vault))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed full build changed DB", err)
	}
}

func TestBuildIncrementalVirtualNamesMatchFull(t *testing.T) {
	for _, change := range []string{"delete", "edit", "add-earlier", "same-line", "move"} {
		t.Run(change, func(t *testing.T) {
			vault := t.TempDir()
			buildTestWrite(t, vault, "B.md", "#UPPER [[Missing]] [path](/abs/Missing.md)\n")
			buildTestWrite(t, vault, "C.md", "#upper [[missing]] [path](/other/missing.md)\n")
			assertBuildMatchesFull(t, vault)
			switch change {
			case "delete":
				if err := os.Remove(filepath.Join(vault, "B.md")); err != nil {
					t.Fatal(err)
				}
			case "edit":
				buildTestWrite(t, vault, "B.md", "#other [[Elsewhere]]\n")
			case "add-earlier":
				buildTestWrite(t, vault, "A.md", "#Upper [[MISSING]]\n")
			case "same-line":
				buildTestWrite(t, vault, "B.md", "#upper #UPPER [[missing]] [[Missing]]\n")
			case "move":
				if err := os.Rename(filepath.Join(vault, "B.md"), filepath.Join(vault, "Z.md")); err != nil {
					t.Fatal(err)
				}
			}
			assertBuildMatchesFull(t, vault)
		})
	}
}
