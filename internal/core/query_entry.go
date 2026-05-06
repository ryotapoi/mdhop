package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// findEntryNode resolves an EntrySpec to a node ID and NodeInfo.
func findEntryNode(db dbExecer, spec EntrySpec) (int64, NodeInfo, error) {
	count := 0
	if spec.File != "" {
		count++
	}
	if spec.Tag != "" {
		count++
	}
	if spec.Phantom != "" {
		count++
	}
	if spec.Name != "" {
		count++
	}
	if count == 0 {
		return 0, NodeInfo{}, fmt.Errorf("no entry specified: provide --file, --tag, --phantom, or --name")
	}
	if count > 1 {
		return 0, NodeInfo{}, fmt.Errorf("multiple entry specs: provide exactly one of --file, --tag, --phantom, --name")
	}

	if spec.File != "" {
		return findEntryByFile(db, spec.File)
	}
	if spec.Tag != "" {
		return findEntryByTag(db, spec.Tag)
	}
	if spec.Phantom != "" {
		return findEntryByPhantom(db, spec.Phantom)
	}
	return findEntryByName(db, spec.Name)
}

func findEntryByKey(db dbExecer, key, errMsg string) (int64, NodeInfo, error) {
	id, err := getNodeID(db, key)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, NodeInfo{}, errors.New(errMsg)
		}
		return 0, NodeInfo{}, err
	}
	info, err := fetchNodeInfo(db, id)
	if err != nil {
		return 0, NodeInfo{}, err
	}
	return id, info, nil
}

func findEntryByFile(db dbExecer, file string) (int64, NodeInfo, error) {
	path := NormalizePath(file)
	// Try note first, then asset. Only fall back on ErrNoRows, not on real DB errors.
	noteID, err := getNodeID(db, noteKey(path))
	if err == nil {
		info, err := fetchNodeInfo(db, noteID)
		if err != nil {
			return 0, NodeInfo{}, err
		}
		return noteID, info, nil
	}
	if err != sql.ErrNoRows {
		return 0, NodeInfo{}, err
	}
	return findEntryByKey(db, assetKey(path), fmt.Sprintf("file not in index: %s", path))
}

func findEntryByTag(db dbExecer, tag string) (int64, NodeInfo, error) {
	if !strings.HasPrefix(tag, "#") {
		tag = "#" + tag
	}
	return findEntryByKey(db, tagKey(tag), fmt.Sprintf("tag not in index: %s", tag))
}

func findEntryByPhantom(db dbExecer, name string) (int64, NodeInfo, error) {
	return findEntryByKey(db, phantomKey(name), fmt.Sprintf("phantom not in index: %s", name))
}

func findEntryByName(db dbExecer, name string) (int64, NodeInfo, error) {
	if strings.HasPrefix(name, "#") {
		return findEntryByTag(db, name)
	}

	// Try note by basename (case-insensitive).
	lower := strings.ToLower(name)
	rows, err := db.Query(
		`SELECT id, type, name, COALESCE(path,''), exists_flag FROM nodes WHERE type='note' AND LOWER(name)=?`,
		lower,
	)
	if err != nil {
		return 0, NodeInfo{}, err
	}
	defer rows.Close()

	var matches []struct {
		id   int64
		info NodeInfo
	}
	for rows.Next() {
		var id int64
		var typ, n, p string
		var exists int
		if err := rows.Scan(&id, &typ, &n, &p, &exists); err != nil {
			return 0, NodeInfo{}, err
		}
		matches = append(matches, struct {
			id   int64
			info NodeInfo
		}{id, NodeInfo{Type: typ, Name: n, Path: p, Exists: exists == 1}})
	}
	if err := rows.Err(); err != nil {
		return 0, NodeInfo{}, err
	}

	if len(matches) == 1 {
		return matches[0].id, matches[0].info, nil
	}
	if len(matches) > 1 {
		// Root-priority: if one match is at vault root, resolve to it.
		for _, m := range matches {
			if isRootFile(m.info.Path) {
				return m.id, m.info, nil
			}
		}
		return 0, NodeInfo{}, fmt.Errorf("ambiguous name: %s matches %d notes", name, len(matches))
	}

	// Try asset by basename (case-insensitive).
	assetRows, err := db.Query(
		`SELECT id, type, name, COALESCE(path,''), exists_flag FROM nodes WHERE type='asset' AND LOWER(name)=?`,
		lower,
	)
	if err != nil {
		return 0, NodeInfo{}, err
	}
	defer assetRows.Close()

	var assetMatches []struct {
		id   int64
		info NodeInfo
	}
	for assetRows.Next() {
		var id int64
		var typ, n, p string
		var exists int
		if err := assetRows.Scan(&id, &typ, &n, &p, &exists); err != nil {
			return 0, NodeInfo{}, err
		}
		assetMatches = append(assetMatches, struct {
			id   int64
			info NodeInfo
		}{id, NodeInfo{Type: typ, Name: n, Path: p, Exists: exists == 1}})
	}
	if err := assetRows.Err(); err != nil {
		return 0, NodeInfo{}, err
	}

	if len(assetMatches) == 1 {
		return assetMatches[0].id, assetMatches[0].info, nil
	}
	if len(assetMatches) > 1 {
		for _, m := range assetMatches {
			if isRootFile(m.info.Path) {
				return m.id, m.info, nil
			}
		}
		return 0, NodeInfo{}, fmt.Errorf("ambiguous name: %s matches %d assets", name, len(assetMatches))
	}

	// Try phantom.
	return findEntryByKey(db, phantomKey(name), fmt.Sprintf("name not found: %s", name))
}

// fetchNodeInfo retrieves NodeInfo for a node by ID.
func fetchNodeInfo(db dbExecer, nodeID int64) (NodeInfo, error) {
	var typ, name, path string
	var existsFlag int

	err := db.QueryRow(
		`SELECT type, name, COALESCE(path,''), exists_flag FROM nodes WHERE id = ?`,
		nodeID,
	).Scan(&typ, &name, &path, &existsFlag)
	if err != nil {
		return NodeInfo{}, err
	}

	return NodeInfo{
		Type:   typ,
		Name:   name,
		Path:   path,
		Exists: existsFlag == 1,
	}, nil
}
