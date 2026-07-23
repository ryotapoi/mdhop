package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Test 4: move causes ambiguous links (root priority resolves) ---
func TestMove_AmbiguousAfterMove(t *testing.T) {
	// A.md has [[C]], B.md has [[A]], C.md exists at root.
	// Move A.md to sub/C.md → basename "C" now has C.md + sub/C.md.
	// Root priority: C.md is at root → [[C]] resolves to root C.md → no ambiguity.
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[C]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Move(vault, MoveOptions{From: "A.md", To: "sub/C.md"})
	if err != nil {
		t.Fatalf("expected success (root priority), got: %v", err)
	}
}

// --- Test 4b: move causes ambiguous outgoing link (no root file) → outgoing rewrite ---
func TestMove_AmbiguousAfterMoveNoRoot(t *testing.T) {
	// A.md has [[C]], sub/C.md exists (no root C).
	// Move A.md to sub2/C.md → basename "C" has sub/C.md + sub2/C.md, no root.
	// → outgoing [[C]] is rewritten to [[sub/C]] (pre-move target).
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[C]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub2/C.md"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// [[C]] should be rewritten to [[sub/C]] in the moved file.
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub2/C.md" && rw.OldLink == "[[C]]" {
			found = true
			if rw.NewLink != "[[sub/C]]" {
				t.Errorf("expected [[sub/C]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("outgoing [[C]] should be rewritten")
	}

	content, err := os.ReadFile(filepath.Join(vault, "sub2", "C.md"))
	if err != nil {
		t.Fatalf("read sub2/C.md: %v", err)
	}
	if !strings.Contains(string(content), "[[sub/C]]") {
		t.Errorf("disk should contain [[sub/C]], got: %s", string(content))
	}

	edges := queryEdges(t, dbPath(vault), "sub2/C.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub/C]]" && e.targetName == "C" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub/C]]")
	}
}

// --- Test 5: basename unchanged + unique → links preserved (no rewrite) ---
func TestMove_BasenameUnchanged(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move A.md to sub/A.md. Basename stays "A".
	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// B.md has [[A]] — basename link, unchanged and unique → no rewrite needed.
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			t.Errorf("[[A]] in B.md should NOT be rewritten, but was rewritten to %s", rw.NewLink)
		}
	}

	if fileExists(filepath.Join(vault, "A.md")) {
		t.Error("A.md should not exist on disk after move")
	}
	if !fileExists(filepath.Join(vault, "sub", "A.md")) {
		t.Error("sub/A.md should exist on disk after move")
	}

	// Verify DB updated.
	dbp := dbPath(vault)
	notes := queryNodes(t, dbp, "note")
	var foundNew bool
	for _, n := range notes {
		if n.path == "sub/A.md" {
			foundNew = true
		}
		if n.path == "A.md" {
			t.Error("DB still contains old path A.md")
		}
	}
	if !foundNew {
		t.Error("DB does not contain new path sub/A.md")
	}

	// Verify B.md's edge still targets the moved note via basename.
	edges := queryEdges(t, dbp, "B.md")
	var hasLinkToA bool
	for _, e := range edges {
		if e.targetName == "A" && e.linkType == LinkTypeWikilink && e.rawLink == "[[A]]" {
			hasLinkToA = true
		}
	}
	if !hasLinkToA {
		t.Error("B.md should still have [[A]] link")
	}
}

