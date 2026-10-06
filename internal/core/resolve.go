package core

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Resolve field names accepted by resolve --fields and used in its output.
const (
	FieldResolveType    = "type"
	FieldResolveName    = "name"
	FieldResolvePath    = "path"
	FieldResolveExists  = "exists"
	FieldResolveSubpath = "subpath"
)

// ResolveResult is the result of resolving a link.
type ResolveResult struct {
	Type    NodeType // NodeTypeNote/NodeTypeAsset/NodeTypePhantom/NodeTypeTag
	Name    string   // note=basename, tag="#tag", phantom=link name, asset=filename
	Path    string   // vault-relative path (note/asset only, empty otherwise)
	Exists  bool     // file existence flag
	Subpath string   // "#Heading" / "#^block" (if any)
}

// Resolve resolves a link from a source file and returns the target node info.
func Resolve(vaultPath, fromPath, link string) (*ResolveResult, error) {
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	fromPath = NormalizePath(fromPath)

	// Look up source node.
	sourceID, err := getNodeID(db, noteKey(fromPath))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("source not in index: %s: %w", fromPath, ErrFileNotRegistered)
		}
		return nil, err
	}

	// Exact raw occurrences carry their index-time context, including table
	// aliases and reference definitions. Never choose an arbitrary occurrence.
	rows, err := db.Query(`SELECT DISTINCT target_id, COALESCE(subpath, '') FROM edges
  WHERE source_id = ? AND raw_link = ? LIMIT 2`, sourceID, link)
	if err != nil {
		return nil, err
	}
	var exactID int64
	var exactSubpath string
	found := rows.Next()
	if found {
		if err := rows.Scan(&exactID, &exactSubpath); err != nil {
			rows.Close()
			return nil, err
		}
	}
	ambiguous := found && rows.Next()
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if ambiguous {
		return nil, fmt.Errorf("%w: raw link has different indexed meanings in %s: %s", ErrAmbiguousLink, fromPath, link)
	}
	if found {
		return fetchNodeResult(db, exactID, exactSubpath)
	}

	// Parse the link string to get linkOccur.
	links := parseLinks(link).Links
	if len(links) == 0 {
		if strings.HasPrefix(link, "[") && !strings.HasPrefix(link, "[[") {
			return nil, fmt.Errorf("%w in source %s: %s", ErrLinkNotFound, fromPath, link)
		}
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
		return nil, fmt.Errorf("%w in source %s: %s", ErrLinkNotFound, fromPath, link)
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
func resolveLinkFromDB(db dbExecer, sourcePath string, link linkOccur) (int64, string, error) {
	return resolveLinkWithBackend(sourcePath, link, dbLinkResolver{db: db})
}

type dbLinkResolver struct {
	db dbExecer
}

func (r dbLinkResolver) resolveSelf(sourcePath string, link linkOccur) (int64, string, error) {
	id, err := getNodeID(r.db, noteKey(sourcePath))
	if err != nil {
		return 0, "", err
	}
	return id, link.subpath, nil
}

func (r dbLinkResolver) resolveTag(link linkOccur) (int64, string, error) {
	key := tagKey(link.target)
	id, err := getNodeID(r.db, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "", fmt.Errorf("tag not found: %s", link.target)
		}
		return 0, "", err
	}
	return id, "", nil
}

func (r dbLinkResolver) resolvePath(resolved string, link linkOccur) (int64, string, error) {
	return resolvePathFromDB(r.db, resolved, link)
}

func (r dbLinkResolver) resolveBasename(target string, link linkOccur) (int64, string, error) {
	return resolveBasenameFromDB(r.db, target, link)
}

// resolvePathFromDB finds a note/asset node by path, falling back to phantom.
// Resolution order: note exact → note+.md → asset exact → phantom.
func resolvePathFromDB(db dbExecer, resolved string, link linkOccur) (int64, string, error) {
	normalized := NormalizePath(resolved)

	// Try note: exact path or path+.md (case-insensitive). Compare in Go so
	// Unicode case conversion matches the build-time resolve maps.
	id, found, err := findPathNodeID(db, NodeTypeNote, normalized, true)
	if err != nil {
		return 0, "", err
	}
	if found {
		return id, link.subpath, nil
	}

	// Try asset: exact path (case-insensitive).
	id, found, err = findPathNodeID(db, NodeTypeAsset, normalized, false)
	if err != nil {
		return 0, "", err
	}
	if found {
		return id, link.subpath, nil
	}

	// Not found → look for phantom.
	// D10: only strip .md extension for phantom name; preserve other extensions.
	name := filepath.Base(normalized)
	if strings.HasSuffix(strings.ToLower(name), ".md") {
		name = name[:len(name)-3]
	}
	pk := phantomKey(name)
	err = db.QueryRow(`SELECT id FROM nodes WHERE node_key = ?`, pk).Scan(&id)
	if err == nil {
		return id, link.subpath, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, "", err
	}

	return 0, "", fmt.Errorf("%w: %s", ErrLinkNotFound, resolved)
}

