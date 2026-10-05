package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMarkdownDestinationInterpretation(t *testing.T) {
	tests := []struct {
		raw, target, fragment string
		external              bool
	}{
		{`a\(b\).md`, "a(b).md", "", false},
		{`a\).md`, "a).md", "", false},
		{"A&amp;B.md", "A&B.md", "", false},
		{"A&#38;B&#x21;.md", "A&B!.md", "", false},
		{`\&amp;.md`, "&amp;.md", "", false},
		{"&amp;amp;.md", "&amp;.md", "", false},
		{"&#92;&amp;.md", `\&.md`, "", false},
		{"&notit;&amp.md", "&notit;&amp.md", "", false},
		{"A%20B.md", "A B.md", "", false},
		{"A%2520B.md", "A%20B.md", "", false},
		{"A%23B.md#H%23I", "A#B.md", "#H#I", false},
		{"A&#35;B", "A", "#B", false},
		{"&semi;&CounterClockwiseContourIntegral;&fjlig;", ";∳fj", "", false},
		{"&#0;&#xD800;&#x110000;", "���", "", false},
		{"A+B%2B%Q0%.md", "A+B+%Q0%.md", "", false},
		{"dir%2FA.md", "dir/A.md", "", false},
		{"%2E%2E%2FA.md", "../A.md", "", false},
		{"https&#58;//example.com", "", "", true},
		{"https%3A%2F%2Fexample.com", "https://example.com", "", false},
		{`a\z.md`, `a\z.md`, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			target, fragment, external := markdownDestination(tt.raw)
			if target != tt.target || fragment != tt.fragment || external != tt.external {
				t.Fatalf("got %q %q %v, want %q %q %v", target, fragment, external, tt.target, tt.fragment, tt.external)
			}
		})
	}
	for _, raw := range []string{`[shown](a\).md)`, `![shown](a\(b\).md)`, "[shown](A&amp;B.md)", "[shown][id]\n[id]: A%2520B.md", `[shown][id]` + "\n" + `[id]: a\).md`, "[shown][id]\n[id]: <a\\>b.md>"} {
		links := parseLinks(raw).Links
		if len(links) != 1 {
			t.Fatalf("raw %q: links %+v", raw, links)
		}
	}
	for _, target := range []string{`A%20B.md`, "A#B(&).md", `dir/a\b.md`, "https://internal.md"} {
		raw := encodeMarkdownDestination(target, "#H#&(%20)")
		got, fragment, external := markdownDestination(raw)
		if got != target || fragment != "#H#&(%20)" || external {
			t.Fatalf("round trip %q: %q %q %v", raw, got, fragment, external)
		}
	}
	wiki := parseLinks("[[A%20B&amp;]]").Links[0]
	if wiki.target != "A%20B&amp;" {
		t.Fatal(wiki)
	}
	raw, ok := frontmatterPathOccur("A%20B&amp;.md", 1)
	if !ok || raw.target != "A%20B&amp;" {
		t.Fatal(raw, ok)
	}
}

