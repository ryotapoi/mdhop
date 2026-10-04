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
	Entry     jsonNodeInfo    `json:"entry"`
	Backlinks *[]jsonNodeInfo `json:"backlinks,omitempty"`
	Outgoing  *[]jsonNodeInfo `json:"outgoing,omitempty"`
	TwoHop    *[]jsonTwoHop   `json:"2hoplink,omitempty"`
	Page      jsonQueryPage   `json:"page"`
}

type jsonTwoHop struct {
	jsonNodeInfo
	Relation       []jsonNodeInfo `json:"relation"`
	HiddenRelation bool           `json:"hidden_relation"`
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
		items := make([]jsonNodeInfo, len(r.Backlinks))
		for i, node := range r.Backlinks {
			items[i] = toJSONNodeInfo(node)
		}
		out.Backlinks = &items
	}
	if r.Outgoing != nil {
		items := make([]jsonNodeInfo, len(r.Outgoing))
		for i, node := range r.Outgoing {
			items[i] = toJSONNodeInfo(node)
		}
		out.Outgoing = &items
	}
	if r.TwoHop != nil {
		items := make([]jsonTwoHop, len(r.TwoHop))
		for i, target := range r.TwoHop {
			relations := make([]jsonNodeInfo, len(target.Relation))
			for j, via := range target.Relation {
				relations[j] = toJSONNodeInfo(via)
			}
			items[i] = jsonTwoHop{
				jsonNodeInfo:   toJSONNodeInfo(target.NodeInfo),
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
			fmt.Fprintln(w, "    relation:")
			for _, via := range target.Relation {
				writeQueryNodeText(w, via, "      ")
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

func writeQueryNodesText(w io.Writer, label string, nodes []core.NodeInfo) {
	fmt.Fprintf(w, "%s:\n", label)
	if len(nodes) == 0 {
		fmt.Fprintln(w, "  （該当なし）")
	}
	for _, node := range nodes {
		writeQueryNodeText(w, node, "  ")
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
