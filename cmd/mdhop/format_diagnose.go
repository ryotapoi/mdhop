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
	return nil
}
