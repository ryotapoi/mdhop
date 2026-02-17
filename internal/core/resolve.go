package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveResult is the result of resolving a link.
type ResolveResult struct {
	Type    string // "note", "phantom", "tag", "url"
	Name    string // note=basename, tag="#tag", phantom=link name
	Path    string // vault-relative path (note only, empty otherwise)
	Exists  bool   // file existence flag
	Subpath string // "#Heading" / "#^block" (if any)
}

// Resolve resolves a link from a source file and returns the target node info.
func Resolve(vaultPath, fromPath, link string) (*ResolveResult, error) {
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	fromPath = normalizePath(fromPath)

	// Look up source node.
	sourceID, err := getNodeID(db, noteKey(fromPath))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("source not in index: %s", fromPath)
		}
		return nil, err
	}

	// Parse the link string to get linkOccur.
	links := parseLinks(link)
	if len(links) == 0 {
		return nil, fmt.Errorf("could not parse link: %s", link)
	}

	// If multiple linkOccurs (e.g. nested tag expansion), filter by rawLink == link.
	occur := selectLinkOccur(links, link)
	if occur == nil {
		return nil, fmt.Errorf("could not parse link: %s", link)
	}

	// Resolve the link via DB.
	targetID, subpath, err := resolveLinkFromDB(db, fromPath, *occur)
	if err != nil {
		return nil, err
	}

	// Verify the edge exists from source to target with matching subpath.
	exists, err := edgeExists(db, sourceID, targetID, subpath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("link not found in source %s: %s", fromPath, link)
	}

	// Fetch target node info.
	return fetchNodeResult(db, targetID, subpath)
}

// selectLinkOccur picks the linkOccur whose rawLink matches the input exactly.
// Returns nil if no match is found.
func selectLinkOccur(links []linkOccur, input string) *linkOccur {
	for i := range links {
		if links[i].rawLink == input {
			return &links[i]
		}
	}
	return nil
}

// resolveLinkFromDB resolves a linkOccur to a target node ID using DB queries.
// Mirrors resolveLink() in build.go but uses DB instead of in-memory maps.
func resolveLinkFromDB(db dbExecer, sourcePath string, link linkOccur) (int64, string, error) {
	// Self-link: [[#Heading]]
	if link.target == "" && link.subpath != "" {
		id, err := getNodeID(db, noteKey(sourcePath))
		if err != nil {
			return 0, "", err
		}
		return id, link.subpath, nil
	}

	// Tag or frontmatter tag
	if link.linkType == "tag" || link.linkType == "frontmatter" {
		key := fmt.Sprintf("tag:name:%s", strings.ToLower(link.target))
		id, err := getNodeID(db, key)
		if err != nil {
			if err == sql.ErrNoRows {
				return 0, "", fmt.Errorf("tag not found: %s", link.target)
			}
			return 0, "", err
		}
		return id, "", nil
	}

	target := link.target

	// Relative path resolution: ./Target or ../Root
	if link.isRelative {
		if escapesVault(sourcePath, target) {
			return 0, "", fmt.Errorf("link escapes vault: %s in %s", link.rawLink, sourcePath)
		}
		resolved := normalizePath(filepath.Join(filepath.Dir(sourcePath), target))
		return resolvePathFromDB(db, resolved, link)
	}

	// Vault-absolute path escape check (defense-in-depth).
	if !link.isBasename && pathEscapesVault(target) {
		return 0, "", fmt.Errorf("link escapes vault: %s in %s", link.rawLink, sourcePath)
	}

	// Absolute path (/ prefix): /sub/B.md → sub/B.md
	if strings.HasPrefix(target, "/") {
		stripped := strings.TrimPrefix(target, "/")
		return resolvePathFromDB(db, stripped, link)
	}

	// Wikilink with vault-relative path (contains /, not relative): [[path/to/Note]]
	if link.linkType == "wikilink" && !link.isBasename {
		return resolvePathFromDB(db, target, link)
	}

	// Basename resolution
	if link.isBasename {
		return resolveBasenameFromDB(db, target, link)
	}

	// Markdown link with path that is not relative and not / prefix
	return resolvePathFromDB(db, target, link)
}

// resolvePathFromDB finds a note node by path, falling back to phantom.
func resolvePathFromDB(db dbExecer, resolved string, link linkOccur) (int64, string, error) {
	normalized := normalizePath(resolved)
	lower := strings.ToLower(normalized)

	// Try exact path match (case-insensitive).
	var id int64
	err := db.QueryRow(
		`SELECT id FROM nodes WHERE type='note' AND (LOWER(path) = ? OR LOWER(path) = ?)`,
		lower, lower+".md",
	).Scan(&id)
	if err == nil {
		return id, link.subpath, nil
	}
	if err != sql.ErrNoRows {
		return 0, "", err
	}

	// Not found → look for phantom.
	name := filepath.Base(normalized)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	pk := phantomKey(name)
	err = db.QueryRow(`SELECT id FROM nodes WHERE node_key = ?`, pk).Scan(&id)
	if err == nil {
		return id, link.subpath, nil
	}
	if err != sql.ErrNoRows {
		return 0, "", err
	}

	return 0, "", fmt.Errorf("link not found: %s", resolved)
}

// resolveBasenameFromDB finds a note node by basename (case-insensitive).
func resolveBasenameFromDB(db dbExecer, target string, link linkOccur) (int64, string, error) {
	lower := strings.ToLower(target)

	rows, err := db.Query(
		`SELECT id FROM nodes WHERE type='note' AND LOWER(name) = ?`,
		lower,
	)
	if err != nil {
		return 0, "", err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, "", err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, "", err
	}

	if len(ids) == 1 {
		return ids[0], link.subpath, nil
	}
	if len(ids) > 1 {
		return 0, "", fmt.Errorf("ambiguous link: %s resolves to %d notes", target, len(ids))
	}

	// 0 matches → look for phantom.
	pk := phantomKey(target)
	var id int64
	err = db.QueryRow(`SELECT id FROM nodes WHERE node_key = ?`, pk).Scan(&id)
	if err == nil {
		return id, link.subpath, nil
	}
	if err != sql.ErrNoRows {
		return 0, "", err
	}

	return 0, "", fmt.Errorf("link not found: %s", target)
}

// edgeExists checks if an edge from source to target with matching subpath exists.
func edgeExists(db dbExecer, sourceID, targetID int64, subpath string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM edges WHERE source_id = ? AND target_id = ? AND COALESCE(subpath, '') = ?`,
		sourceID, targetID, subpath,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// fetchNodeResult retrieves the target node info and builds a ResolveResult.
func fetchNodeResult(db dbExecer, nodeID int64, subpath string) (*ResolveResult, error) {
	var typ, name string
	var path sql.NullString
	var existsFlag int

	err := db.QueryRow(
		`SELECT type, name, path, exists_flag FROM nodes WHERE id = ?`,
		nodeID,
	).Scan(&typ, &name, &path, &existsFlag)
	if err != nil {
		return nil, err
	}

	return &ResolveResult{
		Type:    typ,
		Name:    name,
		Path:    path.String,
		Exists:  existsFlag == 1,
		Subpath: subpath,
	}, nil
}