func TestTableWikilinkContext(t *testing.T) {
	tests := []struct {
		name, content string
		tableLines    []int
	}{
		{"header and body", "| [[X\\|shown]] | next |\n| :--- | ---: |\n| [[X\\|shown]] | cell |", []int{1, 3}},
		{"one column bare delimiter", "| [[X\\|shown]] |\n---\n[[X\\|shown]]", []int{1, 3}},
		{"optional outer pipes", "[[X\\|shown]] | next\n:--- | ---:\n[[X\\|shown]] | cell", []int{1, 3}},
		{"autolink header and body", "<https://example.com> | [[X\\|shown]]\n--- | ---\n<https://example.com> | [[X\\|shown]]", []int{1, 3}},
		{"autolink body", "URL | Note\n--- | ---\n<https://example.com> | [[X\\|shown]]", []int{3}},
		{"email autolink", "<user@example.com> | [[X\\|shown]]\n--- | ---\n<user@example.com> | [[X\\|shown]]", []int{1, 3}},
		{"pipe leading HTML cells", "| <div> | [[X\\|shown]] |\n| --- | --- |\n| <div> | [[X\\|shown]] |", []int{1, 3}},
		{"inline HTML cells", "<span>text</span> | [[X\\|shown]]\n--- | ---\n<span>text</span> | [[X\\|shown]]", []int{1, 3}},
		{"mismatched cells", "[[X\\|shown]] | next\n--- | --- | ---\n[[X\\|shown]]", nil},
		{"invalid delimiter", "[[X\\|shown]] | next\n--- | :---::\n[[X\\|shown]]", nil},
		{"blank termination", "a | b\n--- | ---\n[[X\\|shown]] | c\n\n[[X\\|shown]]", []int{3}},
		{"block termination", "a | b\n--- | ---\n# Heading\n[[X\\|shown]]", nil},
		{"HTML block termination", "a | b\n--- | ---\n<DIV class=\"x\">\n[[X\\|shown]] | cell", nil},
		{"HTML block header", "<div> | [[X\\|shown]]\n--- | ---\n[[X\\|shown]] | cell", nil},
		{"HTML custom tag termination", "a | b\n--- | ---\n<custom-tag title=\"a > b\">\n[[X\\|shown]] | cell", nil},
		{"HTML comment termination", "a | b\n--- | ---\n<!-- comment -->\n[[X\\|shown]] | cell", nil},
		{"fence and inline code", "```\na | b\n--- | ---\n[[X\\|shown]] | c\n```\n`a | b`\n--- | ---\n`[[X\\|shown]]`", nil},
		{"frontmatter", "---\nref: '[[X\\|shown]]'\n---\na | b\n--- | ---\n[[X\\|shown]] | c", []int{6}},
		{"outside", "[[X\\|shown]] | prose", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tableLines []int
			for _, link := range parseLinks(tt.content).Links {
				if link.inTable {
					tableLines = append(tableLines, link.lineStart)
					if link.target != "X" {
						t.Fatal(link)
					}
				} else if link.target != `X\` {
					t.Fatal(link)
				}
				if link.rawLink != `[[X\|shown]]` || link.lineStart != link.lineEnd {
					t.Fatal(link)
				}
			}
			if !reflect.DeepEqual(tableLines, tt.tableLines) {
				t.Fatalf("table lines %v, want %v", tableLines, tt.tableLines)
			}
		})
	}
}

func TestTableHTMLBlockBoundaries(t *testing.T) {
	for _, start := range []string{
		"<script>", "<pre class=x", "<style", "<!--", "<?instruction",
		"<!DOCTYPE html>", "<![CDATA[", "</DIV>", "<div", "<div/>",
		"<custom-tag enabled value='x'>", "</custom-tag>",
	} {
		t.Run(start, func(t *testing.T) {
			links := parseLinks("a | b\n--- | ---\n" + start + "\n[[X\\|shown]] | cell").Links
			if len(links) != 1 || links[0].inTable || links[0].target != `X\` {
				t.Fatalf("HTML block should end table: %+v", links)
			}
		})
	}
	for _, cell := range []string{"<scripture>", "<script/>", "<!doctype html>", "<divine>text", "<span title='x'>text</span>", "<custom-tag invalid=>", "<https://example.com>", "<user@example.com>"} {
		t.Run(cell, func(t *testing.T) {
			links := parseLinks("a | b\n--- | ---\n" + cell + " | [[X\\|shown]]").Links
			if len(links) != 1 || !links[0].inTable || links[0].target != "X" {
				t.Fatalf("inline cell should remain in table: %+v", links)
			}
		})
	}
}

func writeInterpretationFiles(t *testing.T, vault string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(vault, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLinkInterpretationGraphAndUpdate(t *testing.T) {
	for _, exists := range []bool{false, true} {
		t.Run(map[bool]string{false: "phantoms", true: "existing"}[exists], func(t *testing.T) {
			vault := t.TempDir()
			content := "[paren](a\\(b\\).md)\n[amp](A&amp;B.md)\n[space](A%20B.md)\n![percent](A%2520B.md)\n[reference][id]\n[id]: A%2520B.md\n| column |\n| --- |\n| [[X\\|shown]] |\n"
			files := map[string]string{"Source.md": content, "Peer.md": "[[A B]]\n[[A%20B]]\n[[X]]"}
			if exists {
				for _, name := range []string{"a(b).md", "A&B.md", "A B.md", "A%20B.md", "X.md"} {
					files[name] = "target"
				}
			}
			writeInterpretationFiles(t, vault, files)
			buildVault(t, vault)
			assertGraph := func() {
				t.Helper()
				for raw, target := range map[string]string{`[paren](a\(b\).md)`: "a(b)", "[amp](A&amp;B.md)": "A&B", "[space](A%20B.md)": "A B", "[percent](A%2520B.md)": "A%20B", "[reference][id]": "A%20B", `[[X\|shown]]`: "X"} {
					got, err := Resolve(vault, "Source.md", raw)
					if err != nil || got.Name != target || got.Exists != exists {
						t.Fatalf("resolve %q = %+v, %v", raw, got, err)
					}
				}
				query, err := Query(vault, EntrySpec{File: "Source.md"}, QueryOptions{Relations: []string{"outgoing", "twohop"}})
				if err != nil {
					t.Fatal(err)
				}
				if len(query.Outgoing) != 5 {
					t.Fatalf("outgoing %+v", query.Outgoing)
				}
				peer := false
				for _, target := range query.TwoHop {
					peer = peer || target.Path == "Peer.md"
				}
				if !peer {
					t.Fatalf("twohop %+v", query.TwoHop)
				}
				entry := EntrySpec{Phantom: "A%20B"}
				if exists {
					entry = EntrySpec{File: "A%20B.md"}
				}
				back, err := Query(vault, entry, QueryOptions{Relations: []string{"backlinks"}})
				if err != nil || len(back.Backlinks) != 2 {
					t.Fatalf("backlinks %+v, %v", back, err)
				}
			}
			assertGraph()
			writeInterpretationFiles(t, vault, map[string]string{"Source.md": content + "\nupdated"})
			if _, err := Update(vault, UpdateOptions{Files: []string{"Source.md"}}); err != nil {
				t.Fatal(err)
			}
			assertGraph()
		})
	}
}

func TestExactRawTableResolveSnapshotAndAmbiguity(t *testing.T) {
	vault := t.TempDir()
	raw := `[[X\|shown]]`
	content := "| column |\n| --- |\n| " + raw + " |\n| " + raw + " |\n"
	writeInterpretationFiles(t, vault, map[string]string{"Source.md": content, "X.md": "x"})
	buildVault(t, vault)
	writeInterpretationFiles(t, vault, map[string]string{"Source.md": raw})
	if got, err := Resolve(vault, "Source.md", raw); err != nil || got.Path != "X.md" {
		t.Fatalf("snapshot %+v %v", got, err)
	}
	writeInterpretationFiles(t, vault, map[string]string{"Source.md": content + "\n" + raw})
	if _, err := Update(vault, UpdateOptions{Files: []string{"Source.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(vault, "Source.md", raw); !errors.Is(err, ErrAmbiguousLink) {
		t.Fatalf("ambiguous error %v", err)
	}
}

func TestEncodedVaultEscapeAndOldIndexGate(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"Source.md": "[escape](%2E%2E%2Foutside.md)"})
	if _, err := Build(vault); err == nil || !strings.Contains(err.Error(), "escape") {
		t.Fatalf("escape error %v", err)
	}
	writeInterpretationFiles(t, vault, map[string]string{"Source.md": "[space](A%20B.md)", "A B.md": "a"})
	buildVault(t, vault)
	db := openTestDB(t, dbPath(vault))
	if _, err := db.Exec("PRAGMA user_version = 0"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	before, err := os.ReadFile(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []func() error{
		func() error { _, e := Resolve(vault, "Source.md", "[space](A%20B.md)"); return e },
		func() error { _, e := Update(vault, UpdateOptions{Files: []string{"Source.md"}}); return e },
		func() error { _, e := Move(vault, MoveOptions{From: "A B.md", To: "Changed.md"}); return e },
	} {
		if err := operation(); err == nil || !strings.Contains(err.Error(), "mdhop build") {
			t.Fatalf("gate %v", err)
		}
	}
	after, err := os.ReadFile(dbPath(vault))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("old index changed", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "A B.md")); err != nil {
		t.Fatal(err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(vault, ".mdhop", "*.tmp-*"))
	if len(leftovers) != 0 {
		t.Fatal(leftovers)
	}
	buildVault(t, vault)
	if got, err := Resolve(vault, "Source.md", "[space](A%20B.md)"); err != nil || got.Path != "A B.md" {
		t.Fatalf("rebuilt %+v %v", got, err)
	}
}

func TestInterpretedMutationRoundTrips(t *testing.T) {
	t.Run("incoming markdown and table aliases", func(t *testing.T) {
		vault := t.TempDir()
		source := "[space](A%20B.md#H%2520I)\n[percent](A%2520B.md)\n| column |\n| --- |\n| ![[X\\|shown]] |\n"
		writeInterpretationFiles(t, vault, map[string]string{"Source.md": source, "A B.md": "a", "A%20B.md": "b", "X.md": "x"})
		buildVault(t, vault)
		for _, move := range []MoveOptions{{From: "A B.md", To: "renamed/A#B(&).md"}, {From: "X.md", To: "renamed/Y.md"}} {
			if _, err := Move(vault, move); err != nil {
				t.Fatal(err)
			}
		}
		content, err := os.ReadFile(filepath.Join(vault, "Source.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), `![[renamed/Y\|shown]]`) {
			t.Fatalf("table alias %q", content)
		}
		var movedRaw string
		for _, l := range parseLinks(string(content)).Links {
			if l.linkType == LinkTypeMarkdown && l.target == "renamed/A#B(&)" {
				movedRaw = l.rawLink
				if l.subpath != "#H%20I" {
					t.Fatal(l)
				}
			}
		}
		if movedRaw == "" {
			t.Fatalf("new links %q", content)
		}
		for _, rebuild := range []bool{false, true} {
			if rebuild {
				buildVault(t, vault)
			}
			for raw, path := range map[string]string{movedRaw: "renamed/A#B(&).md", "[percent](A%2520B.md)": "A%20B.md", `[[renamed/Y\|shown]]`: "renamed/Y.md"} {
				if got, err := Resolve(vault, "Source.md", raw); err != nil || got.Path != path {
					t.Fatalf("%q: %+v %v", raw, got, err)
				}
			}
		}
	})
	t.Run("relative outgoing encoded path", func(t *testing.T) {
		vault := t.TempDir()
		writeInterpretationFiles(t, vault, map[string]string{"dir/Source.md": "![shown](../A%2520B.md#H%23I)", "A%20B.md": "a"})
		buildVault(t, vault)
		if _, err := Move(vault, MoveOptions{From: "dir/Source.md", To: "else/deep/Source.md"}); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(vault, "else/deep/Source.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "![shown](../../A%2520B.md#H%23I)" {
			t.Fatalf("relative %q", content)
		}
		buildVault(t, vault)
		if got, err := Resolve(vault, "else/deep/Source.md", "[shown](../../A%2520B.md#H%23I)"); err != nil || got.Path != "A%20B.md" || got.Subpath != "#H#I" {
			t.Fatalf("relative %+v %v", got, err)
		}
	})
	t.Run("table and reference phantom promotion", func(t *testing.T) {
		vault := t.TempDir()
		writeInterpretationFiles(t, vault, map[string]string{"Source.md": "| column |\n| --- |\n| [[X\\|shown]] |\n\n[ref][id]\n[id]: A%2520B.md\n"})
		buildVault(t, vault)
		writeInterpretationFiles(t, vault, map[string]string{"X.md": "x", "A%20B.md": "a"})
		if _, err := Add(vault, AddOptions{Files: []string{"X.md", "A%20B.md"}}); err != nil {
			t.Fatal(err)
		}
		for raw, path := range map[string]string{`[[X\|shown]]`: "X.md", "[ref][id]": "A%20B.md"} {
			if got, err := Resolve(vault, "Source.md", raw); err != nil || got.Path != path {
				t.Fatalf("promotion %+v %v", got, err)
			}
		}
		before, _ := os.ReadFile(dbPath(vault))
		if _, err := Move(vault, MoveOptions{From: "A%20B.md", To: "Other.md"}); err == nil || !strings.Contains(err.Error(), "cannot be rewritten") {
			t.Fatalf("reference guard %v", err)
		}
		after, _ := os.ReadFile(dbPath(vault))
		if !reflect.DeepEqual(before, after) {
			t.Fatal("guard changed DB")
		}
		if _, err := os.Stat(filepath.Join(vault, "A%20B.md")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unrepresentable wiki rewrite rejects before mutation", func(t *testing.T) {
		vault := t.TempDir()
		writeInterpretationFiles(t, vault, map[string]string{"Source.md": "[[X|shown]]", "X.md": "x"})
		buildVault(t, vault)
		before, _ := os.ReadFile(dbPath(vault))
		if _, err := Move(vault, MoveOptions{From: "X.md", To: "Y#Z.md"}); err == nil {
			t.Fatal("expected representability error")
		}
		after, _ := os.ReadFile(dbPath(vault))
		if !reflect.DeepEqual(before, after) {
			t.Fatal("rejection changed DB")
		}
		content, _ := os.ReadFile(filepath.Join(vault, "Source.md"))
		if string(content) != "[[X|shown]]" {
			t.Fatal(string(content))
		}
		if _, err := os.Stat(filepath.Join(vault, "X.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(vault, "Y#Z.md")); !os.IsNotExist(err) {
			t.Fatal("destination created")
		}
	})
}

func TestInterpretedConvertAndScanRewrites(t *testing.T) {
	t.Run("convert decodes skips and round trips table", func(t *testing.T) {
		vault := t.TempDir()
		original := "![shown](A%2520B.md#H%20I)\n[hash](A%23B.md)\n[pipe](A%7CB.md)\n| column |\n| --- |\n| [[X\\|shown]] |\n"
		writeInterpretationFiles(t, vault, map[string]string{"Source.md": original, "A%20B.md": "a", "A#B.md": "b", "A|B.md": "c", "X.md": "x"})
		if _, err := Convert(vault, ConvertOptions{ToFormat: "wikilink"}); err != nil {
			t.Fatal(err)
		}
		content, _ := os.ReadFile(filepath.Join(vault, "Source.md"))
		if !strings.Contains(string(content), "![[A%20B#H I|shown]]") || !strings.Contains(string(content), "[hash](A%23B.md)") || !strings.Contains(string(content), "[pipe](A%7CB.md)") {
			t.Fatalf("convert %q", content)
		}
		if _, err := Convert(vault, ConvertOptions{ToFormat: "markdown"}); err != nil {
			t.Fatal(err)
		}
		content, _ = os.ReadFile(filepath.Join(vault, "Source.md"))
		if !strings.Contains(string(content), "![shown](A%2520B.md#H%20I)") || !strings.Contains(string(content), "[shown](X.md)") {
			t.Fatalf("markdown %q", content)
		}
		if _, err := Convert(vault, ConvertOptions{ToFormat: "wikilink"}); err != nil {
			t.Fatal(err)
		}
		content, _ = os.ReadFile(filepath.Join(vault, "Source.md"))
		if !strings.Contains(string(content), `[[X\|shown]]`) {
			t.Fatalf("table convert %q", content)
		}
		buildVault(t, vault)
		if got, err := Resolve(vault, "Source.md", `[[X\|shown]]`); err != nil || got.Path != "X.md" {
			t.Fatalf("table %+v %v", got, err)
		}
	})
	t.Run("repair and simplify use decoded paths", func(t *testing.T) {
		vault := t.TempDir()
		writeInterpretationFiles(t, vault, map[string]string{"Source.md": "[broken](missing/A%2520B.md)\n[long](sub/A%2520B.md)", "sub/A%20B.md": "a"})
		if _, err := Repair(vault, RepairOptions{}); err != nil {
			t.Fatal(err)
		}
		if _, err := Simplify(vault, SimplifyOptions{}); err != nil {
			t.Fatal(err)
		}
		content, _ := os.ReadFile(filepath.Join(vault, "Source.md"))
		if string(content) != "[broken](A%2520B.md)\n[long](A%2520B.md)" {
			t.Fatalf("scan %q", content)
		}
		buildVault(t, vault)
		if got, err := Resolve(vault, "Source.md", "[broken](A%2520B.md)"); err != nil || got.Path != "sub/A%20B.md" {
			t.Fatalf("scan %+v %v", got, err)
		}
	})
}

func TestTableWikiConversionRepresentability(t *testing.T) {
	for _, tt := range []struct {
		name, raw, want string
		inTable         bool
	}{
		{"unsafe self alias in table", `[shown](#H%5C)`, `[shown](#H%5C)`, true},
		{"safe self alias in table", `[shown](#H)`, `[[#H\|shown]]`, true},
		{"even trailing backslashes in table", `[shown](#H%5C%5C)`, `[[#H\\\|shown]]`, true},
		{"self without alias in table", `[#H\](#H%5C)`, `[[#H\]]`, true},
		{"self alias outside table", `[shown](#H%5C)`, `[[#H\|shown]]`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertMarkdownToWikilink(tt.raw, tt.inTable); got != tt.want {
				t.Fatalf("conversion %q, want %q", got, tt.want)
			}
		})
	}
	raw := `[shown](X%5C.md)`
	if got := convertMarkdownToWikilink(raw, true); got != raw {
		t.Fatalf("unsafe table conversion %q", got)
	}
	if got := rewriteRawLink(`[[X\|shown]]`, LinkTypeWikilink, `Y\.md`, true); got != "" {
		t.Fatalf("unsafe table rewrite %q", got)
	}
	content := "| [shown](#H%5C) |\n| --- |\n"
	links := parseLinksForConvert(content).Links
	if len(links) != 1 || !links[0].inTable {
		t.Fatalf("self table context %+v", links)
	}
	_, destination := extractMarkdownParts(links[0].rawLink)
	if target, subpath, external := markdownDestination(destination); target != "" || subpath != `#H\` || external {
		t.Fatalf("self destination %q %q %v", target, subpath, external)
	}
}

func TestInterpretedAddCollateralDisambiguation(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"Source.md": "[shown](A%2520B.md)\n| column |\n| --- |\n| [[X\\|shown]] |\n", "old/A%20B.md": "a", "old/X.md": "x"})
	buildVault(t, vault)
	writeInterpretationFiles(t, vault, map[string]string{"new/A%20B.md": "b", "new/X.md": "y"})
	if _, err := Add(vault, AddOptions{Files: []string{"new/A%20B.md", "new/X.md"}, AutoDisambiguate: true}); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(vault, "Source.md"))
	if !strings.Contains(string(content), "[shown](old/A%2520B.md)") || !strings.Contains(string(content), `[[old/X\|shown]]`) {
		t.Fatalf("collateral %q", content)
	}
	buildVault(t, vault)
	if got, err := Resolve(vault, "Source.md", "[shown](old/A%2520B.md)"); err != nil || got.Path != "old/A%20B.md" {
		t.Fatalf("collateral %+v %v", got, err)
	}
	if got, err := Resolve(vault, "Source.md", `[[old/X\|shown]]`); err != nil || got.Path != "old/X.md" {
		t.Fatalf("collateral table %+v %v", got, err)
	}
}

func TestBacktickDestinationRewritePreservesRelations(t *testing.T) {
	t.Run("convert retains unrepresentable path and fragment", func(t *testing.T) {
		vault := t.TempDir()
		original := "[path](X%60Y.md)\n[fragment](Plain.md#H%60I)\n"
		writeInterpretationFiles(t, vault, map[string]string{"Source.md": original, "X`Y.md": "x", "Plain.md": "plain"})
		buildVault(t, vault)
		if _, err := Convert(vault, ConvertOptions{ToFormat: "wikilink"}); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(vault, "Source.md"))
		if err != nil || string(content) != original {
			t.Fatalf("conversion changed unsupported destinations: %q, %v", content, err)
		}
		buildVault(t, vault)
		for _, tt := range []struct{ raw, path, fragment string }{{"[path](X%60Y.md)", "X`Y.md", ""}, {"[fragment](Plain.md#H%60I)", "Plain.md", "#H`I"}} {
			got, err := Resolve(vault, "Source.md", tt.raw)
			if err != nil || got.Path != tt.path || got.Subpath != tt.fragment {
				t.Fatalf("convert/rebuild %q: %+v, %v", tt.raw, got, err)
			}
		}
	})
	t.Run("markdown move encodes path and fragment", func(t *testing.T) {
		vault := t.TempDir()
		writeInterpretationFiles(t, vault, map[string]string{"Source.md": "[shown](X%60Y.md#H%60I)", "X`Y.md": "x"})
		buildVault(t, vault)
		if _, err := Move(vault, MoveOptions{From: "X`Y.md", To: "Z`Q.md"}); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(vault, "Source.md"))
		raw := "[shown](Z%60Q.md#H%60I)"
		if err != nil || string(content) != raw {
			t.Fatalf("move output %q, %v", content, err)
		}
		buildVault(t, vault)
		got, err := Resolve(vault, "Source.md", raw)
		if err != nil || got.Path != "Z`Q.md" || got.Subpath != "#H`I" {
			t.Fatalf("move/rebuild %+v, %v", got, err)
		}
		outgoing, err := Query(vault, EntrySpec{File: "Source.md"}, QueryOptions{Relations: []string{"outgoing"}})
		if err != nil || len(outgoing.Outgoing) != 1 || outgoing.Outgoing[0].Path != "Z`Q.md" {
			t.Fatalf("outgoing %+v, %v", outgoing, err)
		}
	})
	t.Run("mandatory wiki rewrite rejects before mutation", func(t *testing.T) {
		vault := t.TempDir()
		writeInterpretationFiles(t, vault, map[string]string{"Source.md": "[[X|shown]]", "X.md": "x"})
		buildVault(t, vault)
		before, err := os.ReadFile(dbPath(vault))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Move(vault, MoveOptions{From: "X.md", To: "Z`Q.md"}); err == nil {
			t.Fatal("expected unrepresentable wiki rewrite rejection")
		}
		after, err := os.ReadFile(dbPath(vault))
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("rejection changed DB", err)
		}
		content, _ := os.ReadFile(filepath.Join(vault, "Source.md"))
		if string(content) != "[[X|shown]]" {
			t.Fatal(string(content))
		}
		if _, err := os.Stat(filepath.Join(vault, "X.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(vault, "Z`Q.md")); !os.IsNotExist(err) {
			t.Fatal("destination created")
		}
	})
}

func TestFrontmatterBacktickDestinationRewrite(t *testing.T) {
	for _, tt := range []struct {
		name, source, target, from, to, movedSource, oldRaw, raw, subpath string
	}{
		{"double quoted incoming", "---\nrelated: \"[[A]]\"\n---\nbody\n", "A.md", "A.md", "Z`Q.md", "Source.md", "[[A]]", "[[Z`Q]]", ""},
		{"single quoted target and subpath", "---\nrelated: '[[X`Y#H`I|shown]]'\n---\nbody\n", "X`Y.md", "X`Y.md", "Z`Q.md", "Source.md", "[[X`Y#H`I|shown]]", "[[Z`Q#H`I|shown]]", "#H`I"},
		{"moved relative outgoing", "---\nrelated: \"[[./X`Y#H`I|shown]]\"\n---\nbody\n", "X`Y.md", "Source.md", "sub/Source.md", "sub/Source.md", "[[./X`Y#H`I|shown]]", "[[../X`Y#H`I|shown]]", "#H`I"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			vault := newMoveVault(t, map[string]string{"Source.md": tt.source, tt.target: "target\n"})
			beforeDB, err := os.ReadFile(dbPath(vault))
			if err != nil {
				t.Fatal(err)
			}
			if result, err := PlanMoveTemplate(vault, MoveTemplateOptions{From: tt.from, Template: tt.to}); err != nil || len(result.Moved) != 1 {
				t.Fatalf("dry-run: %+v, %v", result, err)
			}
			if afterDB, err := os.ReadFile(dbPath(vault)); err != nil || !reflect.DeepEqual(beforeDB, afterDB) {
				t.Fatal("dry-run changed DB", err)
			}
			if got := readVaultFile(t, vault, "Source.md"); got != tt.source {
				t.Fatal("dry-run changed source", got)
			}
			if _, err := os.Stat(filepath.Join(vault, tt.to)); !os.IsNotExist(err) {
				t.Fatal("dry-run created destination", err)
			}
			if _, err := Move(vault, MoveOptions{From: tt.from, To: tt.to}); err != nil {
				t.Fatal(err)
			}
			if got, want := readVaultFile(t, vault, tt.movedSource), strings.Replace(tt.source, tt.oldRaw, tt.raw, 1); got != want {
				t.Fatalf("source = %q, want %q", got, want)
			}
			wantTarget := tt.to
			if tt.from == "Source.md" {
				wantTarget = tt.target
			}
			check := func() {
				t.Helper()
				edges := queryEdges(t, dbPath(vault), tt.movedSource)
				if len(edges) != 1 || edges[0].rawLink != tt.raw || edges[0].targetKey != noteKey(wantTarget) || edges[0].subpath != tt.subpath || edges[0].linkType != LinkTypeFrontmatterWikilink {
					t.Fatalf("edges: %+v", edges)
				}
				got, err := Resolve(vault, tt.movedSource, tt.raw)
				if err != nil || got.Path != wantTarget || got.Subpath != tt.subpath {
					t.Fatalf("resolve: %+v, %v", got, err)
				}
			}
			check()
			buildVault(t, vault)
			check()
		})
	}
}

func TestFrontmatterBacktickRewritePreflightRejectsWithoutMutation(t *testing.T) {
	for _, tt := range []struct{ name, source, wantErr string }{
		{"body", "[[A]]\n", "cannot preserve wikilink destination"},
		{"encoded scalar", "---\nrelated: \"\\u005b\\u005bA\\u005d\\u005d\"\n---\n", "correspondence"},
	} {
		for _, dryRun := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dry=%v", tt.name, dryRun), func(t *testing.T) {
				vault := newMoveVault(t, map[string]string{"Source.md": tt.source, "A.md": "target\n"})
				snapshot := func() map[string]string {
					files := map[string]string{}
					if err := filepath.WalkDir(vault, func(path string, entry os.DirEntry, err error) error {
						if err != nil {
							return err
						}
						if entry.IsDir() {
							files[path] = "directory"
							return nil
						}
						content, err := os.ReadFile(path)
						files[path] = string(content)
						return err
					}); err != nil {
						t.Fatal(err)
					}
					return files
				}
				before := snapshot()
				var err error
				if dryRun {
					_, err = PlanMoveTemplate(vault, MoveTemplateOptions{From: "A.md", Template: "Z`Q.md"})
				} else {
					_, err = Move(vault, MoveOptions{From: "A.md", To: "Z`Q.md"})
				}
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %s", err, tt.wantErr)
				}
				if after := snapshot(); !reflect.DeepEqual(before, after) {
					t.Fatalf("rejected move changed vault: before=%v after=%v", before, after)
				}
			})
		}
	}
}
