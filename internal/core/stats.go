package core

import (
	"fmt"
	"os"
)

// StatsOptions controls which fields to return.
type StatsOptions struct {
	Fields []string // nil/empty = all
}

// StatsResult contains vault statistics.
type StatsResult struct {
	NotesTotal    int
	NotesExists   int
	EdgesTotal    int
	TagsTotal     int
	PhantomsTotal int
}

// Stats returns aggregate statistics for the indexed vault.
func Stats(vaultPath string, opts StatsOptions) (*StatsResult, error) {
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result := &StatsResult{}

	if isFieldActive("notes_total", opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type='note'`).Scan(&result.NotesTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive("notes_exists", opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type='note' AND exists_flag=1`).Scan(&result.NotesExists); err != nil {
			return nil, err
		}
	}

	if isFieldActive("edges_total", opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM edges`).Scan(&result.EdgesTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive("tags_total", opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type='tag'`).Scan(&result.TagsTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive("phantoms_total", opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type='phantom'`).Scan(&result.PhantomsTotal); err != nil {
			return nil, err
		}
	}

	return result, nil
}
