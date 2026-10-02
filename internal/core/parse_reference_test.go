package core

import (
	"reflect"
	"testing"
)

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
		{"multiline label is not partial shortcut", "[id]: B.md\n[text\n][id] [text][\nid]", nil, nil},
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
