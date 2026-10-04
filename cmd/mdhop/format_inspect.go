package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/ryotapoi/mdhop/internal/core"
)

type inspectJSONOutput struct {
	Entry jsonNodeInfo         `json:"entry"`
	Tags  *[]string            `json:"tags,omitempty"`
	Meta  *map[string][]string `json:"meta,omitempty"`
	Head  *[]string            `json:"head,omitempty"`
}

func printInspectJSON(w io.Writer, r *core.InspectResult) error {
	out := inspectJSONOutput{Entry: toJSONNodeInfo(r.Entry)}
	if r.Tags != nil {
		out.Tags = &r.Tags
	}
	if r.Meta != nil {
		out.Meta = &r.Meta
	}
	if r.Head != nil {
		out.Head = &r.Head
	}
	return encodeJSON(w, out)
}

func printInspectText(w io.Writer, r *core.InspectResult) error {
	output := &textErrorWriter{w: w}
	w = output
	fmt.Fprintln(w, "entry:")
	writeQueryNodeText(w, r.Entry, "  ")
	if r.Tags != nil {
		if len(r.Tags) == 0 {
			fmt.Fprintln(w, "tags: []")
		} else {
			fmt.Fprintln(w, "tags:")
			for _, tag := range r.Tags {
				fmt.Fprintf(w, "  tag %s\n", strconv.Quote(tag))
			}
		}
	}
	if r.Meta != nil {
		if len(r.Meta) == 0 {
			fmt.Fprintln(w, "meta: {}")
		} else {
			fmt.Fprintln(w, "meta:")
			keys := make([]string, 0, len(r.Meta))
			for key := range r.Meta {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				for _, value := range r.Meta[key] {
					fmt.Fprintf(w, "  %s: %s\n", strconv.Quote(key), strconv.Quote(value))
				}
			}
		}
	}
	if r.Head != nil {
		if len(r.Head) == 0 {
			fmt.Fprintln(w, "head: []")
		} else {
			fmt.Fprintln(w, "head:")
			for _, line := range r.Head {
				fmt.Fprintf(w, "  %s\n", strconv.Quote(line))
			}
		}
	}
	return output.err
}
