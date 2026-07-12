package core

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// moveInfo records one file move within a directory move.
type moveInfo struct {
	from    string
	to      string
	nodeID  int64
	dbMtime int64
	isAsset bool
}

// diskOnlyMove records a non-registered file (not in DB) that must be moved alongside
// the registered files in a directory move.
type diskOnlyMove struct {
	from string
	to   string
}

// validateMoveDirOptions checks that source and destination directories are well-formed
// and disjoint, returning the normalized fromDir/toDir or an error.
func validateMoveDirOptions(opts MoveDirOptions) (fromDir, toDir string, err error) {
	fromDir = NormalizePath(opts.FromDir)
	toDir = NormalizePath(opts.ToDir)

	if filepath.IsAbs(fromDir) {
		return "", "", fmt.Errorf("source directory must be vault-relative: %s", fromDir)
	}
	if filepath.IsAbs(toDir) {
		return "", "", fmt.Errorf("destination directory must be vault-relative: %s", toDir)
	}
	if pathEscapesVault(fromDir) {
		return "", "", fmt.Errorf("source directory escapes vault: %s", fromDir)
	}
	if pathEscapesVault(toDir) {
		return "", "", fmt.Errorf("destination directory escapes vault: %s", toDir)
	}
	if fromDir == toDir {
		return "", "", fmt.Errorf("source and destination are the same: %s", fromDir)
	}
	if strings.HasPrefix(toDir+"/", fromDir+"/") || strings.HasPrefix(fromDir+"/", toDir+"/") {
		return "", "", fmt.Errorf("source and destination directories overlap")
	}
	return fromDir, toDir, nil
}

// loadSingleMoveFromDB resolves one registered note or asset for single-file Move.
func loadSingleMoveFromDB(db dbExecer, from, to string) (moveInfo, error) {
	rm, err := buildMapsFromDB(db)
	if err != nil {
		return moveInfo{}, err
	}

	var nodeID int64
	var isAsset bool
	if id, ok := rm.pathToID[from]; ok {
		nodeID = id
	} else if id, ok := rm.assetPathToID[from]; ok {
		nodeID = id
		isAsset = true
	} else {
		return moveInfo{}, fmt.Errorf("%w: %s", ErrFileNotRegistered, from)
	}

	var dbMtime int64
	if err := db.QueryRow("SELECT mtime FROM nodes WHERE id = ?", nodeID).Scan(&dbMtime); err != nil {
		return moveInfo{}, err
	}
	return moveInfo{from: from, to: to, nodeID: nodeID, dbMtime: dbMtime, isAsset: isAsset}, nil
}

// loadMovesFromDB resolves all registered notes and assets under fromDir into moveInfo
// records carrying their computed to-paths under toDir.
func loadMovesFromDB(db dbExecer, fromDir, toDir string) ([]moveInfo, error) {
	fromNotePaths, err := listDirNodesByType(db, fromDir, NodeTypeNote)
	if err != nil {
		return nil, err
	}
	fromAssetPaths, err := listDirNodesByType(db, fromDir, NodeTypeAsset)
	if err != nil {
		return nil, err
	}
	if len(fromNotePaths) == 0 && len(fromAssetPaths) == 0 {
		return nil, fmt.Errorf("no files registered under directory: %s", fromDir)
	}

	moves := make([]moveInfo, 0, len(fromNotePaths)+len(fromAssetPaths))
	for _, from := range fromNotePaths {
		to := toDir + "/" + strings.TrimPrefix(from, fromDir+"/")
		var nodeID, dbMtime int64
		err := db.QueryRow(
			"SELECT id, mtime FROM nodes WHERE node_key = ? AND type = 'note'",
			noteKey(from),
		).Scan(&nodeID, &dbMtime)
		if err != nil {
			return nil, err
		}
		moves = append(moves, moveInfo{from: from, to: to, nodeID: nodeID, dbMtime: dbMtime})
	}
	for _, from := range fromAssetPaths {
		to := toDir + "/" + strings.TrimPrefix(from, fromDir+"/")
		var nodeID, dbMtime int64
		err := db.QueryRow(
			"SELECT id, mtime FROM nodes WHERE node_key = ? AND type = 'asset'",
			assetKey(from),
		).Scan(&nodeID, &dbMtime)
		if err != nil {
			return nil, err
		}
		moves = append(moves, moveInfo{from: from, to: to, nodeID: nodeID, dbMtime: dbMtime, isAsset: true})
	}
	return moves, nil
}

