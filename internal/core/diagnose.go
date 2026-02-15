package core

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// DiagnoseOptions controls which fields to return.
type DiagnoseOptions struct {
	Fields []string // nil/empty = all
}

// BasenameConflict represents a group of notes with the same case-insensitive basename.
type BasenameConflict struct {
	Name  string   // display name (from the first path in sorted order)
	Paths []string // vault-relative paths (sorted)
}

// DiagnoseResult contains diagnostic information about the indexed vault.
type DiagnoseResult struct {
	BasenameConflicts []BasenameConflict // sorted by name
	Phantoms          []string           // sorted by name
}

// Diagnose returns diagnostic information for the indexed vault.
func Diagnose(vaultPath string, opts DiagnoseOptions) (*DiagnoseResult, error) {
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result := &DiagnoseResult{}

	if isFieldActive("basename_conflicts", opts.Fields) {
		rows, err := db.Query(`SELECT name, path FROM nodes WHERE type='note' AND exists_flag=1 ORDER BY LOWER(name), path`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		type noteEntry struct {
			name string
			path string
		}

		// Group by lowercase name
		groups := make(map[string][]noteEntry)
		var order []string
		for rows.Next() {
			var name, path string
			if err := rows.Scan(&name, &path); err != nil {
				return nil, err
			}
			key := strings.ToLower(name)
			if _, exists := groups[key]; !exists {
				order = append(order, key)
			}
			groups[key] = append(groups[key], noteEntry{name: name, path: path})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		sort.Strings(order)
		for _, key := range order {
			entries := groups[key]
			if len(entries) < 2 {
				continue
			}
			paths := make([]string, len(entries))
			for i, e := range entries {
				paths[i] = e.path
			}
			// Name is from the first entry (paths are already sorted by SQL ORDER BY)
			result.BasenameConflicts = append(result.BasenameConflicts, BasenameConflict{
				Name:  entries[0].name,
				Paths: paths,
			})
		}
	}

	if isFieldActive("phantoms", opts.Fields) {
		rows, err := db.Query(`SELECT name FROM nodes WHERE type='phantom' ORDER BY name`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			result.Phantoms = append(result.Phantoms, name)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return result, nil
}
