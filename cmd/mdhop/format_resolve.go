package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

var validResolveFields = map[string]bool{
	"type":    true,
	"name":    true,
	"path":    true,
	"exists":  true,
	"subpath": true,
}

func printResolveJSON(w io.Writer, r *core.ResolveResult, fields []string) error {
	return encodeJSON(w, buildResolveMap(r, fields))
}

func printResolveText(w io.Writer, r *core.ResolveResult, fields []string) error {
	show := fieldSet(fields, validResolveFields)

	if show["type"] {
		fmt.Fprintf(w, "type: %s\n", r.Type)
	}
	if show["name"] {
		fmt.Fprintf(w, "name: %s\n", r.Name)
	}
	if show["path"] && (r.Type == core.NodeTypeNote || r.Type == core.NodeTypeAsset) {
		fmt.Fprintf(w, "path: %s\n", r.Path)
	}
	if show["exists"] && (r.Type == core.NodeTypeNote || r.Type == core.NodeTypeAsset) {
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
	if show["path"] && (r.Type == core.NodeTypeNote || r.Type == core.NodeTypeAsset) {
		m["path"] = r.Path
	}
	if show["exists"] && (r.Type == core.NodeTypeNote || r.Type == core.NodeTypeAsset) {
		m["exists"] = r.Exists
	}
	if show["subpath"] && r.Subpath != "" {
		m["subpath"] = r.Subpath
	}
	return m
}