// --- Test 6: path links are always rewritten ---
func TestMove_PathLinkAlwaysRewritten(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// C.md has [link to A](./A.md) — path link.
	// sub/D.md has [path link](../A.md) — path link.
	// Move A.md to sub/A.md.
	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// C.md's path link should be rewritten.
	var cRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "C.md" && rw.OldLink == "[link to A](./A.md)" {
			cRewritten = true
			// Target is sub/A.md, source is C.md (root). So new link should be sub/A.md.
			if !strings.Contains(rw.NewLink, "sub/A") {
				t.Errorf("C.md rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !cRewritten {
		t.Error("C.md path link should be rewritten")
	}

	// sub/D.md's path link [path link](../A.md) should be rewritten.
	// Target is sub/A.md, source is sub/D.md. buildRewritePath gives vault-relative
	// for subdirectory targets: "sub/A.md".
	var dRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub/D.md" && rw.OldLink == "[path link](../A.md)" {
			dRewritten = true
			if rw.NewLink != "[path link](sub/A.md)" {
				t.Errorf("sub/D.md rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !dRewritten {
		t.Error("sub/D.md path link should be rewritten")
	}

	cContent, err := os.ReadFile(filepath.Join(vault, "C.md"))
	if err != nil {
		t.Fatalf("read C.md: %v", err)
	}
	if strings.Contains(string(cContent), "./A.md)") {
		t.Error("C.md should not contain ./A.md anymore")
	}
}

// --- Test 7: basename changes → basename links rewritten ---
func TestMove_BasenameChanged(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// B.md has [[A]], sub/D.md has [[A]].
	// Rename A.md to X.md — basename changes from A to X.
	result, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// [[A]] in B.md should be rewritten because basename changed.
	// X.md is at root → vault-relative rewrite gives [[X]].
	var bRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			bRewritten = true
			if rw.NewLink != "[[X]]" {
				t.Errorf("B.md rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !bRewritten {
		t.Error("B.md [[A]] should be rewritten when basename changes")
	}

	// sub/D.md [[A]] should be rewritten too.
	// X.md is at root → vault-relative rewrite gives [[X]].
	var dWikiRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub/D.md" && rw.OldLink == "[[A]]" {
			dWikiRewritten = true
			if rw.NewLink != "[[X]]" {
				t.Errorf("sub/D.md wikilink rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !dWikiRewritten {
		t.Error("sub/D.md [[A]] should be rewritten when basename changes")
	}

	notes := queryNodes(t, dbPath(vault), "note")
	var foundX bool
	for _, n := range notes {
		if n.path == "X.md" {
			foundX = true
		}
	}
	if !foundX {
		t.Error("DB should contain X.md after move")
	}
}

// --- Test 8: outgoing relative links rewritten ---
func TestMove_OutgoingRelativeRewritten(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// A.md has:
	//   [link to B](./B.md)   — relative link to B.md
	//   [link to C](./C.md)   — relative link to C.md
	// Move A.md to sub/A.md.
	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// Outgoing relative links should be rewritten.
	var bOutRewrite, cOutRewrite bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub/A.md" && rw.OldLink == "[link to B](./B.md)" {
			bOutRewrite = true
			if rw.NewLink != "[link to B](../B.md)" {
				t.Errorf("outgoing B link rewrite unexpected: %s", rw.NewLink)
			}
		}
		if rw.File == "sub/A.md" && rw.OldLink == "[link to C](./C.md)" {
			cOutRewrite = true
			if rw.NewLink != "[link to C](../C.md)" {
				t.Errorf("outgoing C link rewrite unexpected: %s", rw.NewLink)
			}
		}
	}
	if !bOutRewrite {
		t.Error("outgoing link to B should be rewritten")
	}
	if !cOutRewrite {
		t.Error("outgoing link to C should be rewritten")
	}

	content, err := os.ReadFile(filepath.Join(vault, "sub", "A.md"))
	if err != nil {
		t.Fatalf("read sub/A.md: %v", err)
	}
	if !strings.Contains(string(content), "../B.md") {
		t.Error("sub/A.md should contain ../B.md")
	}
	if !strings.Contains(string(content), "../C.md") {
		t.Error("sub/A.md should contain ../C.md")
	}
}

func TestMove_OutgoingRelativeRootLinkIsCleaned(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("[[../]]\n[root](../)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "sub/A.md", To: "other/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	wantRewrites := map[string]string{
		"[[../]]":     "[[..]]",
		"[root](../)": "[root](..)",
	}
	for _, rw := range result.Rewritten {
		if rw.File != "other/A.md" {
			continue
		}
		if want, ok := wantRewrites[rw.OldLink]; ok {
			if rw.NewLink != want {
				t.Errorf("rewrite %q: got %q, want %q", rw.OldLink, rw.NewLink, want)
			}
			delete(wantRewrites, rw.OldLink)
		}
		if strings.Contains(rw.NewLink, "/.") {
			t.Errorf("rewrite %q should not contain trailing /. segment: %q", rw.OldLink, rw.NewLink)
		}
	}
	for oldLink, want := range wantRewrites {
		t.Errorf("missing rewrite for %q to %q", oldLink, want)
	}

	content, err := os.ReadFile(filepath.Join(vault, "other", "A.md"))
	if err != nil {
		t.Fatalf("read other/A.md: %v", err)
	}
	gotContent := string(content)
	if strings.Contains(gotContent, "../.") {
		t.Errorf("moved content should not contain ../., got:\n%s", gotContent)
	}
	if !strings.Contains(gotContent, "[[..]]") || !strings.Contains(gotContent, "[root](..)") {
		t.Errorf("moved content missing cleaned root links, got:\n%s", gotContent)
	}

	edges := queryEdges(t, dbPath(vault), "other/A.md")
	wantRawLinks := map[string]bool{
		"[[..]]":     false,
		"[root](..)": false,
	}
	for _, e := range edges {
		if strings.Contains(e.rawLink, "../.") {
			t.Errorf("DB edge raw_link should not contain ../.: %q", e.rawLink)
		}
		if _, ok := wantRawLinks[e.rawLink]; ok {
			wantRawLinks[e.rawLink] = true
		}
	}
	for rawLink, found := range wantRawLinks {
		if !found {
			t.Errorf("DB edge should contain cleaned raw_link %q, got edges: %+v", rawLink, edges)
		}
	}
}

// --- Test 9: phantom promotion ---
// --- Test 13: external rewrite succeeds even when target file has stale mtime ---
func TestMove_ExternalRewriteWithStaleFile(t *testing.T) {
	vault := copyVault(t, "vault_move_basic")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Modify C.md (which has a path link to A.md) after build to make it stale.
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("[link to A](./A.md)\n[[B]]\nmodified\n"), 0o644); err != nil {
		t.Fatalf("write C.md: %v", err)
	}

	// Rename A.md to X.md — C.md is stale but external rewrite should still succeed.
	_, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err != nil {
		t.Fatalf("expected success despite stale C.md, got: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "C.md"))
	if err != nil {
		t.Fatalf("read C.md: %v", err)
	}
	if !strings.Contains(string(content), "X.md") {
		t.Error("C.md should have rewritten link to X.md")
	}
}

// --- Test 14: self-reference in moved file ---
func TestMove_SelfReference(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[#Heading]]\n## Heading\ncontent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move with self-reference: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(vault, "sub", "A.md"))
	if err != nil {
		t.Fatalf("read sub/A.md: %v", err)
	}
	if !strings.Contains(string(content), "[[#Heading]]") {
		t.Error("self-reference [[#Heading]] should be preserved")
	}
}

func TestMove_RelativeSelfLinksKeepPointingToMovedNote(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "wikilink",
			content: "[[./a]]\n",
			want:    "[[./a]]\n",
		},
		{
			name:    "markdown",
			content: "[self](./a.md)\n",
			want:    "[self](./a.md)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vault := newMoveVault(t, map[string]string{
				"a.md": tt.content,
			})

			result, err := Move(vault, MoveOptions{From: "a.md", To: "sub/a.md"})
			if err != nil {
				t.Fatalf("move: %v", err)
			}

			got := readVaultFile(t, vault, "sub/a.md")
			if got != tt.want {
				t.Fatalf("moved note content = %q, want %q; rewrites: %+v", got, tt.want, result.Rewritten)
			}
			assertEdgeRawLinks(t, vault, "sub/a.md", []string{strings.TrimSpace(tt.want)})
		})
	}
}

func TestMove_ParentChildPathIsSingleFileMove(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"a.md": "content\n",
	})

	if _, err := Move(vault, MoveOptions{From: "a.md", To: "a/a.md"}); err != nil {
		t.Fatalf("move from parent-like path to child path should succeed: %v", err)
	}
	if fileExists(filepath.Join(vault, "a.md")) {
		t.Fatal("a.md should not exist after move")
	}
	if !fileExists(filepath.Join(vault, "a", "a.md")) {
		t.Fatal("a/a.md should exist after move")
	}
}

