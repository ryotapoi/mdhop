package core

import (
	"sort"
	"strings"
)

// DiagnoseOptions controls which fields to return.
type DiagnoseOptions struct {
	Fields []string // nil/empty = all
}

// BasenameConflict represents a group of nodes with the same case-insensitive basename.
type BasenameConflict struct {
	Name  string   // display name (from the first path in sorted order)
	Paths []string // vault-relative paths (sorted)
}

// DiagnoseResult contains diagnostic information about the indexed vault.
type DiagnoseResult struct {
	BasenameConflicts      []BasenameConflict // sorted by name (notes)
	AssetBasenameConflicts []BasenameConflict // sorted by name (assets)
	Phantoms               []string           // sorted by name
}

// basenameConflicts returns groups of existing nodes of the given type that
// share the same case-insensitive basename.
func basenameConflicts(db dbExecer, nodeType NodeType) ([]BasenameConflict, error) {
	rows, err := db.Query(`SELECT name, path FROM nodes WHERE type=? AND exists_flag=1 ORDER BY LOWER(name), path`, nodeType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type entry struct {
		name string
		path string
	}

	// Group by lowercase name
	groups := make(map[string][]entry)
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
		groups[key] = append(groups[key], entry{name: name, path: path})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Strings(order)
	var conflicts []BasenameConflict
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
		conflicts = append(conflicts, BasenameConflict{
			Name:  entries[0].name,
			Paths: paths,
		})
	}
	return conflicts, nil
}

// Diagnose returns diagnostic information for the indexed vault.
func Diagnose(vaultPath string, opts DiagnoseOptions) (*DiagnoseResult, error) {
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result := &DiagnoseResult{}

	if isFieldActive("basename_conflicts", opts.Fields) {
		result.BasenameConflicts, err = basenameConflicts(db, NodeTypeNote)
		if err != nil {
			return nil, err
		}
	}

	if isFieldActive("asset_basename_conflicts", opts.Fields) {
		result.AssetBasenameConflicts, err = basenameConflicts(db, NodeTypeAsset)
		if err != nil {
			return nil, err
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
