package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

const buildIndexHint = "hint: run 'mdhop build' to create or update the index"

func emptyIfNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

// printWarnings writes meta normalization warnings to stderr.
func printWarnings(warnings []string) {
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
}

// parseFields splits a comma-separated field string into a slice.
// Returns nil for empty input.
func parseFields(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// validateFormat checks that format is "json" or "text".
func validateFormat(format string) error {
	if format != "json" && format != "text" {
		return fmt.Errorf("invalid format: %q (must be json or text)", format)
	}
	return nil
}

// validateFields checks that all fields are in the valid set.
// name is used in the error message (e.g. "resolve", "query").
func validateFields(fields []string, valid map[string]bool, name string) error {
	for _, f := range fields {
		if !valid[f] {
			return fmt.Errorf("unknown %s field: %s", name, f)
		}
	}
	return nil
}

// fieldSet returns a set of fields to show. If fields is nil/empty, all valid fields are shown.
func fieldSet(fields []string, valid map[string]bool) map[string]bool {
	if len(fields) == 0 {
		all := make(map[string]bool)
		for k := range valid {
			all[k] = true
		}
		return all
	}
	m := make(map[string]bool, len(fields))
	for _, f := range fields {
		m[f] = true
	}
	return m
}

// printStringListText writes a labeled list as YAML-ish text.
func printStringListText(w io.Writer, label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", label)
	for _, item := range items {
		fmt.Fprintf(w, "- %s\n", item)
	}
}

// encodeJSON writes v as indented JSON to w.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// --- Shared NodeInfo JSON form (used by query and search) ---

type jsonNodeInfo struct {
	Type   core.NodeType `json:"type"`
	Name   string        `json:"name"`
	Path   string        `json:"path,omitempty"`
	Exists *bool         `json:"exists,omitempty"`
}

func toJSONNodeInfo(n core.NodeInfo) jsonNodeInfo {
	ji := jsonNodeInfo{Type: n.Type, Name: n.Name}
	if n.Type == core.NodeTypeNote || n.Type == core.NodeTypeAsset {
		ji.Path = n.Path
		ji.Exists = &n.Exists
	}
	return ji
}

// --- Mutation shared output (delete/update/add/move/disambiguate/convert/repair/simplify) ---

// rewrittenJSON is the JSON-serializable form of RewrittenLink.
type rewrittenJSON struct {
	File string `json:"file"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

func toRewrittenJSON(rls []core.RewrittenLink) []rewrittenJSON {
	out := make([]rewrittenJSON, len(rls))
	for i, r := range rls {
		out[i] = rewrittenJSON{File: r.File, Old: r.OldLink, New: r.NewLink}
	}
	return out
}

func printRewrittenText(w io.Writer, rls []core.RewrittenLink) {
	if len(rls) == 0 {
		return
	}
	fmt.Fprintln(w, "rewritten:")
	for _, r := range rls {
		fmt.Fprintf(w, "- file: %s\n", r.File)
		fmt.Fprintf(w, "  old: %q\n", r.OldLink)
		fmt.Fprintf(w, "  new: %q\n", r.NewLink)
	}
}

// --- Repair / Simplify shared helpers ---

type skippedJSON struct {
	File       string   `json:"file"`
	RawLink    string   `json:"raw_link"`
	Basename   string   `json:"basename"`
	Candidates []string `json:"candidates"`
}

func printSkippedText(w io.Writer, sls []core.SkippedLink) {
	if len(sls) == 0 {
		return
	}
	fmt.Fprintln(w, "skipped:")
	for _, s := range sls {
		fmt.Fprintf(w, "- file: %s\n", s.File)
		fmt.Fprintf(w, "  raw_link: %q\n", s.RawLink)
		fmt.Fprintf(w, "  basename: %s\n", s.Basename)
		fmt.Fprintln(w, "  candidates:")
		for _, c := range s.Candidates {
			fmt.Fprintf(w, "  - %s\n", c)
		}
	}
}

func toSkippedJSON(sls []core.SkippedLink) []skippedJSON {
	out := make([]skippedJSON, len(sls))
	for i, s := range sls {
		out[i] = skippedJSON{
			File:       s.File,
			RawLink:    s.RawLink,
			Basename:   s.Basename,
			Candidates: s.Candidates,
		}
	}
	return out
}

type rewriteResultJSONOutput struct {
	Rewritten []rewrittenJSON `json:"rewritten"`
	Skipped   []skippedJSON   `json:"skipped"`
}

func printRewriteResultJSON(w io.Writer, rewritten []core.RewrittenLink, skipped []core.SkippedLink) error {
	out := rewriteResultJSONOutput{
		Rewritten: emptyIfNil(toRewrittenJSON(rewritten)),
		Skipped:   emptyIfNil(toSkippedJSON(skipped)),
	}
	return encodeJSON(w, out)
}