// --- Test 15: Phase 2.5 — root priority resolves third-party ambiguity ---
func TestMove_AmbiguousThirdPartyRootPriority(t *testing.T) {
	vault := t.TempDir()
	// A.md at root. B.md links to [[A]]. C.md exists.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move C.md to sub/A.md → basename "A" count becomes 2.
	// Root priority: A.md is at root before AND after move → [[A]] resolves to root.
	_, err := Move(vault, MoveOptions{From: "C.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("expected success (root priority), got: %v", err)
	}
}

// --- Test 15b: Phase 2.5 — no root file → collateral rewrite ---
func TestMove_AmbiguousThirdPartyNoRoot(t *testing.T) {
	vault := t.TempDir()
	// sub1/A.md exists (not at root). B.md links to [[A]]. C.md exists.
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Move C.md to sub2/A.md → basename "A" has sub1/A.md + sub2/A.md, no root.
	// B.md's [[A]] (pointing to sub1/A.md) should be collateral-rewritten to [[sub1/A]].
	result, err := Move(vault, MoveOptions{From: "C.md", To: "sub2/A.md"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			found = true
			if rw.NewLink != "[[sub1/A]]" {
				t.Errorf("expected [[sub1/A]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("B.md [[A]] should be collateral-rewritten to [[sub1/A]]")
	}

	bContent, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(bContent), "[[sub1/A]]") {
		t.Errorf("B.md disk should contain [[sub1/A]], got: %s", string(bContent))
	}

	edges := queryEdges(t, dbPath(vault), "B.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub1/A]]" && e.targetName == "A" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub1/A]]")
	}
}

// --- Test 16: both from and to absent on disk ---
// --- Test 18: multiple incoming rewrites from different files ---
func TestMove_MultipleIncomingRewrites(t *testing.T) {
	vault := t.TempDir()
	// A.md exists. B.md, C.md, D.md all have path links to A.md.
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[b link](./A.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("[[./A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "D.md"), []byte("[d link](./A.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// All three files should have rewritten links with correct new values.
	rewrittenFiles := make(map[string]string)
	for _, rw := range result.Rewritten {
		if rw.File == "sub/A.md" {
			continue // outgoing rewrites
		}
		rewrittenFiles[rw.File] = rw.NewLink
	}

	// Target has subdirectory → vault-relative rewrite (no ./ prefix).
	// B.md: [b link](./A.md) → [b link](sub/A.md)
	if got, ok := rewrittenFiles["B.md"]; !ok {
		t.Error("B.md path link should be rewritten")
	} else if got != "[b link](sub/A.md)" {
		t.Errorf("B.md new link = %q, want %q", got, "[b link](sub/A.md)")
	}
	// C.md: [[./A]] → [[sub/A]]
	if got, ok := rewrittenFiles["C.md"]; !ok {
		t.Error("C.md wikilink path link should be rewritten")
	} else if got != "[[sub/A]]" {
		t.Errorf("C.md new link = %q, want %q", got, "[[sub/A]]")
	}
	// D.md: [d link](./A.md) → [d link](sub/A.md)
	if got, ok := rewrittenFiles["D.md"]; !ok {
		t.Error("D.md path link should be rewritten")
	} else if got != "[d link](sub/A.md)" {
		t.Errorf("D.md new link = %q, want %q", got, "[d link](sub/A.md)")
	}

	// Verify disk content was actually rewritten with correct new links.
	bContent, _ := os.ReadFile(filepath.Join(vault, "B.md"))
	if !strings.Contains(string(bContent), "sub/A.md") {
		t.Errorf("B.md should contain sub/A.md, got: %s", string(bContent))
	}
	cContent, _ := os.ReadFile(filepath.Join(vault, "C.md"))
	if !strings.Contains(string(cContent), "[[sub/A]]") {
		t.Errorf("C.md should contain [[sub/A]], got: %s", string(cContent))
	}
	dContent, _ := os.ReadFile(filepath.Join(vault, "D.md"))
	if !strings.Contains(string(dContent), "sub/A.md") {
		t.Errorf("D.md should contain sub/A.md, got: %s", string(dContent))
	}
}

// --- Test 19: already-moved stale file → error ---
// --- Test 22: rename to root target → vault-relative rewrite [[X]] ---
func TestMove_RenameToRoot(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "X.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// [[A]] → [[X]] (vault-relative, root target)
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			found = true
			if rw.NewLink != "[[X]]" {
				t.Errorf("B.md rewrite = %q, want [[X]]", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("B.md [[A]] should be rewritten")
	}
}

// --- Test 23: move root file out → Phase 2 rewrites incoming links ---
func TestMove_RootFileMovedOut(t *testing.T) {
	// A.md(root) + sub/A.md. B.md has [[A]].
	// Move A.md → sub2/A.md → root file gone.
	// Phase 2 rewrites B.md's [[A]] to [[sub2/A]] (incoming link to moved file).
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub2/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// B.md's [[A]] should be rewritten to [[sub2/A]].
	var bRewritten bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			bRewritten = true
			if rw.NewLink != "[[sub2/A]]" {
				t.Errorf("B.md rewrite = %q, want [[sub2/A]]", rw.NewLink)
			}
		}
	}
	if !bRewritten {
		t.Error("B.md [[A]] should be rewritten when root file moved out")
	}
}

// --- Test 23b: Phase 2.5 collateral — new root file changes resolution ---
func TestMove_RootFileMovedOutThirdParty(t *testing.T) {
	// sub/A.md exists (unique A). B.md has [[A]] pointing to sub/A.md.
	// C.md exists. Move C.md → A.md (root).
	// Pre-move: no root A. Post-move: root A.md (former C.md) + sub/A.md.
	// B.md's [[A]] (pointing to sub/A.md) should be collateral-rewritten to [[sub/A]].
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "C.md", To: "A.md"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// B.md's [[A]] should be collateral-rewritten to [[sub/A]].
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			found = true
			if rw.NewLink != "[[sub/A]]" {
				t.Errorf("expected [[sub/A]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("B.md [[A]] should be collateral-rewritten to [[sub/A]]")
	}

	bContent, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(bContent), "[[sub/A]]") {
		t.Errorf("B.md disk should contain [[sub/A]], got: %s", string(bContent))
	}

	edges := queryEdges(t, dbPath(vault), "B.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub/A]]" && e.targetName == "A" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub/A]]")
	}
}

// --- Test 24: root file survives move of other file → success ---
func TestMove_RootFileSurvives(t *testing.T) {
	// A.md(root) + sub/A.md. B.md has [[A]]. D.md exists.
	// Move D.md → sub2/A.md → root A.md still exists → [[A]] resolves → success.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "D.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Move(vault, MoveOptions{From: "D.md", To: "sub2/A.md"})
	if err != nil {
		t.Fatalf("expected success (root A.md survives), got: %v", err)
	}
}

// --- Test 25: meaning change — new root file → collateral rewrite ---
func TestMove_MeaningChangeNewRoot(t *testing.T) {
	// sub/A.md is unique A. B.md has [[A]]. C.md exists.
	// Move C.md → A.md → now root A.md exists, but pre-move root had no A.
	// → B.md's [[A]] (pointing to sub/A.md) is collateral-rewritten to [[sub/A]].
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "C.md", To: "A.md"})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// B.md's [[A]] should be collateral-rewritten to [[sub/A]].
	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			found = true
			if rw.NewLink != "[[sub/A]]" {
				t.Errorf("expected [[sub/A]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("B.md [[A]] should be collateral-rewritten to [[sub/A]]")
	}

	bContent, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(bContent), "[[sub/A]]") {
		t.Errorf("B.md disk should contain [[sub/A]], got: %s", string(bContent))
	}

	edges := queryEdges(t, dbPath(vault), "B.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub/A]]" && e.targetName == "A" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub/A]]")
	}
}

