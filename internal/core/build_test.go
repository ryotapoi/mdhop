package core

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/testutil"
)

// --- Test helpers ---

type edgeRow struct {
	sourcePath string
	targetKey  string
	targetType string
	targetName string
	linkType   string
	rawLink    string
	subpath    string
	lineStart  int
	lineEnd    int
}

type nodeRow struct {
	nodeKey    string
	nodeType   string
	name       string
	path       string
	existsFlag int
}

func copyVault(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", name)
	dst := filepath.Join(t.TempDir(), "vault")
	if err := testutil.CopyDir(root, dst); err != nil {
		t.Fatalf("copy vault: %v", err)
	}
	return dst
}

func openTestDB(t *testing.T, dbp string) *sql.DB {
	t.Helper()
	db, err := openDBAt(dbp)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func countNotes(t *testing.T, dbp string) int {
	t.Helper()
	db := openTestDB(t, dbp)
	defer db.Close()
	row := db.QueryRow("SELECT COUNT(*) FROM nodes WHERE type='note'")
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scan count: %v", err)
	}
	return count
}

func queryEdges(t *testing.T, dbp, sourcePath string) []edgeRow {
	t.Helper()
	db := openTestDB(t, dbp)
	defer db.Close()
	rows, err := db.Query(`
		SELECT sn.path, tn.node_key, tn.type, tn.name, e.link_type, e.raw_link, COALESCE(e.subpath,''), e.line_start, e.line_end
		FROM edges e
		JOIN nodes sn ON sn.id = e.source_id
		JOIN nodes tn ON tn.id = e.target_id
		WHERE sn.path = ?
		ORDER BY e.line_start, e.id`, sourcePath)
	if err != nil {
		t.Fatalf("query edges: %v", err)
	}
	defer rows.Close()
	var out []edgeRow
	for rows.Next() {
		var e edgeRow
		if err := rows.Scan(&e.sourcePath, &e.targetKey, &e.targetType, &e.targetName, &e.linkType, &e.rawLink, &e.subpath, &e.lineStart, &e.lineEnd); err != nil {
			t.Fatalf("scan edge: %v", err)
		}
		out = append(out, e)
	}
	return out
}

func queryNodes(t *testing.T, dbp, nodeType string) []nodeRow {
	t.Helper()
	db := openTestDB(t, dbp)
	defer db.Close()
	rows, err := db.Query(`SELECT node_key, type, name, COALESCE(path,''), exists_flag FROM nodes WHERE type = ? ORDER BY node_key`, nodeType)
	if err != nil {
		t.Fatalf("query nodes: %v", err)
	}
	defer rows.Close()
	var out []nodeRow
	for rows.Next() {
		var n nodeRow
		if err := rows.Scan(&n.nodeKey, &n.nodeType, &n.name, &n.path, &n.existsFlag); err != nil {
			t.Fatalf("scan node: %v", err)
		}
		out = append(out, n)
	}
	return out
}

func countEdges(t *testing.T, dbp string) int {
	t.Helper()
	db := openTestDB(t, dbp)
	defer db.Close()
	row := db.QueryRow("SELECT COUNT(*) FROM edges")
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("scan edge count: %v", err)
	}
	return count
}

// --- Existing tests ---

func TestBuildCreatesDB(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := os.Stat(dbPath(vault)); err != nil {
		t.Fatalf("db not created: %v", err)
	}
}

func TestBuildEmptyVaultCreatesDB(t *testing.T) {
	vault := copyVault(t, "vault_build_empty")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := os.Stat(dbPath(vault)); err != nil {
		t.Fatalf("db not created: %v", err)
	}
	if count := countNotes(t, dbPath(vault)); count != 0 {
		t.Fatalf("expected 0 notes, got %d", count)
	}
}

