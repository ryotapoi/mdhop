package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiagnoseOptions controls which fields to return and how to filter sources.
type DiagnoseOptions struct {
	Fields []string // nil/empty = all
	// Path / Exclude filter source notes by path glob (CLI-only; independent
	// of mdhop.yaml exclude settings). When either is set, results are
	// restricted to problems in links written in the matching notes.
	Path    []string // include globs (empty = all notes)
	Exclude []string // exclude globs
}

// sourceFiltered reports whether a source note filter is in effect.
func (o DiagnoseOptions) sourceFiltered() bool {
	return len(o.Path) > 0 || len(o.Exclude) > 0
}

// BasenameConflict represents a group of nodes with the same case-insensitive basename.
type BasenameConflict struct {
	Name  string   // display name (from the first path in sorted order)
	Paths []string // vault-relative paths (sorted)
}

// BrokenAnchor represents a link whose heading fragment does not exist in the
// target note.
type BrokenAnchor struct {
	SourcePath string // note containing the link
	RawLink    string // the link as written
	TargetPath string // resolved target note path
	Fragment   string // the heading fragment that was not found (without '#')
}

// DiagnoseResult contains diagnostic information about the indexed vault.
type DiagnoseResult struct {
	BasenameConflicts      []BasenameConflict // sorted by name (notes)
	AssetBasenameConflicts []BasenameConflict // sorted by name (assets)
	Phantoms               []string           // sorted by name
	BrokenAnchors          []BrokenAnchor     // sorted by source path, then raw link
}

// sourceNoteFilterSQL returns a SQL fragment (starting with " AND") and args
// restricting source note paths (column "src.path") by the Path/Exclude globs.
func (o DiagnoseOptions) sourceNoteFilterSQL() (string, []any) {
	inclSQL, args := pathIncludeSQL("src.path", o.Path)
	ef := &ExcludeFilter{PathGlobs: o.Exclude}
	exclSQL, exclArgs := ef.PathExcludeSQL("src.path")
	return inclSQL + exclSQL, append(args, exclArgs...)
}

