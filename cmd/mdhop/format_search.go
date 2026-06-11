package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

// validSearchComputedFields are the computed (non-meta) opt-in fields.
var validSearchComputedFields = map[string]bool{
	core.FieldLines:         true,
	core.FieldOutgoingCount: true,
	core.FieldIncomingCount: true,
}

const metaFieldPrefix = "meta."

// validateSearchFields accepts "meta" (all meta), "meta.<key>" (a single meta
// key), and the computed fields. Other names are rejected.
func validateSearchFields(fields []string) error {
	for _, f := range fields {
		if f == "meta" || validSearchComputedFields[f] {
			continue
		}
		if strings.HasPrefix(f, metaFieldPrefix) && len(f) > len(metaFieldPrefix) {
			continue
		}
		return fmt.Errorf("unknown search field: %s", f)
	}
	return nil
}

// searchMetaKeys returns the specific meta keys requested via "meta.<key>".
// all is true when "meta" (all keys) was requested.
func searchMetaKeys(fields []string) (keys []string, all bool) {
	for _, f := range fields {
		if f == "meta" {
			all = true
		} else if strings.HasPrefix(f, metaFieldPrefix) {
			keys = append(keys, f[len(metaFieldPrefix):])
		}
	}
	return keys, all
}

func fieldRequested(fields []string, name string) bool {
	for _, f := range fields {
		if f == name {
			return true
		}
	}
	return false
}

type searchJSONItem struct {
	jsonNodeInfo
	Lines         *int                `json:"lines,omitempty"`
	OutgoingCount *int                `json:"outgoing_count,omitempty"`
	IncomingCount *int                `json:"incoming_count,omitempty"`
	Meta          map[string][]string `json:"meta,omitempty"`
	Head          []string            `json:"head,omitempty"`
}

type searchJSONOutput struct {
	Total int              `json:"total"`
	Items []searchJSONItem `json:"items"`
}

func printSearchJSON(w io.Writer, r *core.SearchResult, fields []string) error {
	metaKeys, metaAll := searchMetaKeys(fields)
	wantLines := fieldRequested(fields, core.FieldLines)
	wantOut := fieldRequested(fields, core.FieldOutgoingCount)
	wantIn := fieldRequested(fields, core.FieldIncomingCount)

	items := make([]searchJSONItem, len(r.Items))
	for i, item := range r.Items {
		ji := searchJSONItem{jsonNodeInfo: toJSONNodeInfo(item.Node)}
		if wantLines {
			v := item.Lines
			ji.Lines = &v
		}
		if wantOut {
			v := item.OutgoingCount
			ji.OutgoingCount = &v
		}
		if wantIn {
			v := item.IncomingCount
			ji.IncomingCount = &v
		}
		if item.Meta != nil {
			m := make(map[string][]string)
			for _, mr := range item.Meta {
				if metaAll || containsString(metaKeys, mr.Key) {
					m[mr.Key] = append(m[mr.Key], mr.Value)
				}
			}
			if len(m) > 0 {
				ji.Meta = m
			}
		}
		if item.Head != nil {
			ji.Head = item.Head
		}
		items[i] = ji
	}
	out := searchJSONOutput{Total: r.Total, Items: items}
	return encodeJSON(w, out)
}

func printSearchText(w io.Writer, r *core.SearchResult, fields []string) error {
	metaKeys, metaAll := searchMetaKeys(fields)
	wantLines := fieldRequested(fields, core.FieldLines)
	wantOut := fieldRequested(fields, core.FieldOutgoingCount)
	wantIn := fieldRequested(fields, core.FieldIncomingCount)

	fmt.Fprintf(w, "total: %d\n", r.Total)
	if len(r.Items) > 0 {
		fmt.Fprintln(w, "items:")
		for _, item := range r.Items {
			fmt.Fprintf(w, "- %s: %s\n", item.Node.Type, item.Node.Path)
			if wantLines {
				fmt.Fprintf(w, "  lines: %d\n", item.Lines)
			}
			if wantOut {
				fmt.Fprintf(w, "  outgoing_count: %d\n", item.OutgoingCount)
			}
			if wantIn {
				fmt.Fprintf(w, "  incoming_count: %d\n", item.IncomingCount)
			}
			if item.Meta != nil && len(item.Meta) > 0 {
				printed := false
				for _, m := range item.Meta {
					if !metaAll && !containsString(metaKeys, m.Key) {
						continue
					}
					if !printed {
						fmt.Fprintln(w, "  meta:")
						printed = true
					}
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

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