func TestBuildRebuildOverwritesDB(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	before := countNotes(t, dbPath(vault))

	// Add a new note to force node count change.
	newPath := filepath.Join(vault, "C.md")
	if err := os.WriteFile(newPath, []byte("# C\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	after := countNotes(t, dbPath(vault))
	if after <= before {
		t.Fatalf("expected node count to increase, before=%d after=%d", before, after)
	}
}

func TestBuildFailsOnAmbiguousLink(t *testing.T) {
	vault := copyVault(t, "vault_build_conflict")
	if err := Build(vault); err == nil {
		t.Fatalf("expected build error")
	}
	if _, err := os.Stat(dbPath(vault)); err == nil {
		t.Fatalf("db should not be created on failure")
	}
	if _, err := os.Stat(dbPath(vault) + ".tmp"); err == nil {
		t.Fatalf("temp db should be cleaned up on failure")
	}
}

func TestBuildFailureKeepsExistingDB(t *testing.T) {
	vault := copyVault(t, "vault_build_existing_db")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	before := countNotes(t, dbPath(vault))

	// Add conflicting file to trigger failure.
	conflictDir := filepath.Join(vault, "sub2")
	if err := os.MkdirAll(conflictDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conflictDir, "B.md"), []byte("# B in sub2\n"), 0o644); err != nil {
		t.Fatalf("write conflict: %v", err)
	}
	// Introduce ambiguous link.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("# A\n\nLink to [[B]]\n"), 0o644); err != nil {
		t.Fatalf("write link: %v", err)
	}

	if err := Build(vault); err == nil {
		t.Fatalf("expected build error")
	}
	// Ensure existing DB is still readable and unchanged.
	after := countNotes(t, dbPath(vault))
	if after != before {
		t.Fatalf("expected existing db to be preserved, before=%d after=%d", before, after)
	}
	// Ensure DB file is a valid SQLite file.
	if _, err := sql.Open("sqlite", dbPath(vault)); err != nil {
		t.Fatalf("db should be readable: %v", err)
	}
}

func TestBuildCaseInsensitiveBasename(t *testing.T) {
	vault := copyVault(t, "vault_build_case_insensitive")
	if err := Build(vault); err != nil {
		t.Fatalf("build should succeed with case-insensitive match: %v", err)
	}
}

func TestBuildCaseAmbiguous(t *testing.T) {
	vault := copyVault(t, "vault_build_case_ambiguous")
	if err := Build(vault); err == nil {
		t.Fatalf("expected ambiguous error for case-insensitive basename collision")
	}
}

func TestBuildCaseCollisionWithPathLink(t *testing.T) {
	vault := copyVault(t, "vault_build_case_ambiguous")
	refPath := filepath.Join(vault, "Ref.md")
	if err := os.WriteFile(refPath, []byte("# Ref\n\n[[sub1/Note]]\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build should succeed with path-based link: %v", err)
	}
}

func TestBuildMtime(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	db := openTestDB(t, dbPath(vault))
	defer db.Close()

	rows, err := db.Query("SELECT path, mtime FROM nodes WHERE type='note'")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		var mtime int64
		if err := rows.Scan(&path, &mtime); err != nil {
			t.Fatalf("scan: %v", err)
		}
		info, err := os.Stat(filepath.Join(vault, path))
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		expected := info.ModTime().Unix()
		if mtime != expected {
			t.Errorf("mtime for %s = %d, want %d", path, mtime, expected)
		}
	}
}

