package core

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestMarkdownOpeningBracketEscapes(t *testing.T) {
	for _, form := range []struct {
		name, raw string
		typ       LinkType
		counts    []int
	}{
		{"inline", "[guide](B.md)", LinkTypeMarkdown, []int{0, 1, 2, 3}},
		{"full", "[text][guide]", LinkTypeMarkdownReference, []int{1, 2}},
		{"collapsed", "[guide][]", LinkTypeMarkdownReference, []int{1, 2}},
		{"shortcut", "[guide]", LinkTypeMarkdownReference, []int{0, 1, 2, 3}},
	} {
		for _, count := range form.counts {
			t.Run(fmt.Sprintf("%s/%d", form.name, count), func(t *testing.T) {
				content := "[guide]: B.md\n" + strings.Repeat(`\`, count) + form.raw + " [later](C.md)"
				want := []string{form.raw, "[later](C.md)"}
				if count%2 == 1 {
					want = []string{"[later](C.md)"}
					if form.name == "full" {
						// The literal opening bracket leaves a standalone shortcut.
						want = []string{"[guide]", "[later](C.md)"}
					}
				}
				links := parseLinks(content).Links
				if len(links) != len(want) {
					t.Fatalf("links = %+v, want raw links %v", links, want)
				}
				for i, raw := range want {
					target, typ := "B", form.typ
					if raw == "[later](C.md)" {
						target, typ = "C", LinkTypeMarkdown
					}
					got := links[i]
					if got.rawLink != raw || got.target != target || got.linkType != typ || got.lineStart != 2 || got.lineEnd != 2 {
						t.Fatalf("link %d = %+v, want %q / %q / %s on line 2", i, got, raw, target, typ)
					}
				}
			})
		}
	}
	for _, content := range []string{
		"[guide]: B.md\n`\\\\[guide] \\\\[text](B.md)`\n~~~\n\\\\[guide] \\\\[text](B.md)\n~~~",
		"[guide]: B.md\n\\\\[nested[guide]][guide] \\\\[gu\\ide]",
	} {
		if links := parseLinks(content).Links; len(links) != 0 {
			t.Fatalf("unsupported or code syntax produced links: %+v", links)
		}
	}
	content := "\\[literal\n[normal](B.md) [guide] ]\n[guide]: B.md"
	if links := parseLinks(content).Links; len(links) != 2 || links[0].rawLink != "[normal](B.md)" || links[1].rawLink != "[guide]" || links[0].lineStart != 2 || links[1].lineStart != 2 {
		t.Fatalf("escaped bracket started a multiline reference: %+v", links)
	}
}

func TestMarkdownReferenceLineEndings(t *testing.T) {
	for _, ending := range []struct{ name, value string }{{"LF", "\n"}, {"CRLF", "\r\n"}} {
		for _, destination := range []string{"B.md", "<B.md>"} {
			t.Run(ending.name+"/"+destination, func(t *testing.T) {
				content := strings.Join([]string{"# Note", "", "[guide]: " + destination, "", "[guide]", "", "[guide]", ""}, ending.value)
				links := parseLinks(content).Links
				if len(links) != 2 {
					t.Fatalf("links = %+v, want two reference uses", links)
				}
				for i, line := range []int{5, 7} {
					got := links[i]
					if got.target != "B" || got.referenceTarget != "B.md" || got.rawLink != "[guide]" || got.linkType != LinkTypeMarkdownReference || got.lineStart != line || got.lineEnd != line {
						t.Fatalf("link %d = %+v, want reference to B.md on line %d", i, got, line)
					}
				}
			})
		}
	}
}

func TestParseMarkdownReferences(t *testing.T) {
	tests := []struct {
		name, content string
		raw, targets  []string
	}{
		{"forms and later definition", "[Read][guide] [guide][] [guide] ![image][guide]\n[guide]: B.md", []string{"[Read][guide]", "[guide][]", "[guide]", "[image][guide]"}, []string{"B", "B", "B", "B"}},
		{"normalization and first valid definition", "   [ Ä  Guide ]: <folder/B.md#Heading> \"title\"\n[ä guide]: C.md\n[text][ä\tguide]", []string{"[text][ä\tguide]"}, []string{"folder/B"}},
		{"balanced parentheses and titles", "[a]: B(test).md 'title'\n[b]: C.md (title)\n[a] [b]", []string{"[a]", "[b]"}, []string{"B(test)", "C"}},
		{"undefined full is not shortcut", "[known]: B.md\n[known][missing] [missing][known] [missing][]", []string{"[missing][known]"}, []string{"B"}},
		{"unused external and self definitions", "[unused]: B.md\n[url]: https://example.com\n[self]: #Heading\n[missing] [url] [self]", nil, nil},
		{"code and frontmatter", "---\nref: '[guide]'\n---\n[guide]: B.md\n`[guide]`\n~~~\n[guide]\n~~~\n[guide]", []string{"[guide]"}, []string{"B"}},
		{"invalid definitions", "[guide]: B.md junk\n[x]: <C.md>junk\n[guide] [x]", nil, nil},
		{"multiline labels", "[id]: B.md\n[text\n][id] [text][\nid]", []string{"[text\n][id]", "[text][\nid]"}, []string{"B", "B"}},
		{"nested multiline label is not partial shortcut", "[id]: B.md\n[text\n[inner]][id]", nil, nil},
		{"three-line nested reference is not partial shortcut", "[text\n[id]\n][id]\n[id]: B.md\n", nil, nil},
		{"nested syntax is not partial shortcut", "[id]: B.md\n[outer [inner]][id] [text][nested[id]]", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := parseLinks(tt.content)
			var raw, targets []string
			for _, l := range pr.Links {
				if l.linkType != LinkTypeMarkdownReference {
					t.Fatalf("unexpected link: %+v", l)
				}
				raw = append(raw, l.rawLink)
				targets = append(targets, l.target)
				if l.referenceTarget == "" {
					t.Fatal("missing definition destination")
				}
			}
			if !reflect.DeepEqual(raw, tt.raw) || !reflect.DeepEqual(targets, tt.targets) {
				t.Fatalf("raw=%v targets=%v, want %v %v", raw, targets, tt.raw, tt.targets)
			}
		})
	}
}

func TestMarkdownReferenceMixedSpansAndTags(t *testing.T) {
	pr := parseLinks("[guide]: B.md#Heading \"#notTag\"\n[#notTag][guide] [normal](C.md) [[D]] [guide][] #real")
	var raw []string
	for _, l := range pr.Links {
		raw = append(raw, l.rawLink)
		if l.lineStart != 2 || l.lineEnd != 2 {
			t.Fatalf("wrong occurrence line: %+v", l)
		}
	}
	want := []string{"[[D]]", "[#notTag][guide]", "[normal](C.md)", "[guide][]", "#real"}
	if !reflect.DeepEqual(raw, want) {
		t.Fatalf("raw = %v, want %v", raw, want)
	}
}

func TestMarkdownReferenceUnmatchedProseBracket(t *testing.T) {
	for _, separator := range []string{"\n", "", "```\ncode\n```\n", "[unused]: C.md\n"} {
		t.Run(separator, func(t *testing.T) {
			pr := parseLinks("Literal opening bracket [\n" + separator + "[normal](B.md) [g]\n[g]: B.md\n")
			if len(pr.Links) != 2 {
				t.Fatalf("links = %+v, want normal and reference links", pr.Links)
			}
			for i, want := range []LinkType{LinkTypeMarkdown, LinkTypeMarkdownReference} {
				if pr.Links[i].linkType != want || pr.Links[i].target != "B" {
					t.Fatalf("link %d = %+v", i, pr.Links[i])
				}
			}
			if pr.Links[0].rawLink != "[normal](B.md)" || pr.Links[1].rawLink != "[g]" {
				t.Fatalf("raw links = %+v", pr.Links)
			}
		})
	}
}
