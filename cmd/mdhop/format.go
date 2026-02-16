package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

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

var validResolveFields = map[string]bool{
	"type":    true,
	"name":    true,
	"path":    true,
	"exists":  true,
	"subpath": true,
}

// --- Resolve output ---

func printResolveJSON(w io.Writer, r *core.ResolveResult, fields []string) error {
	m := buildResolveMap(r, fields)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

func printResolveText(w io.Writer, r *core.ResolveResult, fields []string) error {
	show := fieldSet(fields, validResolveFields)

	if show["type"] {
		fmt.Fprintf(w, "type: %s\n", r.Type)
	}
	if show["name"] {
		fmt.Fprintf(w, "name: %s\n", r.Name)
	}
	if show["path"] && r.Path != "" {
		fmt.Fprintf(w, "path: %s\n", r.Path)
	}
	if show["exists"] {
		fmt.Fprintf(w, "exists: %v\n", r.Exists)
	}
	if show["subpath"] && r.Subpath != "" {
		fmt.Fprintf(w, "subpath: %s\n", r.Subpath)
	}
	return nil
}

func buildResolveMap(r *core.ResolveResult, fields []string) map[string]any {
	show := fieldSet(fields, validResolveFields)
	m := make(map[string]any)
	if show["type"] {
		m["type"] = r.Type
	}
	if show["name"] {
		m["name"] = r.Name
	}
	if show["path"] && r.Path != "" {
		m["path"] = r.Path
	}
	if show["exists"] {
		m["exists"] = r.Exists
	}
	if show["subpath"] && r.Subpath != "" {
		m["subpath"] = r.Subpath
	}
	return m
}

// --- Stats output ---

var validStatsFieldsCLI = map[string]bool{
	"notes_total":    true,
	"notes_exists":   true,
	"edges_total":    true,
	"tags_total":     true,
	"phantoms_total": true,
}

func printStatsJSON(w io.Writer, r *core.StatsResult, fields []string) error {
	show := fieldSet(fields, validStatsFieldsCLI)
	m := make(map[string]int)
	if show["notes_total"] {
		m["notes_total"] = r.NotesTotal
	}
	if show["notes_exists"] {
		m["notes_exists"] = r.NotesExists
	}
	if show["edges_total"] {
		m["edges_total"] = r.EdgesTotal
	}
	if show["tags_total"] {
		m["tags_total"] = r.TagsTotal
	}
	if show["phantoms_total"] {
		m["phantoms_total"] = r.PhantomsTotal
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

func printStatsText(w io.Writer, r *core.StatsResult, fields []string) error {
	show := fieldSet(fields, validStatsFieldsCLI)
	if show["notes_total"] {
		fmt.Fprintf(w, "notes_total: %d\n", r.NotesTotal)
	}
	if show["notes_exists"] {
		fmt.Fprintf(w, "notes_exists: %d\n", r.NotesExists)
	}
	if show["edges_total"] {
		fmt.Fprintf(w, "edges_total: %d\n", r.EdgesTotal)
	}
	if show["tags_total"] {
		fmt.Fprintf(w, "tags_total: %d\n", r.TagsTotal)
	}
	if show["phantoms_total"] {
		fmt.Fprintf(w, "phantoms_total: %d\n", r.PhantomsTotal)
	}
	return nil
}

// --- Diagnose output ---

var validDiagnoseFieldsCLI = map[string]bool{
	"basename_conflicts": true,
	"phantoms":           true,
}

type diagnoseJSONConflict struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

func printDiagnoseJSON(w io.Writer, r *core.DiagnoseResult, fields []string) error {
	show := fieldSet(fields, validDiagnoseFieldsCLI)
	m := make(map[string]any)
	if show["basename_conflicts"] {
		conflicts := make([]diagnoseJSONConflict, len(r.BasenameConflicts))
		for i, c := range r.BasenameConflicts {
			conflicts[i] = diagnoseJSONConflict{Name: c.Name, Paths: c.Paths}
		}
		m["basename_conflicts"] = conflicts
	}
	if show["phantoms"] {
		if r.Phantoms != nil {
			m["phantoms"] = r.Phantoms
		} else {
			m["phantoms"] = []string{}
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

func printDiagnoseText(w io.Writer, r *core.DiagnoseResult, fields []string) error {
	show := fieldSet(fields, validDiagnoseFieldsCLI)
	if show["basename_conflicts"] {
		fmt.Fprintln(w, "basename_conflicts:")
		for _, c := range r.BasenameConflicts {
			fmt.Fprintf(w, "- name: %s\n", c.Name)
			fmt.Fprintln(w, "  paths:")
			for _, p := range c.Paths {
				fmt.Fprintf(w, "  - %s\n", p)
			}
		}
	}
	if show["phantoms"] {
		fmt.Fprintln(w, "phantoms:")
		for _, name := range r.Phantoms {
			fmt.Fprintf(w, "- %s\n", name)
		}
	}
	return nil
}

var validQueryFieldsCLI = map[string]bool{
	"backlinks": true,
	"tags":      true,
	"twohop":    true,
	"outgoing":  true,
	"head":      true,
	"snippet":   true,
}

// --- Query output ---

// queryJSONOutput is the JSON-serializable form of QueryResult.
type queryJSONOutput struct {
	Entry     *jsonNodeInfo    `json:"entry"`
	Backlinks []jsonNodeInfo   `json:"backlinks,omitempty"`
	Outgoing  []jsonNodeInfo   `json:"outgoing,omitempty"`
	Tags      []string         `json:"tags,omitempty"`
	TwoHop    []jsonTwoHop     `json:"twohop,omitempty"`
	Head      []string         `json:"head,omitempty"`
	Snippets  []jsonSnippet    `json:"snippet,omitempty"`
}

type jsonNodeInfo struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Path   string `json:"path,omitempty"`
	Exists *bool  `json:"exists,omitempty"`
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

func toJSONNodeInfo(n core.NodeInfo) jsonNodeInfo {
	ji := jsonNodeInfo{Type: n.Type, Name: n.Name}
	if n.Type == "note" {
		ji.Path = n.Path
		ji.Exists = &n.Exists
	}
	return ji
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

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printQueryText(w io.Writer, r *core.QueryResult) error {
	// entry (always present)
	fmt.Fprintln(w, "entry:")
	writeNodeInfoText(w, r.Entry, "  ", "  ")

	if r.Backlinks != nil {
		fmt.Fprintln(w, "backlinks:")
		for _, n := range r.Backlinks {
			writeNodeInfoText(w, n, "- ", "  ")
		}
	}

	if r.Outgoing != nil {
		fmt.Fprintln(w, "outgoing:")
		for _, n := range r.Outgoing {
			writeNodeInfoText(w, n, "- ", "  ")
		}
	}

	if r.Tags != nil {
		fmt.Fprintln(w, "tags:")
		for _, t := range r.Tags {
			fmt.Fprintf(w, "- %s\n", t)
		}
	}

	if r.TwoHop != nil {
		fmt.Fprintln(w, "twohop:")
		for _, th := range r.TwoHop {
			fmt.Fprintf(w, "- via: %s\n", nodeInfoOneLine(th.Via))
			fmt.Fprintln(w, "  targets:")
			for _, t := range th.Targets {
				fmt.Fprintf(w, "  - %s\n", nodeInfoOneLine(t))
			}
		}
	}

	if r.Head != nil {
		fmt.Fprintln(w, "head:")
		for _, line := range r.Head {
			fmt.Fprintf(w, "- %q\n", line)
		}
	}

	if r.Snippets != nil {
		fmt.Fprintln(w, "snippet:")
		for _, s := range r.Snippets {
			fmt.Fprintf(w, "- source: %s\n", s.SourcePath)
			fmt.Fprintf(w, "  lines: %d-%d\n", s.LineStart, s.LineEnd)
			fmt.Fprintln(w, "  content:")
			for _, line := range s.Lines {
				fmt.Fprintf(w, "  - %q\n", line)
			}
		}
	}

	return nil
}

// writeNodeInfoText writes a NodeInfo in multi-line text format.
// firstIndent is the indent for the first line (type:), restIndent for subsequent lines.
func writeNodeInfoText(w io.Writer, n core.NodeInfo, firstIndent, restIndent string) {
	fmt.Fprintf(w, "%stype: %s\n", firstIndent, n.Type)
	fmt.Fprintf(w, "%sname: %s\n", restIndent, n.Name)
	if n.Type == "note" {
		fmt.Fprintf(w, "%spath: %s\n", restIndent, n.Path)
		fmt.Fprintf(w, "%sexists: %v\n", restIndent, n.Exists)
	}
}

// nodeInfoOneLine returns a compact one-line representation for twohop via/targets.
// Format: "note: path" or "phantom: name" or "tag: name"
func nodeInfoOneLine(n core.NodeInfo) string {
	switch n.Type {
	case "note":
		return fmt.Sprintf("note: %s", n.Path)
	default:
		return fmt.Sprintf("%s: %s", n.Type, n.Name)
	}
}
