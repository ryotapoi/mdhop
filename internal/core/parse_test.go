package core

import (
	"testing"
)

func parseLinksSlice(content string) []linkOccur {
	return parseLinks(content).Links
}

func TestParseWikiLinkBasic(t *testing.T) {
	links := parseLinksSlice("# A\n\n[[B]]\n")
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
	links := parseLinksSlice("[[B|alias]]\n")
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
	links := parseLinksSlice("[[B#heading]]\n")
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
	links := parseLinksSlice("[[#Heading]]\n")
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
	links := parseLinksSlice(content)
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
	links := parseLinksSlice("[link](sub/C.md)\n")
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
	links := parseLinksSlice("[up](../Root.md)\n")
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
	links := parseLinksSlice("[link](note.md#section)\n")
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
	links := parseLinksSlice("Hello #tag world\n")
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
	links := parseLinksSlice("#tag at start\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#tag" {
		t.Errorf("target = %q, want #tag", tags[0].target)
	}
}

func TestParseTagNestedExpansion(t *testing.T) {
	links := parseLinksSlice("#a/b/c\n")
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
	links := parseLinksSlice(content)
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("expected no tags in code fence, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagInlineCodeExcluded(t *testing.T) {
	content := "`#not-a-tag`\n"
	links := parseLinksSlice(content)
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("expected no tags in inline code, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagHeadingNotTag(t *testing.T) {
	content := "# Heading\n"
	links := parseLinksSlice(content)
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("heading should not be a tag, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagInWikiLinkNotTag(t *testing.T) {
	content := "[[#Heading]]\n"
	links := parseLinksSlice(content)
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("#Heading in wikilink should not be a tag, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagInMarkdownLinkNotTag(t *testing.T) {
	content := "[link](#heading)\n"
	links := parseLinksSlice(content)
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("#heading in markdown link should not be a tag, got %d: %+v", len(tags), tags)
	}
}

func TestParseFrontmatterTags(t *testing.T) {
	content := "---\ntags:\n  - foo\n  - bar\n---\n# Content\n"
	links := parseLinksSlice(content)
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
	links := parseLinksSlice(content)
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
	links := parseLinksSlice(content)
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

func TestParseFrontmatterScalarTags(t *testing.T) {
	content := "---\ntags: foo, bar\n---\n"
	links := parseLinksSlice(content)
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
	// "---" is line 1, "tags: foo, bar" is line 2
	if fmTags[0].lineStart != 2 {
		t.Errorf("foo lineStart = %d, want 2", fmTags[0].lineStart)
	}
	if fmTags[1].lineStart != 2 {
		t.Errorf("bar lineStart = %d, want 2", fmTags[1].lineStart)
	}
}

func TestParseFrontmatterScalarSingleTag(t *testing.T) {
	content := "---\ntags: solo\n---\n"
	links := parseLinksSlice(content)
	fmTags := filterByType(links, "frontmatter")
	if len(fmTags) != 1 {
		t.Fatalf("expected 1 frontmatter tag, got %d: %+v", len(fmTags), fmTags)
	}
	if fmTags[0].target != "#solo" {
		t.Errorf("tag[0] = %q, want #solo", fmTags[0].target)
	}
}

func TestParseFrontmatterScalarNestedTags(t *testing.T) {
	content := "---\ntags: a/b/c\n---\n"
	links := parseLinksSlice(content)
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

func TestParseFrontmatterScalarHashPrefix(t *testing.T) {
	content := "---\ntags: \"#alpha, beta\"\n---\n"
	links := parseLinksSlice(content)
	fmTags := filterByType(links, "frontmatter")
	if len(fmTags) != 2 {
		t.Fatalf("expected 2 frontmatter tags, got %d: %+v", len(fmTags), fmTags)
	}
	if fmTags[0].target != "#alpha" {
		t.Errorf("tag[0] = %q, want #alpha", fmTags[0].target)
	}
	if fmTags[1].target != "#beta" {
		t.Errorf("tag[1] = %q, want #beta", fmTags[1].target)
	}
}

func TestParseFrontmatterScalarEmptySegment(t *testing.T) {
	content := "---\ntags: \"foo,, bar\"\n---\n"
	links := parseLinksSlice(content)
	fmTags := filterByType(links, "frontmatter")
	if len(fmTags) != 2 {
		t.Fatalf("expected 2 frontmatter tags (empty segment skipped), got %d: %+v", len(fmTags), fmTags)
	}
	if fmTags[0].target != "#foo" {
		t.Errorf("tag[0] = %q, want #foo", fmTags[0].target)
	}
	if fmTags[1].target != "#bar" {
		t.Errorf("tag[1] = %q, want #bar", fmTags[1].target)
	}
}

func TestParseURLIgnored(t *testing.T) {
	links := parseLinksSlice("[link](https://example.com)\n")
	if len(links) != 0 {
		t.Errorf("URL should be ignored, got %d: %+v", len(links), links)
	}
}

func TestParseWikiLinkWithMdExtension(t *testing.T) {
	links := parseLinksSlice("[[Note.md]]\n")
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].target != "Note" {
		t.Errorf("target = %q, want Note", links[0].target)
	}
}

func TestParseWikiLinkVaultRelative(t *testing.T) {
	links := parseLinksSlice("[[path/to/Note]]\n")
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
	links := parseLinksSlice("[link](/sub/B.md)\n")
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

func TestParseTagUnicodeJapanese(t *testing.T) {
	links := parseLinksSlice("text #あいうえお end\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#あいうえお" {
		t.Errorf("target = %q, want #あいうえお", tags[0].target)
	}
}

func TestParseTagHyphenated(t *testing.T) {
	links := parseLinksSlice("text #my-tag end\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#my-tag" {
		t.Errorf("target = %q, want #my-tag", tags[0].target)
	}
}

func TestParseTagUnicodeNested(t *testing.T) {
	links := parseLinksSlice("#日本語/サブタグ\n")
	tags := filterByType(links, "tag")
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %+v", len(tags), tags)
	}
	expected := []string{"#日本語", "#日本語/サブタグ"}
	for i, tag := range tags {
		if tag.target != expected[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag.target, expected[i])
		}
	}
}

func TestParseTagMixedUnicodeASCII(t *testing.T) {
	links := parseLinksSlice("#project-日本語\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#project-日本語" {
		t.Errorf("target = %q, want #project-日本語", tags[0].target)
	}
}

func TestParseTagDigitFirst(t *testing.T) {
	links := parseLinksSlice("#123\n")
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("digit-first should not be a tag, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagDigitNotFirst(t *testing.T) {
	links := parseLinksSlice("#tag123\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#tag123" {
		t.Errorf("target = %q, want #tag123", tags[0].target)
	}
}

func TestParseTagUnicodePunctTermination(t *testing.T) {
	// U+2014 (EM DASH) is in General Punctuation range → terminates tag
	links := parseLinksSlice("#tag\u2014rest\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#tag" {
		t.Errorf("target = %q, want #tag", tags[0].target)
	}
}

func TestParseTagMultiLevelHeadingNotTag(t *testing.T) {
	for _, line := range []string{"## Heading", "### Heading", "##heading"} {
		links := parseLinksSlice(line + "\n")
		tags := filterByType(links, "tag")
		if len(tags) != 0 {
			t.Errorf("line %q: expected 0 tags, got %d: %+v", line, len(tags), tags)
		}
	}
}

func TestParseTagMultipleUnicode(t *testing.T) {
	links := parseLinksSlice("#alpha #ベータ #gamma-delta\n")
	tags := filterByType(links, "tag")
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %+v", len(tags), tags)
	}
	expected := []string{"#alpha", "#ベータ", "#gamma-delta"}
	for i, tag := range tags {
		if tag.target != expected[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag.target, expected[i])
		}
	}
}

func TestParseTagTrailingSlash(t *testing.T) {
	links := parseLinksSlice("#tag/\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#tag" {
		t.Errorf("target = %q, want #tag", tags[0].target)
	}
}

func TestParseTagTrailingPeriod(t *testing.T) {
	// Period is in the blacklist → terminates tag
	links := parseLinksSlice("#tag.\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#tag" {
		t.Errorf("target = %q, want #tag", tags[0].target)
	}
}

func TestParseTagSlashFirst(t *testing.T) {
	links := parseLinksSlice("#/tag\n")
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("slash-first should not be a tag, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagHyphenFirst(t *testing.T) {
	links := parseLinksSlice("#-tag\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#-tag" {
		t.Errorf("target = %q, want #-tag", tags[0].target)
	}
}

func TestParseTagURLFragment(t *testing.T) {
	// '#' preceded by '/' (not space) → not a tag boundary
	links := parseLinksSlice("https://example.com/#tag\n")
	tags := filterByType(links, "tag")
	if len(tags) != 0 {
		t.Errorf("URL fragment should not be a tag, got %d: %+v", len(tags), tags)
	}
}

func TestParseTagNBSP(t *testing.T) {
	// U+00A0 (NBSP) is unicode.IsSpace → acts as tag boundary
	links := parseLinksSlice("text\u00A0#tag end\n")
	tags := filterByType(links, "tag")
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag after NBSP, got %d: %+v", len(tags), tags)
	}
	if tags[0].target != "#tag" {
		t.Errorf("target = %q, want #tag", tags[0].target)
	}
}

func parseMeta(content string) []FrontmatterEntry {
	return parseLinks(content).Meta
}

func TestParseFrontmatterMetaScalar(t *testing.T) {
	content := "---\ntitle: Hello\ndate: 2024-01-15\n---\n"
	meta := parseMeta(content)
	if len(meta) != 2 {
		t.Fatalf("expected 2 meta entries, got %d: %+v", len(meta), meta)
	}
	if meta[0].Key != "title" || meta[0].Value != "Hello" {
		t.Errorf("meta[0] = %+v, want {Key:title, Value:Hello}", meta[0])
	}
	if meta[0].Line != 2 {
		t.Errorf("meta[0].Line = %d, want 2", meta[0].Line)
	}
	if meta[1].Key != "date" || meta[1].Value != "2024-01-15" {
		t.Errorf("meta[1] = %+v, want {Key:date, Value:2024-01-15}", meta[1])
	}
	if meta[1].Line != 3 {
		t.Errorf("meta[1].Line = %d, want 3", meta[1].Line)
	}
}

func TestParseFrontmatterMetaNoFrontmatter(t *testing.T) {
	content := "# No frontmatter\nsome content\n"
	meta := parseMeta(content)
	if meta != nil {
		t.Errorf("expected nil meta, got %+v", meta)
	}
}

func TestParseFrontmatterMetaSequenceMixed(t *testing.T) {
	content := "---\nmixed:\n  - simple\n  - {k: v}\n  - another\n---\n"
	meta := parseMeta(content)
	if len(meta) != 2 {
		t.Fatalf("expected 2 meta entries (mapping in sequence skipped), got %d: %+v", len(meta), meta)
	}
	if meta[0].Value != "simple" {
		t.Errorf("meta[0].Value = %q, want simple", meta[0].Value)
	}
	if meta[1].Value != "another" {
		t.Errorf("meta[1].Value = %q, want another", meta[1].Value)
	}
}

func TestParseFrontmatterMetaSkipEmpty(t *testing.T) {
	content := "---\nempty:\ntitle: Test\n---\n"
	meta := parseMeta(content)
	if len(meta) != 1 {
		t.Fatalf("expected 1 meta entry (empty skipped), got %d: %+v", len(meta), meta)
	}
	if meta[0].Key != "title" || meta[0].Value != "Test" {
		t.Errorf("meta[0] = %+v, want {Key:title, Value:Test}", meta[0])
	}
}

func TestParseFrontmatterMetaSkipMapping(t *testing.T) {
	content := "---\ntitle: Test\nnested:\n  k: v\nstatus: draft\n---\n"
	meta := parseMeta(content)
	if len(meta) != 2 {
		t.Fatalf("expected 2 meta entries (mapping skipped), got %d: %+v", len(meta), meta)
	}
	if meta[0].Key != "title" || meta[0].Value != "Test" {
		t.Errorf("meta[0] = %+v, want {Key:title, Value:Test}", meta[0])
	}
	if meta[1].Key != "status" || meta[1].Value != "draft" {
		t.Errorf("meta[1] = %+v, want {Key:status, Value:draft}", meta[1])
	}
}

func TestParseFrontmatterMetaTagsScalar(t *testing.T) {
	content := "---\ntags: foo, bar\n---\n"
	// Links should still work.
	links := parseLinksSlice(content)
	fmTags := filterByType(links, "frontmatter")
	if len(fmTags) != 2 {
		t.Fatalf("expected 2 frontmatter linkOccur, got %d: %+v", len(fmTags), fmTags)
	}
	// Meta: comma-split, # removed.
	meta := parseMeta(content)
	if len(meta) != 2 {
		t.Fatalf("expected 2 meta entries, got %d: %+v", len(meta), meta)
	}
	if meta[0].Value != "foo" {
		t.Errorf("meta[0].Value = %q, want foo", meta[0].Value)
	}
	if meta[1].Value != "bar" {
		t.Errorf("meta[1].Value = %q, want bar", meta[1].Value)
	}
}

func TestParseFrontmatterMetaTagsCoexist(t *testing.T) {
	content := "---\ntags:\n  - project\n  - status/active\n---\n"
	// Links: nested expansion → #project, #status, #status/active = 3 linkOccur
	links := parseLinksSlice(content)
	fmTags := filterByType(links, "frontmatter")
	if len(fmTags) != 3 {
		t.Fatalf("expected 3 frontmatter linkOccur (with expansion), got %d: %+v", len(fmTags), fmTags)
	}
	// Meta: no expansion → 2 entries
	meta := parseMeta(content)
	if len(meta) != 2 {
		t.Fatalf("expected 2 meta entries, got %d: %+v", len(meta), meta)
	}
	if meta[0].Key != "tags" || meta[0].Value != "project" {
		t.Errorf("meta[0] = %+v, want {Key:tags, Value:project}", meta[0])
	}
	if meta[1].Key != "tags" || meta[1].Value != "status/active" {
		t.Errorf("meta[1] = %+v, want {Key:tags, Value:status/active}", meta[1])
	}
}

func TestParseFrontmatterMetaSequence(t *testing.T) {
	content := "---\naliases:\n  - foo\n  - bar\n---\n"
	meta := parseMeta(content)
	if len(meta) != 2 {
		t.Fatalf("expected 2 meta entries, got %d: %+v", len(meta), meta)
	}
	if meta[0].Key != "aliases" || meta[0].Value != "foo" {
		t.Errorf("meta[0] = %+v, want {Key:aliases, Value:foo}", meta[0])
	}
	if meta[0].Line != 3 {
		t.Errorf("meta[0].Line = %d, want 3", meta[0].Line)
	}
	if meta[1].Key != "aliases" || meta[1].Value != "bar" {
		t.Errorf("meta[1] = %+v, want {Key:aliases, Value:bar}", meta[1])
	}
	if meta[1].Line != 4 {
		t.Errorf("meta[1].Line = %d, want 4", meta[1].Line)
	}
}

func TestParseFrontmatterMetaNullValues(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"null keyword", "---\ntitle: null\nstatus: draft\n---\n", 1},
		{"tilde null", "---\ntitle: ~\nstatus: draft\n---\n", 1},
		{"tags null", "---\ntags: null\nstatus: draft\n---\n", 1},
		{"tags tilde", "---\ntags: ~\nstatus: draft\n---\n", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := parseMeta(tt.content)
			if len(meta) != tt.want {
				t.Errorf("expected %d meta entries, got %d: %+v", tt.want, len(meta), meta)
			}
			if len(meta) > 0 && meta[0].Key != "status" {
				t.Errorf("meta[0].Key = %q, want status", meta[0].Key)
			}
			// Ensure no #null tag is generated
			links := parseLinksSlice(tt.content)
			for _, l := range links {
				if l.target == "#null" || l.target == "#~" {
					t.Errorf("unexpected tag link: %+v", l)
				}
			}
		})
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
