package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

var validDiagnoseFieldsCLI = map[string]bool{
	"basename_conflicts":       true,
	"asset_basename_conflicts": true,
	"phantoms":                 true,
	"anchors":                  true,
}

// anchorsRequested reports whether anchor checking was explicitly requested.
// Unlike the other fields, anchors is opt-in (not shown when --fields is
// omitted) because it reads target notes from disk.
func anchorsRequested(fields []string) bool {
	for _, f := range fields {
		if f == "anchors" {
			return true
		}
	}
	return false
}

type diagnoseJSONConflict struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

type diagnoseJSONBrokenAnchor struct {
	SourcePath string `json:"source_path"`
	RawLink    string `json:"raw_link"`
	TargetPath string `json:"target_path"`
	Fragment   string `json:"fragment"`
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
	if show["asset_basename_conflicts"] {
		conflicts := make([]diagnoseJSONConflict, len(r.AssetBasenameConflicts))
		for i, c := range r.AssetBasenameConflicts {
			conflicts[i] = diagnoseJSONConflict{Name: c.Name, Paths: c.Paths}
		}
		m["asset_basename_conflicts"] = conflicts
	}
	if show["phantoms"] {
		if r.Phantoms != nil {
			m["phantoms"] = r.Phantoms
		} else {
			m["phantoms"] = []string{}
		}
	}
	if anchorsRequested(fields) {
		anchors := make([]diagnoseJSONBrokenAnchor, len(r.BrokenAnchors))
		for i, a := range r.BrokenAnchors {
			anchors[i] = diagnoseJSONBrokenAnchor{
				SourcePath: a.SourcePath,
				RawLink:    a.RawLink,
				TargetPath: a.TargetPath,
				Fragment:   a.Fragment,
			}
		}
		m["broken_anchors"] = anchors
	}
	return encodeJSON(w, m)
}

func printDiagnoseText(w io.Writer, r *core.DiagnoseResult, fields []string) error {
	show := fieldSet(fields, validDiagnoseFieldsCLI)
	if show["basename_conflicts"] && len(r.BasenameConflicts) > 0 {
		fmt.Fprintln(w, "basename_conflicts:")
		for _, c := range r.BasenameConflicts {
			fmt.Fprintf(w, "- name: %s\n", c.Name)
			fmt.Fprintln(w, "  paths:")
			for _, p := range c.Paths {
				fmt.Fprintf(w, "  - %s\n", p)
			}
		}
	}
	if show["asset_basename_conflicts"] && len(r.AssetBasenameConflicts) > 0 {
		fmt.Fprintln(w, "asset_basename_conflicts:")
		for _, c := range r.AssetBasenameConflicts {
			fmt.Fprintf(w, "- name: %s\n", c.Name)
			fmt.Fprintln(w, "  paths:")
			for _, p := range c.Paths {
				fmt.Fprintf(w, "  - %s\n", p)
			}
		}
	}
	if show["phantoms"] && len(r.Phantoms) > 0 {
		fmt.Fprintln(w, "phantoms:")
		for _, name := range r.Phantoms {
			fmt.Fprintf(w, "- %s\n", name)
		}
	}
	if anchorsRequested(fields) && len(r.BrokenAnchors) > 0 {
		fmt.Fprintln(w, "broken_anchors:")
		for _, a := range r.BrokenAnchors {
			fmt.Fprintf(w, "- source_path: %s\n", a.SourcePath)
			fmt.Fprintf(w, "  raw_link: %q\n", a.RawLink)
			fmt.Fprintf(w, "  target_path: %s\n", a.TargetPath)
			fmt.Fprintf(w, "  fragment: %s\n", a.Fragment)
		}
	}
	return nil
}
