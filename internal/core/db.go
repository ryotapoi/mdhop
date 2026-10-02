package core

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// dbExecer abstracts *sql.DB and *sql.Tx for shared upsert/query functions.
type dbExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

const (
	dataDirName = ".mdhop"
	dbFileName  = "index.sqlite"
)

// NodeType is the value set of the `nodes.type` column. Go code uses these
// constants for comparisons and assignments, and some SQL predicates bind
// them as arguments. Other SQL statements still use string literals, so
// changing a type value also requires checking those statements.
type NodeType string

const (
	NodeTypeNote    NodeType = "note"
	NodeTypeAsset   NodeType = "asset"
	NodeTypePhantom NodeType = "phantom"
	NodeTypeTag     NodeType = "tag"
)

// LinkType is the value set of the `edges.link_type` column. SQL predicates
// may bind these constants as arguments; other statements still use literals.
type LinkType string

const (
	LinkTypeWikilink            LinkType = "wikilink"
	LinkTypeMarkdown            LinkType = "markdown"
	LinkTypeMarkdownReference   LinkType = "markdown_reference"
	LinkTypeTag                 LinkType = "tag"
	LinkTypeFrontmatter         LinkType = "frontmatter"
	LinkTypeFrontmatterWikilink LinkType = "frontmatter_wikilink"
	LinkTypeFrontmatterPath     LinkType = "frontmatter_path"
)

var tagLinkTypes = []LinkType{LinkTypeTag, LinkTypeFrontmatter}

func isTagLinkType(linkType LinkType) bool {
	for _, tagLinkType := range tagLinkTypes {
		if linkType == tagLinkType {
			return true
		}
	}
	return false
}

func tagLinkTypeSQLIn(alias string) (string, []any) {
	return linkTypeSQLIn(alias, tagLinkTypes)
}

func linkTypeSQLIn(alias string, linkTypes []LinkType) (string, []any) {
	placeholders := make([]string, len(linkTypes))
	args := make([]any, len(linkTypes))
	for i, linkType := range linkTypes {
		placeholders[i] = "?"
		args[i] = string(linkType)
	}
	return alias + " IN (" + strings.Join(placeholders, ", ") + ")", args
}

func dbPath(vaultPath string) string {
	return filepath.Join(vaultPath, dataDirName, dbFileName)
}