func TestBuildFailsOnVaultEscapeLink(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")
	path := filepath.Join(vault, "Escape.md")
	if err := os.WriteFile(path, []byte("# Escape\n\n[up](../Outside.md)\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := Build(vault); err == nil {
		t.Fatalf("expected build error for vault escape link")
	}
	if _, err := os.Stat(dbPath(vault)); err == nil {
		t.Fatalf("db should not be created on failure")
	}
	if _, err := os.Stat(dbPath(vault) + ".tmp"); err == nil {
		t.Fatalf("temp db should be cleaned up on failure")
	}
}

func TestBuildEscapeVaultNonRelative(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")
	// sub/../../Outside.md escapes vault via a non-relative path containing ".."
	if err := os.WriteFile(filepath.Join(vault, "Escape.md"),
		[]byte("[link](sub/../../Outside.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := Build(vault)
	if err == nil || !strings.Contains(err.Error(), "escapes vault") {
		t.Fatalf("expected vault escape error, got: %v", err)
	}
}

func TestBuildEscapeVaultAbsolutePrefix(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")
	// /sub/../../Outside.md also escapes vault (leading / stripped then normalized)
	if err := os.WriteFile(filepath.Join(vault, "Escape.md"),
		[]byte("[link](/sub/../../Outside.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := Build(vault)
	if err == nil || !strings.Contains(err.Error(), "escapes vault") {
		t.Fatalf("expected vault escape error, got: %v", err)
	}
}

func TestBuildNonEscapingDotDot(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")
	// sub/../A.md resolves to A.md which exists — should succeed
	if err := os.WriteFile(filepath.Join(vault, "sub", "B.md"),
		[]byte("[link](sub/../A.md)\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("expected success for non-escaping dotdot path, got: %v", err)
	}
}

// --- Edge tests ---

func TestBuildEdgesWikilink(t *testing.T) {
	vault := copyVault(t, "vault_build_edges")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "A.md")
	// A.md has: [[B]], [C](sub/C.md), [[D]] (phantom), [[#Heading]], [link](/sub/B.md)
	// Expect edges to B, C, D(phantom), self(#Heading), B(absolute)

	var foundB, foundC, foundD, foundSelf, foundAbsB bool
	for _, e := range edges {
		switch {
		case e.linkType == "wikilink" && e.targetName == "B" && e.rawLink == "[[B]]":
			foundB = true
			if e.targetType != "note" {
				t.Errorf("B should be note, got %s", e.targetType)
			}
			if e.lineStart != 3 {
				t.Errorf("B lineStart = %d, want 3", e.lineStart)
			}
		case e.linkType == "markdown" && e.targetName == "C" && e.rawLink == "[C](sub/C.md)":
			foundC = true
			if e.targetType != "note" {
				t.Errorf("C should be note, got %s", e.targetType)
			}
		case e.linkType == "wikilink" && e.targetName == "D":
			foundD = true
			if e.targetType != "phantom" {
				t.Errorf("D should be phantom, got %s", e.targetType)
			}
		case e.linkType == "wikilink" && e.subpath == "#Heading":
			foundSelf = true
			if e.targetType != "note" {
				t.Errorf("self-edge target should be note, got %s", e.targetType)
			}
			// Source and target should be the same (A.md).
			if e.targetKey != noteKey("A.md") {
				t.Errorf("self-edge target key = %s, want %s", e.targetKey, noteKey("A.md"))
			}
		case e.linkType == "markdown" && e.rawLink == "[link](/sub/B.md)":
			foundAbsB = true
			if e.targetType != "note" {
				t.Errorf("absolute link to B should be note, got %s", e.targetType)
			}
		}
	}
	if !foundB {
		t.Error("wikilink edge A→B not found")
	}
	if !foundC {
		t.Error("markdown edge A→C not found")
	}
	if !foundD {
		t.Error("phantom edge A→D not found")
	}
	if !foundSelf {
		t.Error("self-edge A→A#Heading not found")
	}
	if !foundAbsB {
		t.Error("absolute link edge A→B not found")
	}
}

func TestBuildEdgesBacklink(t *testing.T) {
	vault := copyVault(t, "vault_build_edges")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "sub/B.md")
	var foundBacklink bool
	for _, e := range edges {
		if e.linkType == "wikilink" && e.targetName == "A" {
			foundBacklink = true
		}
	}
	if !foundBacklink {
		t.Error("backlink edge B→A not found")
	}
}

func TestBuildEdgesRelativePath(t *testing.T) {
	vault := copyVault(t, "vault_build_relative")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "dir/Source.md")
	var foundTarget, foundRoot bool
	for _, e := range edges {
		switch {
		case e.targetKey == noteKey("dir/Target.md"):
			foundTarget = true
		case e.targetKey == noteKey("Root.md"):
			foundRoot = true
		}
	}
	if !foundTarget {
		t.Error("relative edge Source→Target not found")
	}
	if !foundRoot {
		t.Error("relative edge Source→Root not found")
	}
}

func TestBuildPhantomNodes(t *testing.T) {
	vault := copyVault(t, "vault_build_phantom")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	phantoms := queryNodes(t, dbPath(vault), "phantom")
	// NonExistent and Missing should be phantoms.
	names := make(map[string]bool)
	for _, p := range phantoms {
		names[p.name] = true
		if p.existsFlag != 0 {
			t.Errorf("phantom %s should have exists_flag=0", p.name)
		}
		if p.path != "" {
			t.Errorf("phantom %s should have empty path, got %s", p.name, p.path)
		}
	}
	if !names["NonExistent"] {
		t.Error("phantom NonExistent not found")
	}
	if !names["Missing"] {
		t.Error("phantom Missing not found")
	}

	// Check that phantom name preserves original case.
	for _, p := range phantoms {
		if p.name == "NonExistent" {
			if p.nodeKey != "phantom:name:nonexistent" {
				t.Errorf("phantom key = %s, want phantom:name:nonexistent", p.nodeKey)
			}
		}
	}
}

func TestBuildPhantomEdges(t *testing.T) {
	vault := copyVault(t, "vault_build_phantom")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "A.md")
	var foundNonExistent, foundMissing bool
	for _, e := range edges {
		if e.targetType == "phantom" && e.targetName == "NonExistent" {
			foundNonExistent = true
		}
		if e.targetType == "phantom" && e.targetName == "Missing" {
			foundMissing = true
			if e.rawLink != "[[Missing|alias]]" {
				t.Errorf("alias phantom rawLink = %q, want [[Missing|alias]]", e.rawLink)
			}
		}
	}
	if !foundNonExistent {
		t.Error("phantom edge A→NonExistent not found")
	}
	if !foundMissing {
		t.Error("phantom edge A→Missing not found")
	}
}

func TestBuildBasicEdgeCount(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// A.md has [[B]], B.md has no links.
	// A→B edge should exist.
	edges := queryEdges(t, dbPath(vault), "A.md")
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge from A.md, got %d: %+v", len(edges), edges)
	}
	if edges[0].targetName != "B" {
		t.Errorf("edge target = %s, want B", edges[0].targetName)
	}
}

// --- Tag tests ---

func TestBuildTagsInline(t *testing.T) {
	vault := copyVault(t, "vault_build_tags")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	tags := queryNodes(t, dbPath(vault), "tag")
	tagNames := make(map[string]bool)
	for _, tag := range tags {
		tagNames[tag.name] = true
	}
	if !tagNames["#simple"] {
		t.Error("tag #simple not found")
	}
	if !tagNames["#parent"] {
		t.Error("tag #parent not found (nested expansion)")
	}
	if !tagNames["#parent/child"] {
		t.Error("tag #parent/child not found")
	}
}

func TestBuildTagsFrontmatter(t *testing.T) {
	vault := copyVault(t, "vault_build_tags")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "A.md")
	var foundFmTag, foundFmNested bool
	for _, e := range edges {
		if e.linkType == "frontmatter" && e.targetName == "#fm_tag" {
			foundFmTag = true
			// frontmatter tag at line 3 ("  - fm_tag" is YAML line 2, + offset 1 = file line 3)
			if e.lineStart != 3 {
				t.Errorf("fm_tag lineStart = %d, want 3", e.lineStart)
			}
		}
		if e.linkType == "frontmatter" && e.targetName == "#nested/deep/tag" {
			foundFmNested = true
		}
	}
	if !foundFmTag {
		t.Error("frontmatter tag edge A→#fm_tag not found")
	}
	if !foundFmNested {
		t.Error("frontmatter nested tag edge A→#nested/deep/tag not found")
	}
}