// --- Test 26: Phase 2 — basename unchanged + ambiguous + root survives → no rewrite ---
func TestMove_Phase2RootSkipsRewrite(t *testing.T) {
	// A.md(root) + sub/A.md. B.md has path link to A.md.
	// Move sub/A.md → sub2/A.md → basename unchanged, ambiguous.
	// Root A.md survives before AND after → incoming basename links don't need rewrite.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Move(vault, MoveOptions{From: "sub/A.md", To: "sub2/A.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// B.md's [[A]] should NOT be rewritten (root A.md is the target, not moved).
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[A]]" {
			t.Errorf("[[A]] in B.md should NOT be rewritten (root priority), but got %s", rw.NewLink)
		}
	}
}

// --- Test 27: collateral + incoming rewrite in same file ---
func TestMove_CollateralAndIncomingSameFile(t *testing.T) {
	// X.md has [[A]] (→ A.md) and [[B]] (→ sub1/B.md).
	// Move A.md → sub2/B.md.
	// [[A]] → incoming rewrite → [[sub2/B]]
	// [[B]] → collateral rewrite → [[sub1/B]]
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "X.md"), []byte("[[A]]\n[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub2/B.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// Check rewrites in X.md.
	var incomingFound, collateralFound bool
	for _, rw := range result.Rewritten {
		if rw.File == "X.md" && rw.OldLink == "[[A]]" {
			incomingFound = true
			if rw.NewLink != "[[sub2/B]]" {
				t.Errorf("incoming rewrite: expected [[sub2/B]], got %s", rw.NewLink)
			}
		}
		if rw.File == "X.md" && rw.OldLink == "[[B]]" {
			collateralFound = true
			if rw.NewLink != "[[sub1/B]]" {
				t.Errorf("collateral rewrite: expected [[sub1/B]], got %s", rw.NewLink)
			}
		}
	}
	if !incomingFound {
		t.Error("X.md [[A]] should be rewritten (incoming)")
	}
	if !collateralFound {
		t.Error("X.md [[B]] should be rewritten (collateral)")
	}

	// Verify disk.
	xContent, err := os.ReadFile(filepath.Join(vault, "X.md"))
	if err != nil {
		t.Fatalf("read X.md: %v", err)
	}
	if !strings.Contains(string(xContent), "[[sub2/B]]") {
		t.Errorf("X.md disk should contain [[sub2/B]], got: %s", string(xContent))
	}
	if !strings.Contains(string(xContent), "[[sub1/B]]") {
		t.Errorf("X.md disk should contain [[sub1/B]], got: %s", string(xContent))
	}

	// Verify DB edges.
	edges := queryEdges(t, dbPath(vault), "X.md")
	var incomingEdge, collateralEdge bool
	for _, e := range edges {
		if e.rawLink == "[[sub2/B]]" {
			incomingEdge = true
		}
		if e.rawLink == "[[sub1/B]]" {
			collateralEdge = true
		}
	}
	if !incomingEdge {
		t.Error("DB should have edge with raw_link [[sub2/B]]")
	}
	if !collateralEdge {
		t.Error("DB should have edge with raw_link [[sub1/B]]")
	}
}