// basenameLinkTargets returns the set of node IDs referenced via basename-form
// links (wikilink/markdown without path separators) from notes matching the
// source path filter.
func basenameLinkTargets(db dbExecer, opts DiagnoseOptions) (map[int64]bool, error) {
	filterSQL, filterArgs := opts.sourceNoteFilterSQL()
	rows, err := db.Query(`SELECT DISTINCT e.target_id, e.raw_link, e.link_type FROM edges e
		JOIN nodes src ON src.id = e.source_id
		WHERE src.type='note' AND src.exists_flag=1`+filterSQL, filterArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make(map[int64]bool)
	for rows.Next() {
		var targetID int64
		var rawLink, linkType string
		if err := rows.Scan(&targetID, &rawLink, &linkType); err != nil {
			return nil, err
		}
		if isBasenameRawLink(rawLink, LinkType(linkType)) {
			targets[targetID] = true
		}
	}
	return targets, rows.Err()
}

// basenameConflicts returns groups of existing nodes of the given type that
// share the same case-insensitive basename. When allowed is non-nil, only
// groups containing at least one node referenced in allowed are returned
// (basename-link resolution risk for the filtered source notes).
func basenameConflicts(db dbExecer, nodeType NodeType, allowed map[int64]bool) ([]BasenameConflict, error) {
	rows, err := db.Query(`SELECT id, name, path FROM nodes WHERE type=? AND exists_flag=1 ORDER BY LOWER(name), path`, nodeType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type entry struct {
		id   int64
		name string
		path string
	}

	// Group by lowercase name
	groups := make(map[string][]entry)
	var order []string
	for rows.Next() {
		var id int64
		var name, path string
		if err := rows.Scan(&id, &name, &path); err != nil {
			return nil, err
		}
		key := strings.ToLower(name)
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], entry{id: id, name: name, path: path})
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
		if allowed != nil {
			referenced := false
			for _, e := range entries {
				if allowed[e.id] {
					referenced = true
					break
				}
			}
			if !referenced {
				continue
			}
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

// phantomNames returns phantom node names, optionally restricted to phantoms
// referenced from notes matching the source path filter.
func phantomNames(db dbExecer, opts DiagnoseOptions) ([]string, error) {
	query := `SELECT name FROM nodes WHERE type='phantom' ORDER BY name`
	var args []any
	if opts.sourceFiltered() {
		filterSQL, filterArgs := opts.sourceNoteFilterSQL()
		query = `SELECT DISTINCT n.name FROM nodes n
			JOIN edges e ON e.target_id = n.id
			JOIN nodes src ON src.id = e.source_id
			WHERE n.type='phantom' AND src.type='note' AND src.exists_flag=1` + filterSQL + ` ORDER BY n.name`
		args = filterArgs
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// brokenAnchors finds links whose heading fragment does not exist in the
// resolved target note. Only edges with a heading fragment (subpath) pointing
// to an existing note are checked; block references (#^id) are skipped. Target
// note headings are read from disk and cached per target path.
func brokenAnchors(db dbExecer, vaultPath string, opts DiagnoseOptions) ([]BrokenAnchor, error) {
	filterSQL, filterArgs := opts.sourceNoteFilterSQL()
	rows, err := db.Query(`SELECT src.path, e.raw_link, e.subpath, tgt.path
		FROM edges e
		JOIN nodes src ON src.id = e.source_id
		JOIN nodes tgt ON tgt.id = e.target_id
		WHERE src.type='note' AND src.exists_flag=1
		  AND tgt.type='note' AND tgt.exists_flag=1
		  AND e.subpath IS NOT NULL AND e.subpath != ''`+filterSQL+`
		ORDER BY src.path, e.raw_link`, filterArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Cache normalized heading sets per target path (read from disk once).
	headingCache := make(map[string]map[string]bool)
	var broken []BrokenAnchor
	for rows.Next() {
		var srcPath, rawLink, subpath, tgtPath string
		if err := rows.Scan(&srcPath, &rawLink, &subpath, &tgtPath); err != nil {
			return nil, err
		}
		wantAnchor, ok := normalizeAnchor(subpath)
		if !ok || wantAnchor == "" {
			continue // block reference or empty fragment
		}
		set, cached := headingCache[tgtPath]
		if !cached {
			set, err = readNoteHeadingSet(vaultPath, tgtPath)
			if err != nil {
				return nil, err
			}
			headingCache[tgtPath] = set
		}
		if !set[wantAnchor] {
			broken = append(broken, BrokenAnchor{
				SourcePath: srcPath,
				RawLink:    rawLink,
				TargetPath: tgtPath,
				Fragment:   strings.TrimPrefix(subpath, "#"),
			})
		}
	}
	return broken, rows.Err()
}

// readNoteHeadingSet reads a note from disk and returns the set of its
// normalized heading anchors.
func readNoteHeadingSet(vaultPath, notePath string) (map[string]bool, error) {
	content, err := os.ReadFile(filepath.Join(vaultPath, notePath))
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool)
	for _, h := range collectHeadings(string(content)) {
		if norm, ok := normalizeAnchor(h); ok && norm != "" {
			set[norm] = true
		}
	}
	return set, nil
}

// Diagnose returns diagnostic information for the indexed vault.
func Diagnose(vaultPath string, opts DiagnoseOptions) (*DiagnoseResult, error) {
	if err := validateGlobPatterns(opts.Path); err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(opts.Exclude); err != nil {
		return nil, err
	}

	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result := &DiagnoseResult{}

	wantNoteConflicts := isFieldActive("basename_conflicts", opts.Fields)
	wantAssetConflicts := isFieldActive("asset_basename_conflicts", opts.Fields)

	// allowed stays nil without a source filter (= report all conflict groups).
	var allowed map[int64]bool
	if opts.sourceFiltered() && (wantNoteConflicts || wantAssetConflicts) {
		allowed, err = basenameLinkTargets(db, opts)
		if err != nil {
			return nil, err
		}
	}

	if wantNoteConflicts {
		result.BasenameConflicts, err = basenameConflicts(db, NodeTypeNote, allowed)
		if err != nil {
			return nil, err
		}
	}

	if wantAssetConflicts {
		result.AssetBasenameConflicts, err = basenameConflicts(db, NodeTypeAsset, allowed)
		if err != nil {
			return nil, err
		}
	}

	if isFieldActive("phantoms", opts.Fields) {
		result.Phantoms, err = phantomNames(db, opts)
		if err != nil {
			return nil, err
		}
	}

	// Anchor checking is opt-in: only when "anchors" is explicitly requested.
	if isFieldActive("anchors", opts.Fields) && len(opts.Fields) > 0 {
		result.BrokenAnchors, err = brokenAnchors(db, vaultPath, opts)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}