func TestBuildTagsFrontmatterNestedExpansion(t *testing.T) {
	vault := copyVault(t, "vault_build_tags")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	tags := queryNodes(t, dbPath(vault), "tag")
	tagNames := make(map[string]bool)
	for _, tag := range tags {
		tagNames[tag.name] = true
	}
	// nested/deep/tag should produce #nested, #nested/deep, #nested/deep/tag
	if !tagNames["#nested"] {
		t.Error("tag #nested not found (frontmatter nested expansion)")
	}
	if !tagNames["#nested/deep"] {
		t.Error("tag #nested/deep not found (frontmatter nested expansion)")
	}
	if !tagNames["#nested/deep/tag"] {
		t.Error("tag #nested/deep/tag not found (frontmatter nested expansion)")
	}
}

func TestBuildTagsInlineEdge(t *testing.T) {
	vault := copyVault(t, "vault_build_tags")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "A.md")
	var foundInlineTag bool
	for _, e := range edges {
		if e.linkType == "tag" && e.targetName == "#simple" {
			foundInlineTag = true
		}
	}
	if !foundInlineTag {
		t.Error("inline tag edge A→#simple not found")
	}
}

func TestBuildTagsCodeFenceExcluded(t *testing.T) {
	vault := copyVault(t, "vault_build_tag_codefence")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	tags := queryNodes(t, dbPath(vault), "tag")
	for _, tag := range tags {
		if tag.name == "#not-a-tag" {
			t.Error("tag in code fence should not be created")
		}
	}
	// Should have #tag only
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
}

