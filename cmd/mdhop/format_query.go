package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

var validQueryFieldsCLI = map[string]bool{
	core.FieldQueryBacklinks: true,
	core.FieldQueryTags:      true,
	core.FieldQueryTwoHop:    true,
	core.FieldQueryOutgoing:  true,
	core.FieldQueryHead:      true,
	core.FieldQuerySnippet:   true,
	core.FieldQueryMeta:      true,
}

// queryJSONOutput is the JSON-serializable form of QueryResult.
type queryJSONOutput struct {
	Entry     *jsonNodeInfo       `json:"entry"`
	Backlinks []jsonNodeInfo      `json:"backlinks,omitempty"`
	Outgoing  []jsonNodeInfo      `json:"outgoing,omitempty"`
	Tags      []string            `json:"tags,omitempty"`
	TwoHop    []jsonTwoHop        `json:"twohop,omitempty"`
	Head      []string            `json:"head,omitempty"`
	Snippets  []jsonSnippet       `json:"snippet,omitempty"`
	Meta      map[string][]string `json:"meta,omitempty"`
}

type jsonTwoHop struct {
	Via     jsonNodeInfo   `json:"via"`
	Targets []jsonNodeInfo `json:"targets"`
}

type jsonSnippet struct {
	Source  string   `json:"source"`
	Lines   string   `json:"lines"`
	Content []string `json:"content"`
}

func printQueryJSON(w io.Writer, r *core.QueryResult) error {
	out := queryJSONOutput{
		Entry: func() *jsonNodeInfo { v := toJSONNodeInfo(r.Entry); return &v }(),
	}
	if r.Backlinks != nil {
		out.Backlinks = make([]jsonNodeInfo, len(r.Backlinks))
		for i, n := range r.Backlinks {
			out.Backlinks[i] = toJSONNodeInfo(n)
		}
	}
	if r.Outgoing != nil {
		out.Outgoing = make([]jsonNodeInfo, len(r.Outgoing))
		for i, n := range r.Outgoing {
			out.Outgoing[i] = toJSONNodeInfo(n)
		}
	}
	if r.Tags != nil {
		out.Tags = r.Tags
	}
	if r.TwoHop != nil {
		out.TwoHop = make([]jsonTwoHop, len(r.TwoHop))
		for i, th := range r.TwoHop {
			targets := make([]jsonNodeInfo, len(th.Targets))
			for j, t := range th.Targets {
				targets[j] = toJSONNodeInfo(t)
			}
			out.TwoHop[i] = jsonTwoHop{
				Via:     toJSONNodeInfo(th.Via),
				Targets: targets,
			}
		}
	}
	if r.Head != nil {
		out.Head = r.Head
	}
	if r.Snippets != nil {
		out.Snippets = make([]jsonSnippet, len(r.Snippets))
		for i, s := range r.Snippets {
			out.Snippets[i] = jsonSnippet{
				Source:  s.SourcePath,
				Lines:   fmt.Sprintf("%d-%d", s.LineStart, s.LineEnd),
				Content: s.Lines,
			}
		}
	}

	if r.Meta != nil {
		m := make(map[string][]string)
		for _, mr := range r.Meta {
			m[mr.Key] = append(m[mr.Key], mr.Value)
		}
		out.Meta = m
	}

	return encodeJSON(w, out)
}

func printQueryText(w io.Writer, r *core.QueryResult) error {
	// entry (always present)
	fmt.Fprintln(w, "entry:")
	writeNodeInfoText(w, r.Entry, "  ", "  ")

	if r.Backlinks != nil {
		fmt.Fprintf(w, "%s:\n", core.FieldQueryBacklinks)
		for _, n := range r.Backlinks {
			writeNodeInfoText(w, n, "- ", "  ")
		}
	}

	if r.Outgoing != nil {
		fmt.Fprintf(w, "%s:\n", core.FieldQueryOutgoing)
		for _, n := range r.Outgoing {
			writeNodeInfoText(w, n, "- ", "  ")
		}
	}

	if r.Tags != nil {
		fmt.Fprintf(w, "%s:\n", core.FieldQueryTags)
		for _, t := range r.Tags {
			fmt.Fprintf(w, "- %s\n", t)
		}
	}

	if r.TwoHop != nil {
		fmt.Fprintf(w, "%s:\n", core.FieldQueryTwoHop)
		for _, th := range r.TwoHop {
			fmt.Fprintf(w, "- via: %s\n", nodeInfoOneLine(th.Via))
			fmt.Fprintln(w, "  targets:")
			for _, t := range th.Targets {
				fmt.Fprintf(w, "  - %s\n", nodeInfoOneLine(t))
			}
		}
	}

	if r.Head != nil {
		fmt.Fprintf(w, "%s:\n", core.FieldQueryHead)
		for _, line := range r.Head {
			fmt.Fprintf(w, "- %q\n", line)
		}
	}

	if r.Snippets != nil {
		fmt.Fprintf(w, "%s:\n", core.FieldQuerySnippet)
		for _, s := range r.Snippets {
			fmt.Fprintf(w, "- source: %s\n", s.SourcePath)
			fmt.Fprintf(w, "  lines: %d-%d\n", s.LineStart, s.LineEnd)
			fmt.Fprintln(w, "  content:")
			for _, line := range s.Lines {
				fmt.Fprintf(w, "  - %q\n", line)
			}
		}
	}

	if r.Meta != nil && len(r.Meta) > 0 {
		fmt.Fprintf(w, "%s:\n", core.FieldQueryMeta)
		for _, m := range r.Meta {
			fmt.Fprintf(w, "- %s: %s\n", m.Key, m.Value)
		}
	}

	return nil
}

// writeNodeInfoText writes a NodeInfo in multi-line text format.
// firstIndent is the indent for the first line (type:), restIndent for subsequent lines.
func writeNodeInfoText(w io.Writer, n core.NodeInfo, firstIndent, restIndent string) {
	fmt.Fprintf(w, "%stype: %s\n", firstIndent, n.Type)
	fmt.Fprintf(w, "%sname: %s\n", restIndent, n.Name)
	if n.Type == core.NodeTypeNote || n.Type == core.NodeTypeAsset {
		fmt.Fprintf(w, "%spath: %s\n", restIndent, n.Path)
		fmt.Fprintf(w, "%sexists: %v\n", restIndent, n.Exists)
	}
}

// nodeInfoOneLine returns a compact one-line representation for twohop via/targets.
// Format: "note: path" or "phantom: name" or "tag: name"
func nodeInfoOneLine(n core.NodeInfo) string {
	switch n.Type {
	case core.NodeTypeNote, core.NodeTypeAsset:
		return fmt.Sprintf("%s: %s", n.Type, n.Path)
	default:
		return fmt.Sprintf("%s: %s", n.Type, n.Name)
	}
}
