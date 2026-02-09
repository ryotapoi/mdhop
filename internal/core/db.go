package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	dataDirName = ".mdhop"
	dbFileName  = "index.sqlite"
)

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
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func upsertNote(db *sql.DB, path, name string, mtime int64) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag, mtime)
		 VALUES (?, 'note', ?, ?, 1, ?)
		 ON CONFLICT(node_key) DO UPDATE SET
		   name=excluded.name,
		   path=excluded.path,
		   exists_flag=excluded.exists_flag,
		   mtime=excluded.mtime`,
		noteKey(path), name, path, mtime,
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
		row := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", noteKey(path))
		if err := row.Scan(&id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func noteKey(path string) string {
	return fmt.Sprintf("note:path:%s", path)
}

func upsertPhantom(db *sql.DB, name string) (int64, error) {
	key := fmt.Sprintf("phantom:name:%s", strings.ToLower(name))
	res, err := db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag)
		 VALUES (?, 'phantom', ?, NULL, 0)
		 ON CONFLICT(node_key) DO NOTHING`,
		key, name,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 {
		row := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", key)
		if err := row.Scan(&id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func upsertTag(db *sql.DB, name string) (int64, error) {
	key := fmt.Sprintf("tag:name:%s", strings.ToLower(name))
	res, err := db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag)
		 VALUES (?, 'tag', ?, NULL, 0)
		 ON CONFLICT(node_key) DO NOTHING`,
		key, name,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 {
		row := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", key)
		if err := row.Scan(&id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func insertEdge(db *sql.DB, sourceID, targetID int64, linkType, rawLink, subpath string, lineStart, lineEnd int) error {
	_, err := db.Exec(
		`INSERT INTO edges (source_id, target_id, link_type, raw_link, subpath, line_start, line_end)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sourceID, targetID, linkType, rawLink, subpath, lineStart, lineEnd,
	)
	return err
}

func getNodeID(db *sql.DB, nodeKey string) (int64, error) {
	var id int64
	row := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", nodeKey)
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}
