package core

import (
	"testing"
)

func TestParseWikiLinkBasic(t *testing.T) {
	links := parseLinks("# A\n\n[[B]]\n")
	var found bool
	for _, l := range links {
		if l.linkType == "wikilink" && l.target == "B" && l.isBasename {
			found = true
			if l.rawLink != "[[B]]" {
				t.Errorf("rawLink = %q, want [[B]]", l.rawLink)
			}
			if l.lineStart != 3 || l.lineEnd != 3 {
				t.Errorf("line = %d-%d, want 3-3", l.lineStart, l.lineEnd)
			}
		}
	}
	if !found {
		t.Fatalf("wikilink to B not found, got %+v", links)
	}
}

func TestParseWikiLinkWithAlias(t *testing.T) {
	links := parseLinks("[[B|alias]]\n")
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	l := links[0]
	if l.target != "B" {
		t.Errorf("target = %q, want B", l.target)
	}
	if l.rawLink != "[[B|alias]]" {
		t.Errorf("rawLink = %q, want [[B|alias]]", l.rawLink)
	}
}

func TestParseWikiLinkWithSubpath(t *testing.T) {
	links := parseLinks("[[B#heading]]\n")
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	l := links[0]
	if l.target != "B" {
		t.Errorf("target = %q, want B", l.target)
	}
	if l.subpath != "#heading" {
		t.Errorf("subpath = %q, want #heading", l.subpath)
	}
	if l.rawLink != "[[B#heading]]" {
		t.Errorf("rawLink = %q, want [[B#heading]]", l.rawLink)
	}
}

func TestParseWikiLinkSelfHeading(t *testing.T) {
	links := parseLinks("[[#Heading]]\n")
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	l := links[0]
	if l.target != "" {
		t.Errorf("target = %q, want empty", l.target)
	}
	if l.subpath != "#Heading" {
		t.Errorf("subpath = %q, want #Heading", l.subpath)
	}
	if l.linkType != "wikilink" {
		t.Errorf("linkType = %q, want wikilink", l.linkType)
	}
}

func TestParseWikiLinkLineNumber(t *testing.T) {
	content := "line1\n[[A]]\nline3\n[[B]]\n"
	links := parseLinks(content)
	wikilinks := filterByType(links, "wikilink")
	if len(wikilinks) != 2 {
		t.Fatalf("expected 2 wikilinks, got %d", len(wikilinks))
	}
	if wikilinks[0].lineStart != 2 {
		t.Errorf("first link line = %d, want 2", wikilinks[0].lineStart)
	}
	if wikilinks[1].lineStart != 4 {
		t.Errorf("second link line = %d, want 4", wikilinks[1].lineStart)
	}
}

func TestParseMarkdownLink(t *testing.T) {
	links := parseLinks("[link](sub/C.md)\n")
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	l := links[0]
	if l.linkType != "markdown" {
		t.Errorf("linkType = %q, want markdown", l.linkType)
	}
	if l.target != "sub/C" {
		t.Errorf("target = %q, want sub/C", l.target)
	}
	if l.rawLink != "[link](sub/C.md)" {
		t.Errorf("rawLink = %q, want [link](sub/C.md)", l.rawLink)
	}
}

func TestParseMarkdownLinkRelative(t *testing.T) {
	links := parseLinks("[up](../Root.md)\n")
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	l := links[0]
	if !l.isRelative {
		t.Error("expected isRelative=true")
	}
	if l.target != "../Root" {
		t.Errorf("target = %q, want ../Root", l.target)
	}
}

func TestParseMarkdownLinkSubpath(t *testing.T) {
	links := parseLinks("[link](note.md#section)\n")
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	l := links[0]
	if l.target != "note" {
		t.Errorf("target = %q, want note", l.target)
	}
	if l.subpath != "#section" {
		t.Errorf("subpath = %q, want #section", l.subpath)
	}
}

func TestParseTagBasic(t *testing.T) {
	links := parseLinks("Hello #tag world\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#tag" {
		t.Errorf("target = %q, want #tag", tags[0].target)
	}
	if tags[0].lineStart != 1 {
		t.Errorf("lineStart = %d, want 1", tags[0].lineStart)
	}
}

func TestParseTagAtLineStart(t *testing.T) {
	links := parseLinks("#tag at start\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#tag" {
		t.Errorf("target = %q, want #tag", tags[0].target)
	}
}

func TestParseTagNestedExpansion(t *testing.T) {
	links := parseLinks("#a/b/c\n")
	tags := filterByType(links, "tag")
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags from nested expansion, got %d: %+v", len(tags), tags)
	}
	expected := []string{"#a", "#a/b", "#a/b/c"}
	for i, tag := range tags {
		if tag.target != expected[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag.target, expected[i])
		}
	}
}

