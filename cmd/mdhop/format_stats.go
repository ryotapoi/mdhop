package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

var validStatsFieldsCLI = map[string]bool{
	core.FieldStatsNotesTotal:    true,
	core.FieldStatsNotesExists:   true,
	core.FieldStatsEdgesTotal:    true,
	core.FieldStatsTagsTotal:     true,
	core.FieldStatsPhantomsTotal: true,
	core.FieldStatsAssetsTotal:   true,
}

func printStatsJSON(w io.Writer, r *core.StatsResult, fields []string) error {
	show := fieldSet(fields, validStatsFieldsCLI)
	m := make(map[string]int)
	if show[core.FieldStatsNotesTotal] {
		m[core.FieldStatsNotesTotal] = r.NotesTotal
	}
	if show[core.FieldStatsNotesExists] {
		m[core.FieldStatsNotesExists] = r.NotesExists
	}
	if show[core.FieldStatsEdgesTotal] {
		m[core.FieldStatsEdgesTotal] = r.EdgesTotal
	}
	if show[core.FieldStatsTagsTotal] {
		m[core.FieldStatsTagsTotal] = r.TagsTotal
	}
	if show[core.FieldStatsPhantomsTotal] {
		m[core.FieldStatsPhantomsTotal] = r.PhantomsTotal
	}
	if show[core.FieldStatsAssetsTotal] {
		m[core.FieldStatsAssetsTotal] = r.AssetsTotal
	}
	return encodeJSON(w, m)
}

func printStatsText(w io.Writer, r *core.StatsResult, fields []string) error {
	show := fieldSet(fields, validStatsFieldsCLI)
	if show[core.FieldStatsNotesTotal] {
		fmt.Fprintf(w, "%s: %d\n", core.FieldStatsNotesTotal, r.NotesTotal)
	}
	if show[core.FieldStatsNotesExists] {
		fmt.Fprintf(w, "%s: %d\n", core.FieldStatsNotesExists, r.NotesExists)
	}
	if show[core.FieldStatsEdgesTotal] {
		fmt.Fprintf(w, "%s: %d\n", core.FieldStatsEdgesTotal, r.EdgesTotal)
	}
	if show[core.FieldStatsTagsTotal] {
		fmt.Fprintf(w, "%s: %d\n", core.FieldStatsTagsTotal, r.TagsTotal)
	}
	if show[core.FieldStatsPhantomsTotal] {
		fmt.Fprintf(w, "%s: %d\n", core.FieldStatsPhantomsTotal, r.PhantomsTotal)
	}
	if show[core.FieldStatsAssetsTotal] {
		fmt.Fprintf(w, "%s: %d\n", core.FieldStatsAssetsTotal, r.AssetsTotal)
	}
	return nil
}