// --- Test 28: collateral rewrite in multiple files ---
func TestMove_CollateralMultipleFiles(t *testing.T) {
	// X.md and Y.md both have [[B]] (→ sub1/B.md).
	// Move A.md → sub2/B.md → basename "B" becomes ambiguous.
	// Both X.md and Y.md should have [[B]] → [[sub1/B]] collateral rewrite.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "X.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Y.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub2/B.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	var xFound, yFound bool
	for _, rw := range result.Rewritten {
		if rw.File == "X.md" && rw.OldLink == "[[B]]" {
			xFound = true
			if rw.NewLink != "[[sub1/B]]" {
				t.Errorf("X.md: expected [[sub1/B]], got %s", rw.NewLink)
			}
		}
		if rw.File == "Y.md" && rw.OldLink == "[[B]]" {
			yFound = true
			if rw.NewLink != "[[sub1/B]]" {
				t.Errorf("Y.md: expected [[sub1/B]], got %s", rw.NewLink)
			}
		}
	}
	if !xFound {
		t.Error("X.md [[B]] should be collateral-rewritten")
	}
	if !yFound {
		t.Error("Y.md [[B]] should be collateral-rewritten")
	}

	// Verify disk.
	xContent, _ := os.ReadFile(filepath.Join(vault, "X.md"))
	if !strings.Contains(string(xContent), "[[sub1/B]]") {
		t.Errorf("X.md disk should contain [[sub1/B]], got: %s", string(xContent))
	}
	yContent, _ := os.ReadFile(filepath.Join(vault, "Y.md"))
	if !strings.Contains(string(yContent), "[[sub1/B]]") {
		t.Errorf("Y.md disk should contain [[sub1/B]], got: %s", string(yContent))
	}

	// Verify DB edges.
	xEdges := queryEdges(t, dbPath(vault), "X.md")
	var xEdgeFound bool
	for _, e := range xEdges {
		if e.rawLink == "[[sub1/B]]" {
			xEdgeFound = true
		}
	}
	if !xEdgeFound {
		t.Error("X.md DB should have edge with raw_link [[sub1/B]]")
	}
	yEdges := queryEdges(t, dbPath(vault), "Y.md")
	var yEdgeFound bool
	for _, e := range yEdges {
		if e.rawLink == "[[sub1/B]]" {
			yEdgeFound = true
		}
	}
	if !yEdgeFound {
		t.Error("Y.md DB should have edge with raw_link [[sub1/B]]")
	}
}