func TestParseTagCodeFenceExcluded(t *testing.T) {
	content := "```\n#not-a-tag\n```\n"
	links := parseLinks(content)
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("expected no tags in code fence, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagInlineCodeExcluded(t *testing.T) {
	content := "`#not-a-tag`\n"
	links := parseLinks(content)
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("expected no tags in inline code, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagHeadingNotTag(t *testing.T) {
	content := "# Heading\n"
	links := parseLinks(content)
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("heading should not be a tag, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagInWikiLinkNotTag(t *testing.T) {
	content := "[[#Heading]]\n"
	links := parseLinks(content)
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("#Heading in wikilink should not be a tag, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagInMarkdownLinkNotTag(t *testing.T) {
	content := "[link](#heading)\n"
	links := parseLinks(content)
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("#heading in markdown link should not be a tag, got %d: %+v", len(tags), tags)
	}
}

func TestParseFrontmatterTags(t *testing.T) {
	content := "---\ntags:\n  - foo\n  - bar\n---\n# Content\n"
	links := parseLinks(content)
	fmTags := filterByType(links, "frontmatter")
	if len(fmTags) != 2 {
		t.Fatalf("expected 2 frontmatter tags, got %d: %+v", len(fmTags), fmTags)
	}
	if fmTags[0].target != "#foo" {
		t.Errorf("tag[0] = %q, want #foo", fmTags[0].target)
	}
	if fmTags[1].target != "#bar" {
		t.Errorf("tag[1] = %q, want #bar", fmTags[1].target)
	}
}

func TestParseFrontmatterTagLineNumbers(t *testing.T) {
	content := "---\ntags:\n  - foo\n  - bar\n---\n"
	links := parseLinks(content)
	fmTags := filterByType(links, "frontmatter")
	if len(fmTags) != 2 {
		t.Fatalf("expected 2 frontmatter tags, got %d: %+v", len(fmTags), fmTags)
	}
	// "---" is line 1, "tags:" is line 2, "  - foo" is line 3, "  - bar" is line 4
	if fmTags[0].lineStart != 3 {
		t.Errorf("foo lineStart = %d, want 3", fmTags[0].lineStart)
	}
	if fmTags[1].lineStart != 4 {
		t.Errorf("bar lineStart = %d, want 4", fmTags[1].lineStart)
	}
}

func TestParseFrontmatterNestedTags(t *testing.T) {
	content := "---\ntags:\n  - a/b/c\n---\n"
	links := parseLinks(content)
	fmTags := filterByType(links, "frontmatter")
	if len(fmTags) != 3 {
		t.Fatalf("expected 3 frontmatter tags from nested expansion, got %d: %+v", len(fmTags), fmTags)
	}
	expected := []string{"#a", "#a/b", "#a/b/c"}
	for i, tag := range fmTags {
		if tag.target != expected[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag.target, expected[i])
		}
	}
}

func TestParseURLIgnored(t *testing.T) {
	links := parseLinks("[link](https://example.com)\n")
	if len(links) != 0 {
		t.Errorf("URL should be ignored, got %d: %+v", len(links), links)
	}
}

func TestParseWikiLinkWithMdExtension(t *testing.T) {
	links := parseLinks("[[Note.md]]\n")
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].target != "Note" {
		t.Errorf("target = %q, want Note", links[0].target)
	}
}

func TestParseWikiLinkVaultRelative(t *testing.T) {
	links := parseLinks("[[path/to/Note]]\n")
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	l := links[0]
	if l.isBasename {
		t.Error("path/to/Note should not be basename")
	}
	if l.isRelative {
		t.Error("path/to/Note should not be relative")
	}
	if l.target != "path/to/Note" {
		t.Errorf("target = %q, want path/to/Note", l.target)
	}
}

func TestParseMarkdownLinkSlashPrefix(t *testing.T) {
	links := parseLinks("[link](/sub/B.md)\n")
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	l := links[0]
	if l.target != "/sub/B" {
		t.Errorf("target = %q, want /sub/B", l.target)
	}
	if l.isBasename {
		t.Error("/ prefix should not be basename")
	}
	if l.isRelative {
		t.Error("/ prefix should not be relative")
	}
}

func filterByType(links []linkOccur, linkType string) []linkOccur {
	var out []linkOccur
	for _, l := range links {
		if l.linkType == linkType {
			out = append(out, l)
		}
	}
	return out
}