func TestBuildTagsSharedAcrossFiles(t *testing.T) {
	vault := copyVault(t, "vault_build_tags")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	// Both A.md and B.md have #simple
	edgesA := queryEdges(t, dbPath(vault), "A.md")
	edgesB := queryEdges(t, dbPath(vault), "B.md")
	var aHasSimple, bHasSimple bool
	var aTargetKey, bTargetKey string
	for _, e := range edgesA {
		if e.linkType == "tag" && e.targetName == "#simple" {
			aHasSimple = true
			aTargetKey = e.targetKey
		}
	}
	for _, e := range edgesB {
		if e.linkType == "tag" && e.targetName == "#simple" {
			bHasSimple = true
			bTargetKey = e.targetKey
		}
	}
	if !aHasSimple || !bHasSimple {
		t.Fatalf("both files should link to #simple, A=%v B=%v", aHasSimple, bHasSimple)
	}
	if aTargetKey != bTargetKey {
		t.Errorf("both should point to same tag node, A=%s B=%s", aTargetKey, bTargetKey)
	}
}

func TestBuildTagsUnicode(t *testing.T) {
	vault := copyVault(t, "vault_build_tags_unicode")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	tags := queryNodes(t, dbPath(vault), "tag")
	tagNames := make(map[string]bool)
	for _, tag := range tags {
		tagNames[tag.name] = true
	}
	// Inline Unicode tags
	if !tagNames["#my-tag"] {
		t.Error("tag #my-tag not found")
	}
	if !tagNames["#あいうえお"] {
		t.Error("tag #あいうえお not found")
	}
	// Nested Unicode expansion
	if !tagNames["#parent"] {
		t.Error("tag #parent not found (nested expansion)")
	}
	if !tagNames["#parent/子タグ"] {
		t.Error("tag #parent/子タグ not found")
	}
	// Frontmatter Unicode tag
	if !tagNames["#日本語タグ"] {
		t.Error("tag #日本語タグ not found (frontmatter)")
	}
	// Digit-first should not exist
	if tagNames["#123"] {
		t.Error("digit-first #123 should not be a tag")
	}
	// #my-tag shared between A.md and B.md
	edgesA := queryEdges(t, dbPath(vault), "A.md")
	edgesB := queryEdges(t, dbPath(vault), "B.md")
	var aTargetKey, bTargetKey string
	for _, e := range edgesA {
		if e.targetName == "#my-tag" {
			aTargetKey = e.targetKey
		}
	}
	for _, e := range edgesB {
		if e.targetName == "#my-tag" {
			bTargetKey = e.targetKey
		}
	}
	if aTargetKey == "" || bTargetKey == "" {
		t.Fatalf("#my-tag should be shared: A=%q B=%q", aTargetKey, bTargetKey)
	}
	if aTargetKey != bTargetKey {
		t.Errorf("should be same tag node: A=%s B=%s", aTargetKey, bTargetKey)
	}
	// Total tag count: #my-tag, #あいうえお, #parent, #parent/子タグ, #日本語タグ = 5
	if len(tags) != 5 {
		names := make([]string, 0, len(tags))
		for _, tag := range tags {
			names = append(names, tag.name)
		}
		t.Errorf("expected 5 tags, got %d: %v", len(tags), names)
	}
}

