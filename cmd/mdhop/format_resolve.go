package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

var validResolveFields = map[string]bool{
	core.FieldResolveType:    true,
	core.FieldResolveName:    true,
	core.FieldResolvePath:    true,
	core.FieldResolveExists:  true,
	core.FieldResolveSubpath: true,
}

func printResolveJSON(w io.Writer, r *core.ResolveResult, fields []string) error {
	return encodeJSON(w, buildResolveMap(r, fields))
}

func printResolveText(w io.Writer, r *core.ResolveResult, fields []string) error {
	show := fieldSet(fields, validResolveFields)

	if show[core.FieldResolveType] {
		fmt.Fprintf(w, "%s: %s\n", core.FieldResolveType, r.Type)
	}
	if show[core.FieldResolveName] {
		fmt.Fprintf(w, "%s: %s\n", core.FieldResolveName, r.Name)
	}
	if show[core.FieldResolvePath] && (r.Type == core.NodeTypeNote || r.Type == core.NodeTypeAsset) {
		fmt.Fprintf(w, "%s: %s\n", core.FieldResolvePath, r.Path)
	}
	if show[core.FieldResolveExists] && (r.Type == core.NodeTypeNote || r.Type == core.NodeTypeAsset) {
		fmt.Fprintf(w, "%s: %v\n", core.FieldResolveExists, r.Exists)
	}
	if show[core.FieldResolveSubpath] && r.Subpath != "" {
		fmt.Fprintf(w, "%s: %s\n", core.FieldResolveSubpath, r.Subpath)
	}
	return nil
}

func buildResolveMap(r *core.ResolveResult, fields []string) map[string]any {
	show := fieldSet(fields, validResolveFields)
	m := make(map[string]any)
	if show[core.FieldResolveType] {
		m[core.FieldResolveType] = r.Type
	}
	if show[core.FieldResolveName] {
		m[core.FieldResolveName] = r.Name
	}
	if show[core.FieldResolvePath] && (r.Type == core.NodeTypeNote || r.Type == core.NodeTypeAsset) {
		m[core.FieldResolvePath] = r.Path
	}
	if show[core.FieldResolveExists] && (r.Type == core.NodeTypeNote || r.Type == core.NodeTypeAsset) {
		m[core.FieldResolveExists] = r.Exists
	}
	if show[core.FieldResolveSubpath] && r.Subpath != "" {
		m[core.FieldResolveSubpath] = r.Subpath
	}
	return m
}