func ensureDataDir(vaultPath string) (string, error) {
	dir := filepath.Join(vaultPath, dataDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// openDBAt opens the SQLite database at path. Do not append URI query
// parameters (e.g. "?mode=ro") to path: the "file:%s" format embeds path
// verbatim, so query parameters are treated as part of the filename rather
// than SQLite URI options, silently opening (or creating) the wrong file.
func openDBAt(path string) (*sql.DB, error) {
	return sql.Open("sqlite", fmt.Sprintf("file:%s", path))
}

func openDBChecked(vaultPath string) (*sql.DB, error) {
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: run 'mdhop build' first", ErrIndexNotFound)
	}
	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("SELECT reference_target FROM edges LIMIT 0"); err != nil {
		db.Close()
		return nil, fmt.Errorf("index schema requires rebuild: run 'mdhop build' first: %w", err)
	}
	return db, nil
}

func initSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS nodes (
			id          INTEGER PRIMARY KEY,
			node_key    TEXT NOT NULL UNIQUE,
			type        TEXT NOT NULL,
			name        TEXT NOT NULL,
			path        TEXT,
			exists_flag INTEGER NOT NULL DEFAULT 1,
			mtime       INTEGER,
			lines       INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_type_name ON nodes(type, name);`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_path ON nodes(path);`,
		`CREATE TABLE IF NOT EXISTS edges (
			id              INTEGER PRIMARY KEY,
			source_id       INTEGER NOT NULL,
			target_id       INTEGER NOT NULL,
			link_type       TEXT NOT NULL,
			raw_link        TEXT NOT NULL,
			frontmatter_key TEXT,
			reference_target TEXT,
			subpath         TEXT,
			line_start      INTEGER,
			line_end        INTEGER,
			FOREIGN KEY(source_id) REFERENCES nodes(id),
			FOREIGN KEY(target_id) REFERENCES nodes(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source_id);`,
		`CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target_id);`,
		`CREATE INDEX IF NOT EXISTS idx_edges_source_target ON edges(source_id, target_id);`,
		`CREATE TABLE IF NOT EXISTS meta (
			id         INTEGER PRIMARY KEY,
			node_id    INTEGER NOT NULL,
			key        TEXT NOT NULL,
			value      TEXT NOT NULL,
			line       INTEGER NOT NULL,
			sort_value TEXT,
			value_type TEXT,
			FOREIGN KEY(node_id) REFERENCES nodes(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_meta_node_id ON meta(node_id);`,
		`CREATE INDEX IF NOT EXISTS idx_meta_key_sort_value ON meta(key, sort_value);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// upsertNode inserts or updates a node. lines holds the note line count for
// note nodes; it is nil for assets and phantoms, which have no line count.
func upsertNode(db dbExecer, key string, typ NodeType, name, path string, mtime int64, lines *int) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag, mtime, lines)
		 VALUES (?, ?, ?, ?, 1, ?, ?)
		 ON CONFLICT(node_key) DO UPDATE SET
		   name=excluded.name,
		   path=excluded.path,
		   exists_flag=excluded.exists_flag,
		   mtime=excluded.mtime,
		   lines=excluded.lines`,
		key, typ, name, path, mtime, lines,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 {
		// modernc.org/sqlite returns 0 from LastInsertId() on ON CONFLICT DO
		// UPDATE (the row is updated, not inserted). Fall back to a SELECT.
		row := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", key)
		if err := row.Scan(&id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func upsertNote(db dbExecer, path, name string, mtime int64, lines int) (int64, error) {
	return upsertNode(db, noteKey(path), NodeTypeNote, name, path, mtime, &lines)
}

func upsertAsset(db dbExecer, path, name string, mtime int64) (int64, error) {
	return upsertNode(db, assetKey(path), NodeTypeAsset, name, path, mtime, nil)
}

func noteKey(path string) string {
	return fmt.Sprintf("note:path:%s", NormalizePath(path))
}

func assetKey(path string) string {
	return fmt.Sprintf("asset:path:%s", NormalizePath(path))
}

func tagKey(name string) string {
	return fmt.Sprintf("tag:name:%s", strings.ToLower(normalizeTextNFC(name)))
}

func phantomKey(name string) string {
	return fmt.Sprintf("phantom:name:%s", strings.ToLower(normalizeTextNFC(name)))
}

func upsertPhantom(db dbExecer, name string) (int64, error) {
	return upsertNamedNode(db, phantomKey(name), NodeTypePhantom, name)
}

// upsertTag inserts a tag node. The name column keeps its original case; only
// node_key is lowercased (via tagKey). Callers comparing n.name must use
// LOWER(n.name) or strings.ToLower because the stored name retains case.
func upsertTag(db dbExecer, name string) (int64, error) {
	return upsertNamedNode(db, tagKey(name), NodeTypeTag, name)
}

func upsertNamedNode(db dbExecer, key string, nodeType NodeType, name string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag)
		 VALUES (?, ?, ?, NULL, 0)
		 ON CONFLICT(node_key) DO NOTHING`,
		key, nodeType, name,
	)
	if err != nil {
		return 0, err
	}
	// modernc.org/sqlite returns the previous INSERT's rowid (not 0) from
	// LastInsertId() on ON CONFLICT DO NOTHING, so it cannot tell whether the
	// row was actually inserted. Check RowsAffected() first; only trust
	// LastInsertId() when exactly one row was inserted.
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		id, err := res.LastInsertId()
		if err != nil {
			return 0, err
		}
		return id, nil
	}
	// ON CONFLICT: row already exists — fetch its ID.
	var id int64
	row := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", key)
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// MetaRow represents a row in the meta table (frontmatter key-value pair).
type MetaRow struct {
	Key       string
	Value     string
	SortValue string
	ValueType string
}

// insertMetaEntries inserts all frontmatter entries for a node, applying type
// normalization from metaCfg. Returns warnings for values that fail normalization.
func insertMetaEntries(db dbExecer, nodeID int64, path string, entries []FrontmatterEntry, metaCfg MetaConfig) ([]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	var warnings []string
	for _, entry := range entries {
		typeInfo, _ := metaCfg.LookupType(entry.Key)
		sortValue, warning := NormalizeSortValue(entry.Value, typeInfo)
		storedType := string(typeInfo.Name)
		if warning != "" {
			warnings = append(warnings, fmt.Sprintf("%s:%d: %s (key=%s)", path, entry.Line, warning, entry.Key))
			storedType = string(MetaTypeString)
		}
		if err := insertMeta(db, nodeID, entry.Key, entry.Value, entry.Line, sortValue, storedType); err != nil {
			return nil, err
		}
	}
	return warnings, nil
}

func insertMeta(db dbExecer, nodeID int64, key, value string, line int, sortValue, valueType string) error {
	_, err := db.Exec(
		`INSERT INTO meta (node_id, key, value, line, sort_value, value_type)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		nodeID, key, value, line, sortValue, valueType,
	)
	return err
}

func deleteMetaByNode(db dbExecer, nodeID int64) error {
	_, err := db.Exec("DELETE FROM meta WHERE node_id = ?", nodeID)
	return err
}

func queryMetaByNode(db dbExecer, nodeID int64) ([]MetaRow, error) {
	rows, err := db.Query(
		`SELECT key, value, COALESCE(sort_value,''), COALESCE(value_type,'') FROM meta WHERE node_id = ? ORDER BY key, value`,
		nodeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []MetaRow
	for rows.Next() {
		var r MetaRow
		if err := rows.Scan(&r.Key, &r.Value, &r.SortValue, &r.ValueType); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func insertEdge(db dbExecer, sourceID, targetID int64, linkType LinkType, rawLink, frontmatterKey, referenceTarget, subpath string, lineStart, lineEnd int) error {
	var key any
	if frontmatterKey != "" {
		key = frontmatterKey
	}
	var reference any
	if referenceTarget != "" {
		reference = referenceTarget
	}
	_, err := db.Exec(
		`INSERT INTO edges (source_id, target_id, link_type, raw_link, frontmatter_key, reference_target, subpath, line_start, line_end)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sourceID, targetID, string(linkType), rawLink, key, reference, subpath, lineStart, lineEnd,
	)
	return err
}

func getNodeID(db dbExecer, nodeKey string) (int64, error) {
	var id int64
	row := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", nodeKey)
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// removeOrPhantomize removes a note node. If it has incoming references
// (excluding self-links via source_id != nodeID), converts to phantom.
// Otherwise fully deletes the node and its edges.
func removeOrPhantomize(tx dbExecer, nodeID int64, name string) (phantomized bool, err error) {
	// Check incoming edges (excluding self-links).
	var incomingCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM edges WHERE target_id = ? AND source_id != ?", nodeID, nodeID).Scan(&incomingCount); err != nil {
		return false, err
	}

	if incomingCount > 0 {
		// Phantom conversion: has incoming references.
		// Delete all outgoing edges.
		if _, err := tx.Exec("DELETE FROM edges WHERE source_id = ?", nodeID); err != nil {
			return false, err
		}
		// Delete meta entries (phantoms have no frontmatter).
		if err := deleteMetaByNode(tx, nodeID); err != nil {
			return false, err
		}

		// Check if a phantom with the same name already exists.
		pk := phantomKey(name)
		var existingPhantomID int64
		err := tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&existingPhantomID)
		if err == nil {
			// Existing phantom found: reassign incoming edges and delete the note node.
			if _, err := tx.Exec("UPDATE edges SET target_id = ? WHERE target_id = ?", existingPhantomID, nodeID); err != nil {
				return false, err
			}
			if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", nodeID); err != nil {
				return false, err
			}
		} else if errors.Is(err, sql.ErrNoRows) {
			// No existing phantom: convert note to phantom in-place.
			// Clear lines too: phantoms have no line count.
			if _, err := tx.Exec(
				"UPDATE nodes SET type='phantom', node_key=?, path=NULL, exists_flag=0, mtime=NULL, lines=NULL WHERE id=?",
				pk, nodeID,
			); err != nil {
				return false, err
			}
		} else {
			return false, err
		}
		return true, nil
	}

	// Complete deletion: no incoming references.
	if _, err := tx.Exec("DELETE FROM edges WHERE source_id = ? OR target_id = ?", nodeID, nodeID); err != nil {
		return false, err
	}
	if err := deleteMetaByNode(tx, nodeID); err != nil {
		return false, err
	}
	if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", nodeID); err != nil {
		return false, err
	}
	return false, nil
}

// cleanupOrphanedNodes removes tag, phantom, and asset nodes not referenced by any edge.
// url nodes are not affected.
func cleanupOrphanedNodes(tx dbExecer) error {
	_, err := tx.Exec("DELETE FROM nodes WHERE type IN (?,?,?) AND id NOT IN (SELECT DISTINCT target_id FROM edges)", NodeTypeTag, NodeTypePhantom, NodeTypeAsset)
	return err
}

// escapeLikePattern escapes %, _, and \ in s for use in SQL LIKE patterns with ESCAPE '\'.
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// listDirNodesByType returns vault-relative paths of all registered nodes of
// the given type under the given directory prefix.
func listDirNodesByType(db dbExecer, dirPrefix string, nodeType NodeType) ([]string, error) {
	pattern := escapeLikePattern(dirPrefix) + "/%"
	rows, err := db.Query(
		`SELECT path FROM nodes WHERE type=? AND exists_flag=1 AND (path LIKE ? ESCAPE '\')`,
		nodeType, pattern,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