// --- Integration tests ---

func TestBuildFullVault(t *testing.T) {
	vault := copyVault(t, "vault_build_full")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// 3 notes: Index.md, Design.md, sub/Impl.md
	notes := queryNodes(t, dbPath(vault), "note")
	if len(notes) != 3 {
		t.Errorf("expected 3 notes, got %d", len(notes))
	}

	// Phantoms: Missing, NonExistent
	phantoms := queryNodes(t, dbPath(vault), "phantom")
	if len(phantoms) != 2 {
		t.Errorf("expected 2 phantoms, got %d: %+v", len(phantoms), phantoms)
	}

	// Tags: #project, #status, #status/active, #overview, #design, #code
	tags := queryNodes(t, dbPath(vault), "tag")
	tagNames := make(map[string]bool)
	for _, tag := range tags {
		tagNames[tag.name] = true
	}
	expectedTags := []string{"#project", "#status", "#status/active", "#overview", "#design", "#code"}
	for _, et := range expectedTags {
		if !tagNames[et] {
			t.Errorf("tag %s not found", et)
		}
	}
	if len(tags) != len(expectedTags) {
		t.Errorf("expected %d tags, got %d: %+v", len(expectedTags), len(tags), tags)
	}

	// Verify mtime is set on all notes.
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	rows, err := db.Query("SELECT path, mtime FROM nodes WHERE type='note'")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		var mtime int64
		if err := rows.Scan(&path, &mtime); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if mtime == 0 {
			t.Errorf("mtime for %s is 0", path)
		}
	}

	// Verify self-edge [[#Index]]
	indexEdges := queryEdges(t, dbPath(vault), "Index.md")
	var foundSelfEdge bool
	for _, e := range indexEdges {
		if e.subpath == "#Index" && e.targetKey == noteKey("Index.md") {
			foundSelfEdge = true
		}
	}
	if !foundSelfEdge {
		t.Error("self-edge Index→Index#Index not found")
	}

	// Verify key edges exist
	designEdges := queryEdges(t, dbPath(vault), "Design.md")
	var designToIndex, designToImplSubpath bool
	for _, e := range designEdges {
		if e.targetKey == noteKey("Index.md") && e.linkType == "wikilink" {
			designToIndex = true
		}
		if e.targetKey == noteKey("sub/Impl.md") && e.subpath == "#Details" {
			designToImplSubpath = true
		}
	}
	if !designToIndex {
		t.Error("edge Design→Index not found")
	}
	if !designToImplSubpath {
		t.Error("edge Design→sub/Impl#Details not found")
	}

	// Verify absolute link from sub/Impl.md → Index.md
	implEdges := queryEdges(t, dbPath(vault), "sub/Impl.md")
	var implAbsToIndex bool
	for _, e := range implEdges {
		if e.targetKey == noteKey("Index.md") && e.linkType == "markdown" {
			implAbsToIndex = true
		}
	}
	if !implAbsToIndex {
		t.Error("absolute link edge Impl→Index not found")
	}

	// Verify total edge count is reasonable.
	totalEdges := countEdges(t, dbPath(vault))
	if totalEdges < 10 {
		t.Errorf("expected at least 10 edges, got %d", totalEdges)
	}
}

