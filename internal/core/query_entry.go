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
		return 0, NodeInfo{}, errors.New("no entry specified: provide --file, --tag, --phantom, or --name")
	}
	if count > 1 {
		return 0, NodeInfo{}, errors.New("multiple entry specs: provide exactly one of --file, --tag, --phantom, --name")
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
		if errors.Is(err, sql.ErrNoRows) {
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
	if !errors.Is(err, sql.ErrNoRows) {
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

	// Try note by basename (case-insensitive), with root-priority (ADR 0004).
	lower := strings.ToLower(name)
	noteMatches, err := queryBasenameMatches(db, NodeTypeNote, lower)
	if err != nil {
		return 0, NodeInfo{}, err
	}
	if m, ok := pickBasenameMatch(noteMatches); ok {
		info, err := fetchNodeInfo(db, m.id)
		if err != nil {
			return 0, NodeInfo{}, err
		}
		return m.id, info, nil
	}
	if len(noteMatches) > 1 {
		return 0, NodeInfo{}, fmt.Errorf("%w: %s matches %d notes", ErrAmbiguousName, name, len(noteMatches))
	}

	// Try asset by basename (case-insensitive).
	assetMatches, err := queryBasenameMatches(db, NodeTypeAsset, lower)
	if err != nil {
		return 0, NodeInfo{}, err
	}
	if m, ok := pickBasenameMatch(assetMatches); ok {
		info, err := fetchNodeInfo(db, m.id)
		if err != nil {
			return 0, NodeInfo{}, err
		}
		return m.id, info, nil
	}
	if len(assetMatches) > 1 {
		return 0, NodeInfo{}, fmt.Errorf("%w: %s matches %d assets", ErrAmbiguousName, name, len(assetMatches))
	}

	// Try phantom.
	return findEntryByKey(db, phantomKey(name), fmt.Sprintf("name not found: %s", name))
}

// fetchNodeInfo retrieves NodeInfo for a node by ID.
func fetchNodeInfo(db dbExecer, nodeID int64) (NodeInfo, error) {
	var typ NodeType
	var name, path string
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