// checkDestinationsFree returns ErrAlreadyRegistered if any destination key already exists.
func checkDestinationsFree(db dbExecer, moves []moveInfo) error {
	for _, m := range moves {
		var toKey string
		if m.isAsset {
			toKey = assetKey(m.to)
		} else {
			toKey = noteKey(m.to)
		}
		var existingID int64
		err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", toKey).Scan(&existingID)
		if err == nil {
			return fmt.Errorf("%w: %s", ErrAlreadyRegistered, m.to)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	return nil
}

// collectDiskOnlyFiles walks fromDir on disk and returns non-.md files that are not
// registered in the DB. They will be moved verbatim alongside the registered files.
// Returns an empty slice when fromDir does not exist on disk (already-moved mode).
func collectDiskOnlyFiles(vaultPath, fromDir, toDir string, moves []moveInfo) ([]diskOnlyMove, error) {
	var diskOnlyFiles []diskOnlyMove
	absDir := filepath.Join(vaultPath, fromDir)
	registeredPaths := make(map[string]bool, len(moves))
	for _, m := range moves {
		registeredPaths[m.from] = true
	}
	if err := filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(vaultPath, path)
		relNorm := NormalizePath(rel)
		if !registeredPaths[relNorm] {
			to := toDir + "/" + strings.TrimPrefix(relNorm, fromDir+"/")
			diskOnlyFiles = append(diskOnlyFiles, diskOnlyMove{from: relNorm, to: to})
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return diskOnlyFiles, nil
}

// classifyDiskState determines whether the registered files still need to be moved on disk.
// Returns needDiskMove=true when all files are at their from paths, false when they are
// already at the to paths. Mixed or missing states return an error.
//
// Callers must pass a non-empty moves slice; an empty slice returns (false, nil) which
// is meaningless. MoveDir guards this via loadMovesFromDB which errors on no matches.
func classifyDiskState(vaultPath string, moves []moveInfo) (needDiskMove bool, err error) {
	var normalMode, alreadyMovedMode bool
	for _, m := range moves {
		fromOnDisk := fileExists(filepath.Join(vaultPath, m.from))
		toOnDisk := fileExists(filepath.Join(vaultPath, m.to))
		switch {
		case fromOnDisk && !toOnDisk:
			normalMode = true
		case !fromOnDisk && toOnDisk:
			alreadyMovedMode = true
		case fromOnDisk && toOnDisk:
			return false, fmt.Errorf("%w: %s", ErrAlreadyExistsOnDisk, m.to)
		default:
			return false, fmt.Errorf("%w: %s", ErrSourceFileMissing, m.from)
		}
	}
	if normalMode && alreadyMovedMode {
		return false, fmt.Errorf("inconsistent disk state for directory move")
	}
	return normalMode, nil
}

// checkMovedFilesNotStale verifies that on-disk mtimes match the DB-recorded mtimes
// at the appropriate location for each move.
func checkMovedFilesNotStale(vaultPath string, moves []moveInfo, needDiskMove bool) error {
	for _, m := range moves {
		var checkPath string
		if needDiskMove {
			checkPath = filepath.Join(vaultPath, m.from)
		} else {
			checkPath = filepath.Join(vaultPath, m.to)
		}
		info, err := os.Stat(checkPath)
		if err != nil {
			return err
		}
		if info.ModTime().Unix() != m.dbMtime {
			if needDiskMove {
				return fmt.Errorf("%w: %s", ErrSourceStale, m.from)
			}
			return fmt.Errorf("%w: %s", ErrMovedFileStale, m.to)
		}
	}
	return nil
}

// fileExists checks if a file exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