// --- Test 29: outgoing basename disambiguation ---
func TestMove_OutgoingBasenameDisambiguation(t *testing.T) {
	// A.md has [[B]] (→ sub1/B.md).
	// Move A.md → sub2/B.md → [[B]] becomes ambiguous → outgoing rewrite to [[sub1/B]].
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "sub2/B.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "sub2/B.md" && rw.OldLink == "[[B]]" {
			found = true
			if rw.NewLink != "[[sub1/B]]" {
				t.Errorf("expected [[sub1/B]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("outgoing [[B]] should be rewritten to [[sub1/B]]")
	}

	content, err := os.ReadFile(filepath.Join(vault, "sub2", "B.md"))
	if err != nil {
		t.Fatalf("read sub2/B.md: %v", err)
	}
	if !strings.Contains(string(content), "[[sub1/B]]") {
		t.Errorf("disk should contain [[sub1/B]], got: %s", string(content))
	}

	edges := queryEdges(t, dbPath(vault), "sub2/B.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub1/B]]" && e.targetName == "B" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub1/B]]")
	}
}

// --- Test 30: outgoing meaning change via root priority ---
func TestMove_OutgoingMeaningChangeRoot(t *testing.T) {
	// sub/B.md exists (unique B). A.md has [[B]] (→ sub/B.md).
	// Move A.md → B.md (root). Post-move: [[B]] would resolve to root B.md (self)
	// via root priority → meaning change → outgoing rewrite to [[sub/B]].
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub", "B.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "A.md", To: "B.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "B.md" && rw.OldLink == "[[B]]" {
			found = true
			if rw.NewLink != "[[sub/B]]" {
				t.Errorf("expected [[sub/B]], got %s", rw.NewLink)
			}
		}
	}
	if !found {
		t.Error("outgoing [[B]] should be rewritten to [[sub/B]]")
	}

	content, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(content), "[[sub/B]]") {
		t.Errorf("disk should contain [[sub/B]], got: %s", string(content))
	}

	edges := queryEdges(t, dbPath(vault), "B.md")
	var edgeFound bool
	for _, e := range edges {
		if e.rawLink == "[[sub/B]]" && e.targetName == "B" {
			edgeFound = true
		}
	}
	if !edgeFound {
		t.Error("DB should have edge with raw_link [[sub/B]]")
	}
}

