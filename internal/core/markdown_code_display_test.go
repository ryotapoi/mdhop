package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMarkdownCodeDisplayRaw(t *testing.T) {
	for _, tt := range []struct {
		raw, definition string
		typ             LinkType
	}{
		{"[読む `](fake)`](B.md#Heading)", "", LinkTypeMarkdown},
		{"[読む `](fake)`][guide]", "[guide]: B.md#Heading\n", LinkTypeMarkdownReference},
		{"[読む `code`][]", "[読む `code`]: B.md#Heading\n", LinkTypeMarkdownReference},
		{"[読む `code`]", "[読む `code`]: B.md#Heading\n", LinkTypeMarkdownReference},
	} {
		t.Run(tt.raw, func(t *testing.T) {
			content := tt.definition + "前 ` [ignored](D.md) ` " + tt.raw + " [other](C.md)\n~~~\n[ignored](D.md)\n~~~\n"
			links := parseLinks(content).Links
			line := 1
			if tt.definition != "" {
				line = 2
			}
			if len(links) != 2 || links[0].rawLink != tt.raw || links[0].linkType != tt.typ || links[0].target != "B" || links[0].subpath != "#Heading" || links[0].lineStart != line || links[0].lineEnd != line || links[1].rawLink != "[other](C.md)" {
				t.Fatalf("links=%+v", links)
			}
			if tt.typ == LinkTypeMarkdown {
				direct := parseMarkdownLinks("前 `[ignored](D.md)` "+tt.raw+" [other](C.md)", 9)
				if len(direct) != 2 || direct[0].rawLink != tt.raw || direct[0].lineStart != 9 {
					t.Fatalf("direct links=%+v", direct)
				}
			}
		})
	}
}

func TestMarkdownCodeDisplayParts(t *testing.T) {
	raw := "[読む `](fake)`]( ./B.md#Heading )"
	text, url := extractMarkdownParts(raw)
	if text != "読む `](fake)`" || url != " ./B.md#Heading " {
		t.Fatalf("parts=(%q,%q)", text, url)
	}
	parts, ok := parseRelativeLink(raw, LinkTypeMarkdown)
	if !ok || parts.prefix != "[読む `](fake)`]( " || parts.target != "./B.md" || parts.suffix != "#Heading )" {
		t.Fatalf("relative=%+v ok=%v", parts, ok)
	}
	if got := rewriteRawLink(raw, LinkTypeMarkdown, "C.md"); got != "[読む `](fake)`](/C.md#Heading)" {
		t.Fatalf("rewrite=%q", got)
	}
}

func TestMarkdownCodeDisplayMoveAndResolve(t *testing.T) {
	raw := "[読む `](fake)`](B.md#Heading)"
	protected := "`[ignored](B.md)`\n~~~\n[ignored](B.md)\n~~~\n"
	content := "# A\n前 " + raw + " [other `code`](B.md)\n" + protected
	v := newMoveVault(t, map[string]string{"A.md": content, "B.md": "# Heading\n"})
	r, err := ResolveAtLine(v, "A.md", raw, 2)
	if err != nil || r.Path != "B.md" || r.Subpath != "#Heading" {
		t.Fatalf("resolve=%+v err=%v", r, err)
	}
	if _, err := ResolveAtLine(v, "A.md", raw, 1); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("wrong line=%v", err)
	}
	if _, err := Move(v, MoveOptions{From: "B.md", To: "C.md"}); err != nil {
		t.Fatal(err)
	}
	wantRaw := "[読む `](fake)`](/C.md#Heading)"
	want := "# A\n前 " + wantRaw + " [other `code`](/C.md)\n" + protected
	if got := readVaultFile(t, v, "A.md"); got != want {
		t.Fatalf("body=%q want=%q", got, want)
	}
	edges := queryEdges(t, dbPath(v), "A.md")
	if len(edges) != 2 {
		t.Fatalf("edges=%+v", edges)
	}
	for _, raw := range []string{wantRaw, "[other `code`](/C.md)"} {
		r, err := ResolveAtLine(v, "A.md", raw, 2)
		if err != nil || r.Path != "C.md" {
			t.Fatalf("moved resolve=%+v err=%v", r, err)
		}
	}
	if _, err := Move(v, MoveOptions{From: "A.md", To: "sub/A.md"}); err != nil {
		t.Fatal(err)
	}
	if got := readVaultFile(t, v, "sub/A.md"); got != want {
		t.Fatalf("source moved body=%q", got)
	}
}

func TestMarkdownCodeDisplayReferenceGuard(t *testing.T) {
	raw := "[読む `](fake)`][guide]"
	content := "# A\n" + raw + "\n[guide]: B.md#Heading\n"
	v := referenceVault(t, map[string]string{"A.md": content, "B.md": "# Heading\n"})
	r, err := ResolveAtLine(v, "A.md", raw, 2)
	if err != nil || r.Path != "B.md" || r.Subpath != "#Heading" {
		t.Fatalf("reference=%+v err=%v", r, err)
	}
	before, err := os.ReadFile(dbPath(v))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Move(v, MoveOptions{From: "B.md", To: "C.md"}); err == nil {
		t.Fatal("reference move accepted")
	}
	after, err := os.ReadFile(dbPath(v))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("DB changed: %v", err)
	}
	if got := readVaultFile(t, v, "A.md"); got != content {
		t.Fatalf("source changed=%q", got)
	}
	if _, err := os.Stat(filepath.Join(v, "B.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(v, "C.md")); !os.IsNotExist(err) {
		t.Fatalf("target created: %v", err)
	}
}

func TestBuildReplacesVersionSevenMaskedMarkdown(t *testing.T) {
	raw := "[読む `code`](B.md)"
	v := referenceVault(t, map[string]string{"A.md": raw + "\n", "B.md": ""})
	db := openTestDB(t, dbPath(v))
	for _, stmt := range []string{
		"UPDATE edges SET raw_link='[読む       ](B.md)'",
		"UPDATE build_links SET link=json_set(link,'$.Raw','[読む       ](B.md)')",
		"PRAGMA user_version=7",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	if _, err := ResolveAtLine(v, "A.md", raw, 1); err == nil {
		t.Fatal("legacy read accepted")
	}
	if _, err := Update(v, UpdateOptions{Files: []string{"A.md"}}); err == nil {
		t.Fatal("legacy partial update accepted")
	}
	if _, err := Move(v, MoveOptions{From: "B.md", To: "C.md"}); err == nil {
		t.Fatal("legacy move accepted")
	}
	assertBuildMatchesFull(t, v)
	if r, err := ResolveAtLine(v, "A.md", raw, 1); err != nil || r.Path != "B.md" {
		t.Fatalf("rebuilt=%+v err=%v", r, err)
	}
	db = openTestDB(t, dbPath(v))
	defer db.Close()
	var saved string
	if err := db.QueryRow("SELECT json_extract(link,'$.Raw') FROM build_links WHERE source_path='A.md'").Scan(&saved); err != nil || saved != raw {
		t.Fatalf("cached raw=%q err=%v", saved, err)
	}
}