func findPathNodeID(db dbExecer, nodeType NodeType, path string, includeMarkdownExtension bool) (int64, bool, error) {
	lowerPath := strings.ToLower(NormalizePath(path))
	lowerMarkdownPath := lowerPath + ".md"
	rows, err := db.Query(`SELECT id, path FROM nodes WHERE type = ? ORDER BY id`, nodeType)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var storedPath string
		if err := rows.Scan(&id, &storedPath); err != nil {
			return 0, false, err
		}
		lowerStoredPath := strings.ToLower(NormalizePath(storedPath))
		if lowerStoredPath == lowerPath || includeMarkdownExtension && lowerStoredPath == lowerMarkdownPath {
			return id, true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return 0, false, err
	}
	return 0, false, nil
}

// resolveBasenameFromDB finds a note/asset node by basename (case-insensitive).
// Resolution order: note → asset → phantom.
// When multiple nodes match within the same type, applies root-priority rule.
func resolveBasenameFromDB(db dbExecer, target string, link linkOccur) (int64, string, error) {
	lower := strings.ToLower(normalizeTextNFC(target))

	// Try note by basename.
	noteMatches, err := queryBasenameMatches(db, NodeTypeNote, lower)
	if err != nil {
		return 0, "", err
	}
	if m, ok := pickBasenameMatch(noteMatches); ok {
		return m.id, link.subpath, nil
	}
	if len(noteMatches) > 1 {
		return 0, "", fmt.Errorf("%w: %s resolves to %d notes", ErrAmbiguousLink, target, len(noteMatches))
	}

	// Try asset by basename (name = filename with extension).
	assetMatches, err := queryBasenameMatches(db, NodeTypeAsset, lower)
	if err != nil {
		return 0, "", err
	}
	if m, ok := pickBasenameMatch(assetMatches); ok {
		return m.id, link.subpath, nil
	}
	if len(assetMatches) > 1 {
		return 0, "", fmt.Errorf("%w: %s resolves to %d assets", ErrAmbiguousLink, target, len(assetMatches))
	}

	// 0 matches → look for phantom.
	pk := phantomKey(target)
	var id int64
	err = db.QueryRow(`SELECT id FROM nodes WHERE node_key = ?`, pk).Scan(&id)
	if err == nil {
		return id, link.subpath, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, "", err
	}

	return 0, "", fmt.Errorf("%w: %s", ErrLinkNotFound, target)
}

// basenameMatch is a node matched by case-insensitive basename lookup.
type basenameMatch struct {
	id   int64
	path string
}

// queryBasenameMatches queries nodes of the given type matching a lowercase
// NFC basename. The comparison is done in Go, not SQL, so pre-v0.12 NFD index
// rows are still found by basename input. SQL discards only printable ASCII
// names that cannot match; non-ASCII (including invalid UTF-8) stays in Go.
// GLOB stops at NUL, so retain those names separately. The pattern is fixed,
// never derived from user input. No normalized column or custom SQL function is needed.
// Path/source lookups are migrated by rebuilding the index.
func queryBasenameMatches(db dbExecer, nodeType NodeType, lowerName string) ([]basenameMatch, error) {
	rows, err := db.Query(
		`SELECT id, name, path FROM nodes WHERE type=?
		 AND (name = ? COLLATE NOCASE OR name GLOB '*[^ -~]*' OR instr(name, char(0)) > 0)`,
		nodeType, lowerName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []basenameMatch
	for rows.Next() {
		var m basenameMatch
		var name string
		if err := rows.Scan(&m.id, &name, &m.path); err != nil {
			return nil, err
		}
		if strings.ToLower(normalizeTextNFC(name)) != lowerName {
			continue
		}
		m.path = NormalizePath(m.path)
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

// pickBasenameMatch selects a single match from basename lookup results.
// A unique match wins; among multiple matches the vault-root file wins
// (root-priority rule, ADR 0004). Returns false when no unambiguous choice
// exists (zero matches, or multiple matches none of which is at the root).
func pickBasenameMatch(matches []basenameMatch) (basenameMatch, bool) {
	if len(matches) == 1 {
		return matches[0], true
	}
	for _, m := range matches {
		if isRootFile(m.path) {
			return m, true
		}
	}
	return basenameMatch{}, false
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
	var typ NodeType
	var name string
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
