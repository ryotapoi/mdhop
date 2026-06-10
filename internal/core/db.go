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

// NodeType is the value set of the `nodes.type` column, used for
// comparisons and assignments in Go code. Single-quoted SQL literals
// such as `'phantom'` share the same string values but are kept as
// part of the SQL syntax rather than referencing these constants.
type NodeType string

const (
	NodeTypeNote    NodeType = "note"
	NodeTypeAsset   NodeType = "asset"
	NodeTypePhantom NodeType = "phantom"
	NodeTypeTag     NodeType = "tag"
)

// LinkType is the value set of the `edges.link_type` column. Same
// convention as NodeType: Go code uses these constants, while SQL
// literals such as `'wikilink'` keep the same string values.
type LinkType string

const (
	LinkTypeWikilink            LinkType = "wikilink"
	LinkTypeMarkdown            LinkType = "markdown"
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
	placeholders := make([]string, len(tagLinkTypes))
	args := make([]any, len(tagLinkTypes))
	for i, linkType := range tagLinkTypes {
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

func openDBAt(path string) (*sql.DB, error) {
	return sql.Open("sqlite", fmt.Sprintf("file:%s", path))
}

func openDBChecked(vaultPath string) (*sql.DB, error) {
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: run 'mdhop build' first", ErrIndexNotFound)
	}
	return openDBAt(dbp)
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
			mtime       INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_type_name ON nodes(type, name);`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_path ON nodes(path);`,
		`CREATE TABLE IF NOT EXISTS edges (
			id         INTEGER PRIMARY KEY,
			source_id  INTEGER NOT NULL,
			target_id  INTEGER NOT NULL,
			link_type  TEXT NOT NULL,
			raw_link   TEXT NOT NULL,
			subpath    TEXT,
			line_start INTEGER,
			line_end   INTEGER,
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

func upsertNode(db dbExecer, key string, typ NodeType, name, path string, mtime int64) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag, mtime)
		 VALUES (?, ?, ?, ?, 1, ?)
		 ON CONFLICT(node_key) DO UPDATE SET
		   name=excluded.name,
		   path=excluded.path,
		   exists_flag=excluded.exists_flag,
		   mtime=excluded.mtime`,
		key, typ, name, path, mtime,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 {
		// ON CONFLICT updated — fetch the existing ID.
		row := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", key)
		if err := row.Scan(&id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func upsertNote(db dbExecer, path, name string, mtime int64) (int64, error) {
	return upsertNode(db, noteKey(path), NodeTypeNote, name, path, mtime)
}

func upsertAsset(db dbExecer, path, name string, mtime int64) (int64, error) {
	return upsertNode(db, assetKey(path), NodeTypeAsset, name, path, mtime)
}

func noteKey(path string) string {
	return fmt.Sprintf("note:path:%s", path)
}

func assetKey(path string) string {
	return fmt.Sprintf("asset:path:%s", path)
}

func tagKey(name string) string {
	return fmt.Sprintf("tag:name:%s", strings.ToLower(name))
}

func phantomKey(name string) string {
	return fmt.Sprintf("phantom:name:%s", strings.ToLower(name))
}

func upsertPhantom(db dbExecer, name string) (int64, error) {
	key := phantomKey(name)
	res, err := db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag)
		 VALUES (?, 'phantom', ?, NULL, 0)
		 ON CONFLICT(node_key) DO NOTHING`,
		key, name,
	)
	if err != nil {
		return 0, err
	}
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

func upsertTag(db dbExecer, name string) (int64, error) {
	key := tagKey(name)
	res, err := db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag)
		 VALUES (?, 'tag', ?, NULL, 0)
		 ON CONFLICT(node_key) DO NOTHING`,
		key, name,
	)
	if err != nil {
		return 0, err
	}
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
		if err := insertMeta(db, nodeID, entry.Key, entry.Value, sortValue, storedType); err != nil {
			return nil, err
		}
	}
	return warnings, nil
}

func insertMeta(db dbExecer, nodeID int64, key, value, sortValue, valueType string) error {
	_, err := db.Exec(
		`INSERT INTO meta (node_id, key, value, sort_value, value_type)
		 VALUES (?, ?, ?, ?, ?)`,
		nodeID, key, value, sortValue, valueType,
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

func insertEdge(db dbExecer, sourceID, targetID int64, linkType LinkType, rawLink, subpath string, lineStart, lineEnd int) error {
	_, err := db.Exec(
		`INSERT INTO edges (source_id, target_id, link_type, raw_link, subpath, line_start, line_end)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sourceID, targetID, string(linkType), rawLink, subpath, lineStart, lineEnd,
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
			if _, err := tx.Exec(
				"UPDATE nodes SET type='phantom', node_key=?, path=NULL, exists_flag=0, mtime=NULL WHERE id=?",
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
	_, err := tx.Exec("DELETE FROM nodes WHERE type IN ('tag','phantom','asset') AND id NOT IN (SELECT DISTINCT target_id FROM edges)")
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

// ListDirNotes returns vault-relative paths of all registered notes
// under the given directory prefix.
// dirPrefix should not have a trailing slash (e.g., "sub", "sub/inner").
func ListDirNotes(vaultPath, dirPrefix string) ([]string, error) {
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return listDirNodesByType(db, dirPrefix, NodeTypeNote)
}

// ListDirAssets returns vault-relative paths of all registered assets
// under the given directory prefix.
// dirPrefix should not have a trailing slash (e.g., "sub", "sub/inner").
func ListDirAssets(vaultPath, dirPrefix string) ([]string, error) {
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return listDirNodesByType(db, dirPrefix, NodeTypeAsset)
}
