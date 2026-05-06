package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

var validSearchFieldsCLI = map[string]bool{
	"meta": true,
}

type searchJSONItem struct {
	jsonNodeInfo
	Meta map[string][]string `json:"meta,omitempty"`
	Head []string            `json:"head,omitempty"`
}

type searchJSONOutput struct {
	Total int              `json:"total"`
	Items []searchJSONItem `json:"items"`
}

func printSearchJSON(w io.Writer, r *core.SearchResult) error {
	items := make([]searchJSONItem, len(r.Items))
	for i, item := range r.Items {
		ji := searchJSONItem{
			jsonNodeInfo: toJSONNodeInfo(item.Node),
		}
		if item.Meta != nil {
			m := make(map[string][]string)
			for _, mr := range item.Meta {
				m[mr.Key] = append(m[mr.Key], mr.Value)
			}
			ji.Meta = m
		}
		if item.Head != nil {
			ji.Head = item.Head
		}
		items[i] = ji
	}
	out := searchJSONOutput{
		Total: r.Total,
		Items: items,
	}
	return encodeJSON(w, out)
}

func printSearchText(w io.Writer, r *core.SearchResult) error {
	fmt.Fprintf(w, "total: %d\n", r.Total)
	if len(r.Items) > 0 {
		fmt.Fprintln(w, "items:")
		for _, item := range r.Items {
			fmt.Fprintf(w, "- %s: %s\n", item.Node.Type, item.Node.Path)
			if item.Meta != nil && len(item.Meta) > 0 {
				fmt.Fprintln(w, "  meta:")
				for _, m := range item.Meta {
					fmt.Fprintf(w, "  - %s: %s\n", m.Key, m.Value)
				}
			}
			if item.Head != nil && len(item.Head) > 0 {
				fmt.Fprintln(w, "  head:")
				for _, line := range item.Head {
					fmt.Fprintf(w, "  - %q\n", line)
				}
			}
		}
	}
	return nil
}