func TestBuildRootPriority(t *testing.T) {
	// A.md at root + sub/A.md → basename "A" collides.
	// B.md has [[A]] → root priority resolves to root A.md → build succeeds.
	vault := copyVault(t, "vault_build_root_priority")
	if err := Build(vault); err != nil {
		t.Fatalf("expected build success (root priority), got: %v", err)
	}

	// [[A]] in B.md should point to root A.md.
	edges := queryEdges(t, dbPath(vault), "B.md")
	var foundA bool
	for _, e := range edges {
		if e.targetName == "A" && e.targetType == "note" {
			foundA = true
			// Check it resolves to the root file via DB query.
			db := openTestDB(t, dbPath(vault))
			var path string
			err := db.QueryRow("SELECT path FROM nodes WHERE id = (SELECT target_id FROM edges WHERE raw_link='[[A]]' AND source_id = (SELECT id FROM nodes WHERE path='B.md'))").Scan(&path)
			db.Close()
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			if path != "A.md" {
				t.Errorf("[[A]] resolves to %q, want A.md (root)", path)
			}
		}
	}
	if !foundA {
		t.Error("B→A edge not found")
	}
}

func TestBuildRootPriorityNoRoot(t *testing.T) {
	// sub1/A.md + sub2/A.md (no root A). B.md has [[A]] → ambiguous → error.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("# A1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub2", "A.md"), []byte("# A2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(vault); err == nil {
		t.Fatal("expected build error for ambiguous link (no root)")
	}
}

