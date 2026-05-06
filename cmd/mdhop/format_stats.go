package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

var validStatsFieldsCLI = map[string]bool{
	"notes_total":    true,
	"notes_exists":   true,
	"edges_total":    true,
	"tags_total":     true,
	"phantoms_total": true,
	"assets_total":   true,
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
	if show["assets_total"] {
		m["assets_total"] = r.AssetsTotal
	}
	return encodeJSON(w, m)
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
	if show["assets_total"] {
		fmt.Fprintf(w, "assets_total: %d\n", r.AssetsTotal)
	}
	return nil
}
