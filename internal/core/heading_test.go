package core

import (
	"reflect"
	"testing"
)

func TestCollectHeadings(t *testing.T) {
	content := "---\ntitle: x\n---\n" +
		"# Top\n" +
		"Some text\n" +
		"## Sub Section\n" +
		"```\n# Not a heading (in fence)\n```\n" +
		"### Deep\n" +
		"## API `v2` Reference\n" +
		"#NoSpace is not a heading\n" +
		"####### Too many hashes\n"
	got := collectHeadings(content)
	// Spec: docs/specs/overview.md — anchor 正規化とインラインコードの扱い。
	want := []string{"Top", "Sub Section", "Deep", "API `v2` Reference"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collectHeadings = %v, want %v", got, want)
	}
}

func TestAtxHeadingText(t *testing.T) {
	tests := []struct {
		line string
		want string
		ok   bool
	}{
		{"# Title", "Title", true},
		{"###### Six", "Six", true},
		{"  ## Indented", "Indented", true},
		{"#NoSpace", "", false},
		{"####### Seven", "", false},
		{"text", "", false},
		{"#", "", false},
		{"## ", "", false},
	}
	for _, tt := range tests {
		got, ok := atxHeadingText(tt.line)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("atxHeadingText(%q) = (%q, %v), want (%q, %v)", tt.line, got, ok, tt.want, tt.ok)
		}
	}
}

func TestNormalizeAnchor(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"#Heading", "Heading", true},
		{"#My Section", "My Section", true},
		{"#heading!", "heading", true}, // punctuation stripped
		{"#heading?", "heading", true}, // same as heading!
		{"#A, B & C", "A B C", true},   // comma/ampersand stripped, spaces collapse
		{"#  spaced  out  ", "spaced out", true},
		{"#café", "café", true},   // accents preserved
		{"#Top", "Top", true},     // case preserved
		{"#^block-id", "", false}, // block reference
	}
	for _, tt := range tests {
		got, ok := normalizeAnchor(tt.in)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("normalizeAnchor(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestNormalizeAnchor_HeadingMatchesFragment(t *testing.T) {
	// A heading and a link fragment that differ only in punctuation must
	// normalize to the same anchor (Obsidian behavior).
	h, _ := normalizeAnchor("Setup & Config")
	f, _ := normalizeAnchor("#Setup and Config")
	if h == f {
		t.Errorf("expected differing text (and vs &) to differ: %q vs %q", h, f)
	}
	h2, _ := normalizeAnchor("Setup, Config")
	f2, _ := normalizeAnchor("#Setup Config")
	if h2 != f2 {
		t.Errorf("expected punctuation-only difference to match: %q vs %q", h2, f2)
	}
	// A heading with inline code matches a link fragment with the same words:
	// the backticks drop out but the code text ("v2") is preserved on both sides.
	hc, _ := normalizeAnchor("API `v2` Reference")
	fc, _ := normalizeAnchor("#API v2 Reference")
	if hc != fc {
		t.Errorf("expected inline-code heading to match fragment: %q vs %q", hc, fc)
	}
}