func TestBuildRootPriorityRoundTrip(t *testing.T) {
	// Build → add (causes collision, root-priority resolves) → rebuild → should succeed.
	vault := copyVault(t, "vault_build_root_priority")

	// Initial build: A.md(root) + sub/A.md + B.md has [[A]].
	if err := Build(vault); err != nil {
		t.Fatalf("first build: %v", err)
	}

	// Add another file with same basename at subdir.
	sub2 := filepath.Join(vault, "sub2")
	if err := os.MkdirAll(sub2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub2, "A.md"), []byte("# A in sub2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Add(vault, AddOptions{Files: []string{"sub2/A.md"}})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// Rebuild should still succeed (root-priority resolves [[A]] to root A.md).
	if err := Build(vault); err != nil {
		t.Fatalf("rebuild after add: %v", err)
	}
}

func TestBuildExcludesMdhopDir(t *testing.T) {
	vault := copyVault(t, "vault_build_basic")
	// Create a .md file inside .mdhop dir — it should be excluded from the index.
	mdhopDir := filepath.Join(vault, ".mdhop")
	if err := os.MkdirAll(mdhopDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mdhopDir, "test.md"), []byte("# Hidden\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	notes := queryNodes(t, dbPath(vault), "note")
	for _, n := range notes {
		if strings.Contains(n.path, ".mdhop") {
			t.Errorf("file inside .mdhop should be excluded: %s", n.path)
		}
	}
}

func TestBuildEdgeLineEnd(t *testing.T) {
	vault := copyVault(t, "vault_build_edges")
	if err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	edges := queryEdges(t, dbPath(vault), "A.md")
	for _, e := range edges {
		if e.lineEnd < e.lineStart {
			t.Errorf("edge %s: lineEnd (%d) < lineStart (%d)", e.rawLink, e.lineEnd, e.lineStart)
		}
		// All links in A.md are single-line, so lineStart == lineEnd.
		if e.lineEnd != e.lineStart {
			t.Errorf("edge %s: expected lineStart == lineEnd for single-line link, got %d != %d", e.rawLink, e.lineStart, e.lineEnd)
		}
	}
}

func TestBuildIdempotent(t *testing.T) {
	vault := copyVault(t, "vault_build_full")

	// Build twice.
	if err := Build(vault); err != nil {
		t.Fatalf("first build: %v", err)
	}
	firstNotes := countNotes(t, dbPath(vault))
	firstEdges := countEdges(t, dbPath(vault))
	firstPhantoms := len(queryNodes(t, dbPath(vault), "phantom"))
	firstTags := len(queryNodes(t, dbPath(vault), "tag"))

	if err := Build(vault); err != nil {
		t.Fatalf("second build: %v", err)
	}
	secondNotes := countNotes(t, dbPath(vault))
	secondEdges := countEdges(t, dbPath(vault))
	secondPhantoms := len(queryNodes(t, dbPath(vault), "phantom"))
	secondTags := len(queryNodes(t, dbPath(vault), "tag"))

	if firstNotes != secondNotes {
		t.Errorf("notes changed: %d → %d", firstNotes, secondNotes)
	}
	if firstEdges != secondEdges {
		t.Errorf("edges changed: %d → %d", firstEdges, secondEdges)
	}
	if firstPhantoms != secondPhantoms {
		t.Errorf("phantoms changed: %d → %d", firstPhantoms, secondPhantoms)
	}
	if firstTags != secondTags {
		t.Errorf("tags changed: %d → %d", firstTags, secondTags)
	}
}

func TestBuildCollectsMultipleErrors(t *testing.T) {
	vault := copyVault(t, "vault_build_multi_error")
	err := Build(vault)
	if err == nil {
		t.Fatal("expected build error")
	}
	msg := err.Error()
	// Should contain all 3 errors: 2 ambiguous + 1 escape.
	if !strings.Contains(msg, "ambiguous link: A") {
		t.Errorf("missing ambiguous A error: %s", msg)
	}
	if !strings.Contains(msg, "ambiguous link: B") {
		t.Errorf("missing ambiguous B error: %s", msg)
	}
	if !strings.Contains(msg, "escapes vault") {
		t.Errorf("missing vault escape error: %s", msg)
	}
	if !strings.Contains(msg, "3 errors total") {
		t.Errorf("missing summary line: %s", msg)
	}
	// DB should not be created.
	if _, err := os.Stat(dbPath(vault)); err == nil {
		t.Error("DB should not exist after build failure")
	}
}

func TestBuildSingleErrorFormatUnchanged(t *testing.T) {
	vault := copyVault(t, "vault_build_conflict")
	err := Build(vault)
	if err == nil {
		t.Fatal("expected build error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ambiguous link") {
		t.Errorf("expected ambiguous link error, got: %s", msg)
	}
	// Single error should NOT contain "errors" summary.
	if strings.Contains(msg, "errors") {
		t.Errorf("single error should not have summary line, got: %s", msg)
	}
}

func TestBuildErrorCapAtMax(t *testing.T) {
	vault := t.TempDir()
	// Create 7 ambiguous basenames (each in sub1/ and sub2/, no root).
	for _, dir := range []string{"sub1", "sub2"} {
		if err := os.MkdirAll(filepath.Join(vault, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	names := []string{"A", "B", "C", "D", "E", "F", "G"}
	for _, name := range names {
		for _, dir := range []string{"sub1", "sub2"} {
			path := filepath.Join(vault, dir, name+".md")
			if err := os.WriteFile(path, []byte("# "+name+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		// Reference file with ambiguous link.
		refPath := filepath.Join(vault, "Ref_"+name+".md")
		if err := os.WriteFile(refPath, []byte("[["+name+"]]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := Build(vault)
	if err == nil {
		t.Fatal("expected build error")
	}
	msg := err.Error()
	// Should be capped at maxBuildErrors (5).
	lines := strings.Split(strings.TrimSpace(msg), "\n")
	// 5 error lines + 1 summary line = 6 lines.
	if len(lines) != 6 {
		t.Errorf("expected 6 lines (5 errors + summary), got %d:\n%s", len(lines), msg)
	}
	if !strings.Contains(msg, "too many errors (first 5 shown)") {
		t.Errorf("missing cap summary: %s", msg)
	}
}
