package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestRewriteMovedOutgoingLink(t *testing.T) {
	tests := []struct {
		name      string
		link      linkOccur
		from      string
		to        string
		preTarget string
		maps      movedLinkMaps
		want      string
		wantOK    bool
		wantErr   string
	}{
		{
			name: "relative wikilink preserves alias and subpath",
			link: linkOccur{rawLink: "[[./B#Heading|label]]", isRelative: true, linkType: LinkTypeWikilink},
			from: "A.md", to: "sub/A.md", want: "[[../B#Heading|label]]", wantOK: true,
		},
		{
			name: "relative frontmatter wikilink preserves syntax",
			link: linkOccur{rawLink: "[[./B#Heading|label]]", isRelative: true, linkType: LinkTypeFrontmatterWikilink},
			from: "A.md", to: "sub/A.md", want: "[[../B#Heading|label]]", wantOK: true,
		},
		{
			name: "relative markdown preserves extension and fragment",
			link: linkOccur{rawLink: "[label](./B.md#heading)", isRelative: true, linkType: LinkTypeMarkdown},
			from: "A.md", to: "sub/A.md", want: "[label](../B.md#heading)", wantOK: true,
		},
		{
			name: "relative markdown preserves destination whitespace",
			link: parseMarkdownLinks("[link]( ./B.md )", 1)[0],
			from: "A.md", to: "sub/A.md", want: "[link]( ../B.md )", wantOK: true,
		},
		{
			name: "relative markdown preserves whitespace and fragment for moved target",
			link: parseMarkdownLinks("[label](\t./B.md#heading \t)", 1)[0],
			from: "old/A.md", to: "new/A.md",
			maps: movedLinkMaps{movedFromTo: map[string]string{"old/B.md": "else/B.md"}},
			want: "[label](\t../else/B.md#heading \t)", wantOK: true,
		},
		{
			name: "relative markdown unchanged destination preserves whitespace",
			link: parseMarkdownLinks("[link]( ./B.md )", 1)[0],
			from: "old/A.md", to: "new/A.md",
			maps: movedLinkMaps{movedFromTo: map[string]string{"old/B.md": "new/B.md"}},
		},
		{
			name: "relative target moved with source",
			link: linkOccur{rawLink: "[[./B]]", isRelative: true, linkType: LinkTypeWikilink},
			from: "old/A.md", to: "new/A.md",
			maps: movedLinkMaps{movedFromTo: map[string]string{"old/B.md": "else/B.md"}},
			want: "[[../else/B]]", wantOK: true,
		},
		{
			name:      "basename root priority changes target",
			link:      linkOccur{rawLink: "[[A]]", target: "A", isBasename: true, linkType: LinkTypeWikilink},
			preTarget: "sub/A.md",
			maps:      movedLinkMaps{rootBasenameToPath: map[string]string{"a": "A.md"}},
			want:      "[[sub/A]]", wantOK: true,
		},
		{
			name:      "basename still resolves to same root target",
			link:      linkOccur{rawLink: "[[A]]", target: "A", isBasename: true, linkType: LinkTypeWikilink},
			preTarget: "A.md",
			maps:      movedLinkMaps{rootBasenameToPath: map[string]string{"a": "A.md"}},
		},
		{
			name:      "ambiguous basename rewrites to the original target",
			link:      linkOccur{rawLink: "[[A]]", target: "A", isBasename: true, linkType: LinkTypeWikilink},
			preTarget: "sub/A.md",
			maps:      movedLinkMaps{basenameCounts: map[string]int{"a": 2}},
			want:      "[[sub/A]]", wantOK: true,
		},
		{
			name: "basename missing target skips",
			link: linkOccur{rawLink: "[[A]]", target: "A", isBasename: true, linkType: LinkTypeWikilink},
		},
		{
			name:      "path target moved preserves markdown extension and fragment",
			link:      linkOccur{rawLink: "[label](old/B.md#heading)", target: "old/B.md", linkType: LinkTypeMarkdown},
			preTarget: "old/B.md",
			maps:      movedLinkMaps{movedFromTo: map[string]string{"old/B.md": "new/B.md"}},
			want:      "[label](new/B.md#heading)", wantOK: true,
		},
		{
			name:      "root relative target not moved skips",
			link:      linkOccur{rawLink: "[[root/B]]", target: "root/B", linkType: LinkTypeWikilink},
			preTarget: "root/B.md",
			maps:      movedLinkMaps{movedFromTo: map[string]string{"A.md": "sub/A.md"}},
		},
		{
			name:      "unsupported link type passes through",
			link:      linkOccur{rawLink: "[[A]]", target: "A", isBasename: true, linkType: LinkTypeTag},
			preTarget: "A.md",
			maps:      movedLinkMaps{movedFromTo: map[string]string{"A.md": "B.md"}},
		},
		{
			name: "relative vault escape fails",
			link: linkOccur{rawLink: "[[../../outside]]", isRelative: true, linkType: LinkTypeWikilink},
			from: "sub/A.md", to: "other/A.md", wantErr: "rewritten link would escape vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapsBefore := cloneMovedLinkMaps(tt.maps)
			got, ok, err := rewriteMovedOutgoingLink(tt.link, tt.from, tt.to, tt.preTarget, tt.maps)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ok != tt.wantOK {
				t.Fatalf("candidate = %v, want %v", ok, tt.wantOK)
			}
			if ok && got.newRawLink != tt.want {
				t.Errorf("new raw link = %q, want %q", got.newRawLink, tt.want)
			}
			if !reflect.DeepEqual(tt.maps, mapsBefore) {
				t.Errorf("rewrite mutated input maps: got %#v, want %#v", tt.maps, mapsBefore)
			}
		})
	}
}

func cloneMovedLinkMaps(maps movedLinkMaps) movedLinkMaps {
	cloneStrings := func(source map[string]string) map[string]string {
		if source == nil {
			return nil
		}
		result := make(map[string]string, len(source))
		for key, value := range source {
			result[key] = value
		}
		return result
	}
	cloneInts := func(source map[string]int) map[string]int {
		if source == nil {
			return nil
		}
		result := make(map[string]int, len(source))
		for key, value := range source {
			result[key] = value
		}
		return result
	}
	return movedLinkMaps{
		movedFromTo:        cloneStrings(maps.movedFromTo),
		basenameToPath:     cloneStrings(maps.basenameToPath),
		rootBasenameToPath: cloneStrings(maps.rootBasenameToPath),
		basenameCounts:     cloneInts(maps.basenameCounts),
	}
}
