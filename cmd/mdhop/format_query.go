package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

// A pointer preserves the difference between an unselected relation and an
// explicitly selected relation with no targets.
type queryJSONOutput struct {
	Entry     jsonNodeInfo     `json:"entry"`
	Backlinks *[]jsonQueryNode `json:"backlinks,omitempty"`
	Outgoing  *[]jsonQueryNode `json:"outgoing,omitempty"`
	TwoHop    *[]jsonTwoHop    `json:"2hoplink,omitempty"`
	Page      jsonQueryPage    `json:"page"`
}

type jsonTwoHop struct {
	jsonQueryNode
	Relation       []jsonQueryNode `json:"relation"`
	HiddenRelation bool            `json:"hidden_relation"`
}

type jsonQueryPage struct {
	Offset     int  `json:"offset"`
	Limit      *int `json:"limit"`
	NextOffset *int `json:"next_offset"`
}

func printQueryJSON(w io.Writer, r *core.QueryResult) error {
	out := queryJSONOutput{
		Entry: toJSONNodeInfo(r.Entry),
		Page: jsonQueryPage{
			Offset: r.Page.Offset, Limit: r.Page.Limit, NextOffset: r.Page.NextOffset,
		},
	}
	if r.Backlinks != nil {
		items := make([]jsonQueryNode, len(r.Backlinks))
		for i, node := range r.Backlinks {
			items[i] = toJSONQueryNode(node)
		}
		out.Backlinks = &items
	}
	if r.Outgoing != nil {
		items := make([]jsonQueryNode, len(r.Outgoing))
		for i, node := range r.Outgoing {
			items[i] = toJSONQueryNode(node)
		}
		out.Outgoing = &items
	}
	if r.TwoHop != nil {
		items := make([]jsonTwoHop, len(r.TwoHop))
		for i, target := range r.TwoHop {
			relations := make([]jsonQueryNode, len(target.Relation))
			for j, via := range target.Relation {
				relations[j] = toJSONQueryNode(via)
			}
			items[i] = jsonTwoHop{
				jsonQueryNode:  toJSONQueryNode(target.QueryNode),
				Relation:       relations,
				HiddenRelation: target.HiddenRelation,
			}
		}
		out.TwoHop = &items
	}
	return encodeJSON(w, out)
}

func printQueryText(w io.Writer, r *core.QueryResult) error {
	output := &textErrorWriter{w: w}
	w = output
	fmt.Fprintln(w, "entry:")
	writeQueryNodeText(w, r.Entry, "  ")
	if r.Backlinks != nil {
		writeQueryNodesText(w, core.FieldQueryBacklinks, r.Backlinks)
	}
	if r.Outgoing != nil {
		writeQueryNodesText(w, core.FieldQueryOutgoing, r.Outgoing)
	}
	if r.TwoHop != nil {
		fmt.Fprintln(w, "2hoplink:")
		if len(r.TwoHop) == 0 {
			fmt.Fprintln(w, "  （該当なし）")
		}
		for _, target := range r.TwoHop {
			writeQueryNodeText(w, target.NodeInfo, "  ")
			writeQueryPreviewText(w, target.QueryNode, "    ")
			fmt.Fprintln(w, "    relation:")
			for _, via := range target.Relation {
				writeQueryNodeText(w, via.NodeInfo, "      ")
				writeQueryPreviewText(w, via, "        ")
			}
			if target.HiddenRelation {
				fmt.Fprintln(w, "    hidden_relation: true")
			}
		}
	}
	fmt.Fprintln(w, "page:")
	fmt.Fprintf(w, "  offset: %d\n", r.Page.Offset)
	writeQueryPageNumber(w, "limit", r.Page.Limit)
	writeQueryPageNumber(w, "next_offset", r.Page.NextOffset)
	return output.err
}

func writeQueryNodesText(w io.Writer, label string, nodes []core.QueryNode) {
	fmt.Fprintf(w, "%s:\n", label)
	if len(nodes) == 0 {
		fmt.Fprintln(w, "  （該当なし）")
	}
	for _, node := range nodes {
		writeQueryNodeText(w, node.NodeInfo, "  ")
		writeQueryPreviewText(w, node, "    ")
	}
}

func writeQueryNodeText(w io.Writer, node core.NodeInfo, indent string) {
	identifier := node.Name
	if node.Type == core.NodeTypeNote || node.Type == core.NodeTypeAsset {
		identifier = node.Path
	}
	quoted, _ := json.Marshal(identifier)
	fmt.Fprintf(w, "%s%s %s\n", indent, node.Type, quoted)
}

func writeQueryPageNumber(w io.Writer, label string, value *int) {
	if value == nil {
		fmt.Fprintf(w, "  %s: null\n", label)
		return
	}
	fmt.Fprintf(w, "  %s: %d\n", label, *value)
}

type jsonQueryNode struct {
	jsonNodeInfo
	Head    *[]string           `json:"head,omitempty"`
	Snippet *[]jsonQuerySnippet `json:"snippet,omitempty"`
}
type jsonQuerySnippet struct {
	SourcePath string   `json:"source_path"`
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
	Lines      []string `json:"lines"`
}

func toJSONQueryNode(node core.QueryNode) jsonQueryNode {
	out := jsonQueryNode{jsonNodeInfo: toJSONNodeInfo(node.NodeInfo)}
	if node.Head != nil {
		out.Head = &node.Head
	}
	if node.Snippet != nil {
		snippets := make([]jsonQuerySnippet, len(node.Snippet))
		for i, s := range node.Snippet {
			snippets[i] = jsonQuerySnippet{s.SourcePath, s.LineStart, s.LineEnd, s.Lines}
		}
		out.Snippet = &snippets
	}
	return out
}
func writeQueryPreviewText(w io.Writer, node core.QueryNode, indent string) {
	if node.Head != nil {
		if len(node.Head) == 0 {
			fmt.Fprintf(w, "%shead: []\n", indent)
		} else {
			fmt.Fprintf(w, "%shead:\n", indent)
			for _, line := range node.Head {
				quoted, _ := json.Marshal(line)
				fmt.Fprintf(w, "%s  %s\n", indent, quoted)
			}
		}
	}
	if node.Snippet != nil {
		if len(node.Snippet) == 0 {
			fmt.Fprintf(w, "%ssnippet: []\n", indent)
		} else {
			fmt.Fprintf(w, "%ssnippet:\n", indent)
			for _, s := range node.Snippet {
				quoted, _ := json.Marshal(s.SourcePath)
				fmt.Fprintf(w, "%s  source_path: %s\n%s  start_line: %d\n%s  end_line: %d\n%s  lines:\n", indent, quoted, indent, s.LineStart, indent, s.LineEnd, indent)
				for _, line := range s.Lines {
					quoted, _ := json.Marshal(line)
					fmt.Fprintf(w, "%s    %s\n", indent, quoted)
				}
			}
		}
	}
}