// --- Test 31: collateral rewrite succeeds despite stale source file ---
func TestMove_CollateralRewriteWithStaleFile(t *testing.T) {
	// sub1/A.md exists. B.md has [[A]] (→ sub1/A.md). C.md exists.
	// Build, then modify B.md to make it stale.
	// Move C.md → sub2/A.md → collateral rewrite needed for B.md.
	// Should succeed despite B.md being stale.
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "sub1", "A.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "C.md"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(vault, "B.md"), []byte("[[A]]\nmodified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Move(vault, MoveOptions{From: "C.md", To: "sub2/A.md"})
	if err != nil {
		t.Fatalf("expected success despite stale B.md, got: %v", err)
	}

	// Verify B.md was rewritten — [[A]] should become path link to sub1/A.md.
	content, err := os.ReadFile(filepath.Join(vault, "B.md"))
	if err != nil {
		t.Fatalf("read B.md: %v", err)
	}
	if !strings.Contains(string(content), "sub1/A") {
		t.Errorf("B.md should have collateral rewrite to disambiguate, got: %s", string(content))
	}
}

// ===============================================
// MoveDir tests
// ===============================================

// --- Frontmatter wikilink rewriting ---
//
// A.md frontmatter (vault_build_frontmatter_wikilink):
//
//	related: "[[B]]"
//	parent: [[B]]
//	seealso:
//	  - "[[Sub/C]]"
//	  - "[[Sub/D|Display]]"
//	heading_ref: "[[B#Heading]]"
//	phantom_ref: "[[Ghost]]"
//
// Renaming B.md → NewB.md exercises quoted/bare style preservation and subpath
// preservation in a single pass.
func TestMove_FrontmatterWikilink_BasenameChange(t *testing.T) {
	vault := copyVault(t, "vault_build_frontmatter_wikilink")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "B.md", To: "NewB.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	// Edge rawLinks store the inner [[...]] form (parser strips YAML quoting).
	// Two edges share rawLink "[[B]]" (one quoted, one bare); both report the
	// same OldLink/NewLink pair. We assert at least one such rewrite plus the
	// subpath rewrite, then validate disk content for quoted/bare preservation.
	var sawB, sawBHeading int
	for _, rw := range result.Rewritten {
		if rw.File != "A.md" {
			continue
		}
		switch rw.OldLink {
		case "[[B]]":
			if rw.NewLink != "[[NewB]]" {
				t.Errorf("[[B]] rewrite: got %q, want [[NewB]]", rw.NewLink)
			}
			sawB++
		case "[[B#Heading]]":
			if rw.NewLink != "[[NewB#Heading]]" {
				t.Errorf("[[B#Heading]] rewrite: got %q, want [[NewB#Heading]]", rw.NewLink)
			}
			sawBHeading++
		}
	}
	if sawB == 0 {
		t.Errorf("A.md should rewrite [[B]] (frontmatter wikilink), rewrites: %+v", result.Rewritten)
	}
	if sawBHeading == 0 {
		t.Errorf("A.md should rewrite [[B#Heading]] (frontmatter wikilink with subpath), rewrites: %+v", result.Rewritten)
	}

	// Disk content: quoted style stays quoted, bare style stays bare.
	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	got := string(content)
	wantLines := []string{
		`related: "[[NewB]]"`,
		`parent: [[NewB]]`,
		`heading_ref: "[[NewB#Heading]]"`,
	}
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Errorf("A.md should contain %q after move, got:\n%s", want, got)
		}
	}

	// Untouched frontmatter wikilinks remain (Sub/C, Sub/D, Ghost).
	wantUnchanged := []string{
		`"[[Sub/C]]"`,
		`"[[Sub/D|Display]]"`,
		`"[[Ghost]]"`,
	}
	for _, want := range wantUnchanged {
		if !strings.Contains(got, want) {
			t.Errorf("A.md should still contain %q, got:\n%s", want, got)
		}
	}

	// DB edges must reflect the rewritten rawLinks (not stale).
	edges := queryEdges(t, dbPath(vault), "A.md")
	wantEdgeRaw := map[string]bool{
		"[[NewB]]":         true,
		"[[NewB#Heading]]": true,
	}
	for _, e := range edges {
		if e.linkType != LinkTypeFrontmatterWikilink {
			continue
		}
		if e.rawLink == "[[B]]" || e.rawLink == "[[B#Heading]]" {
			t.Errorf("DB edge still has stale rawLink %q after move", e.rawLink)
		}
		delete(wantEdgeRaw, e.rawLink)
	}
	for raw := range wantEdgeRaw {
		t.Errorf("DB should contain frontmatter_wikilink edge with rawLink %q", raw)
	}
}

