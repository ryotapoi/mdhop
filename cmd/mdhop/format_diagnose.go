package main

import (
	"fmt"
	"io"
	"slices"

	"github.com/ryotapoi/mdhop/internal/core"
)

var validDiagnoseFieldsCLI = map[string]bool{
	core.FieldDiagnoseBasenameConflicts:      true,
	core.FieldDiagnoseAssetBasenameConflicts: true,
	core.FieldDiagnosePhantoms:               true,
	core.FieldDiagnoseAnchors:                true,
}

// anchorsRequested reports whether anchor checking was explicitly requested.
// Unlike the other fields, anchors is opt-in (not shown when --fields is
// omitted) because it reads target notes from disk.
func anchorsRequested(fields []string) bool {
	return slices.Contains(fields, core.FieldDiagnoseAnchors)
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
	if show[core.FieldDiagnoseBasenameConflicts] {
		conflicts := make([]diagnoseJSONConflict, len(r.BasenameConflicts))
		for i, c := range r.BasenameConflicts {
			conflicts[i] = diagnoseJSONConflict{Name: c.Name, Paths: c.Paths}
		}
		m[core.FieldDiagnoseBasenameConflicts] = conflicts
	}
	if show[core.FieldDiagnoseAssetBasenameConflicts] {
		conflicts := make([]diagnoseJSONConflict, len(r.AssetBasenameConflicts))
		for i, c := range r.AssetBasenameConflicts {
			conflicts[i] = diagnoseJSONConflict{Name: c.Name, Paths: c.Paths}
		}
		m[core.FieldDiagnoseAssetBasenameConflicts] = conflicts
	}
	if show[core.FieldDiagnosePhantoms] {
		m[core.FieldDiagnosePhantoms] = emptyIfNil(r.Phantoms)
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
		m[core.FieldDiagnoseBrokenAnchors] = anchors
	}
	return encodeJSON(w, m)
}

func printDiagnoseText(w io.Writer, r *core.DiagnoseResult, fields []string) error {
	show := fieldSet(fields, validDiagnoseFieldsCLI)
	if show[core.FieldDiagnoseBasenameConflicts] && len(r.BasenameConflicts) > 0 {
		fmt.Fprintf(w, "%s:\n", core.FieldDiagnoseBasenameConflicts)
		for _, c := range r.BasenameConflicts {
			fmt.Fprintf(w, "- name: %s\n", c.Name)
			fmt.Fprintln(w, "  paths:")
			for _, p := range c.Paths {
				fmt.Fprintf(w, "  - %s\n", p)
			}
		}
	}
	if show[core.FieldDiagnoseAssetBasenameConflicts] && len(r.AssetBasenameConflicts) > 0 {
		fmt.Fprintf(w, "%s:\n", core.FieldDiagnoseAssetBasenameConflicts)
		for _, c := range r.AssetBasenameConflicts {
			fmt.Fprintf(w, "- name: %s\n", c.Name)
			fmt.Fprintln(w, "  paths:")
			for _, p := range c.Paths {
				fmt.Fprintf(w, "  - %s\n", p)
			}
		}
	}
	if show[core.FieldDiagnosePhantoms] && len(r.Phantoms) > 0 {
		fmt.Fprintf(w, "%s:\n", core.FieldDiagnosePhantoms)
		for _, name := range r.Phantoms {
			fmt.Fprintf(w, "- %s\n", name)
		}
	}
	if anchorsRequested(fields) && len(r.BrokenAnchors) > 0 {
		fmt.Fprintf(w, "%s:\n", core.FieldDiagnoseBrokenAnchors)
		for _, a := range r.BrokenAnchors {
			fmt.Fprintf(w, "- source_path: %s\n", a.SourcePath)
			fmt.Fprintf(w, "  raw_link: %q\n", a.RawLink)
			fmt.Fprintf(w, "  target_path: %s\n", a.TargetPath)
			fmt.Fprintf(w, "  fragment: %s\n", a.Fragment)
		}
	}
	return nil
}