// Renaming Sub/D.md → Sub/NewD.md exercises alias preservation in
// frontmatter wikilink rewriting.
func TestMove_FrontmatterWikilink_AliasPreserved(t *testing.T) {
	vault := copyVault(t, "vault_build_frontmatter_wikilink")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "Sub/D.md", To: "Sub/NewD.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	var found bool
	for _, rw := range result.Rewritten {
		if rw.File == "A.md" && rw.OldLink == "[[Sub/D|Display]]" {
			found = true
			if rw.NewLink != "[[Sub/NewD|Display]]" {
				t.Errorf("alias preservation failed: got %q", rw.NewLink)
			}
		}
	}
	if !found {
		t.Errorf("A.md should rewrite [[Sub/D|Display]] frontmatter wikilink, rewrites: %+v", result.Rewritten)
	}

	// Disk: alias preserved.
	content, err := os.ReadFile(filepath.Join(vault, "A.md"))
	if err != nil {
		t.Fatalf("read A.md: %v", err)
	}
	if !strings.Contains(string(content), `"[[Sub/NewD|Display]]"`) {
		t.Errorf("A.md should contain quoted alias link, got:\n%s", content)
	}
}

// Moving a note that itself contains a relative frontmatter wikilink must
// rewrite that link from the old location's perspective to the new location's.
// Without this rewrite the frontmatter relative link silently breaks: the disk
// file keeps the stale path and the DB edge becomes a phantom.
func TestMove_FrontmatterWikilink_RelativeLinkInMovedNote(t *testing.T) {
	vault := copyVault(t, "vault_build_frontmatter_wikilink")
	if err := os.MkdirAll(filepath.Join(vault, "old"), 0o755); err != nil {
		t.Fatalf("mkdir old: %v", err)
	}
	relA := "---\nrel: \"[[./Target]]\"\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(vault, "old", "RelA.md"), []byte(relA), 0o644); err != nil {
		t.Fatalf("write old/RelA.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "old", "Target.md"), []byte("# Target\n"), 0o644); err != nil {
		t.Fatalf("write old/Target.md: %v", err)
	}

	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := Move(vault, MoveOptions{From: "old/RelA.md", To: "new/RelA.md"})
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	var sawRewrite bool
	for _, rw := range result.Rewritten {
		if rw.File == "new/RelA.md" && rw.OldLink == "[[./Target]]" {
			sawRewrite = true
			if rw.NewLink != "[[../old/Target]]" {
				t.Errorf("relative frontmatter wikilink rewrite: got %q, want [[../old/Target]]", rw.NewLink)
			}
		}
	}
	if !sawRewrite {
		t.Errorf("moved note should rewrite [[./Target]] frontmatter wikilink, rewrites: %+v", result.Rewritten)
	}

	content, err := os.ReadFile(filepath.Join(vault, "new", "RelA.md"))
	if err != nil {
		t.Fatalf("read new/RelA.md: %v", err)
	}
	if !strings.Contains(string(content), `rel: "[[../old/Target]]"`) {
		t.Errorf("new/RelA.md should contain rewritten relative link, got:\n%s", content)
	}
	if strings.Contains(string(content), `[[./Target]]`) {
		t.Errorf("new/RelA.md must not retain stale link [[./Target]], got:\n%s", content)
	}

	edges := queryEdges(t, dbPath(vault), "new/RelA.md")
	var edgePointsToTarget bool
	for _, e := range edges {
		if e.linkType != LinkTypeFrontmatterWikilink {
			continue
		}
		if e.rawLink == "[[./Target]]" {
			t.Errorf("DB edge still has stale rawLink %q after move", e.rawLink)
		}
		if e.rawLink == "[[../old/Target]]" {
			edgePointsToTarget = true
		}
	}
	if !edgePointsToTarget {
		t.Errorf("DB should contain frontmatter_wikilink edge with rawLink [[../old/Target]] from new/RelA.md, got edges: %+v", edges)
	}
}
