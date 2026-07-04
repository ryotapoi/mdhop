package core

import (
	"fmt"
	"path/filepath"
)

// MoveOptions controls the move operation.
type MoveOptions struct {
	From string // vault-relative old path
	To   string // vault-relative new path
}

// MoveResult reports the outcome of the move operation.
type MoveResult struct {
	Rewritten []RewrittenLink
}

// Move moves a file from one path to another, updating the index and rewriting links.
// If the file has already been moved on disk (from absent, to present), the disk move
// is skipped and only link rewrites + DB updates are performed.
func Move(vaultPath string, opts MoveOptions) (*MoveResult, error) {
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	cfg, err := LoadConfig(vaultPath)
	if err != nil {
		return nil, err
	}

	from, to, err := validateMoveOptions(opts)
	if err != nil {
		return nil, err
	}
	move, err := loadSingleMoveFromDB(db, from, to)
	if err != nil {
		return nil, err
	}
	if err := checkDestinationsFree(db, []moveInfo{move}); err != nil {
		return nil, err
	}
	needDiskMove, err := classifyDiskState(vaultPath, []moveInfo{move})
	if err != nil {
		return nil, err
	}
	if err := checkMovedFilesNotStale(vaultPath, []moveInfo{move}, needDiskMove); err != nil {
		return nil, err
	}
	result, err := executeMoves(vaultPath, db, cfg, []moveInfo{move}, nil, needDiskMove)
	if err != nil {
		return nil, err
	}
	return &MoveResult{Rewritten: result.Rewritten}, nil
}

func validateMoveOptions(opts MoveOptions) (from, to string, err error) {
	from = NormalizePath(opts.From)
	to = NormalizePath(opts.To)

	if filepath.IsAbs(from) {
		return "", "", fmt.Errorf("source path must be vault-relative: %s", from)
	}
	if filepath.IsAbs(to) {
		return "", "", fmt.Errorf("destination path must be vault-relative: %s", to)
	}
	if pathEscapesVault(from) {
		return "", "", fmt.Errorf("source path escapes vault: %s", from)
	}
	if pathEscapesVault(to) {
		return "", "", fmt.Errorf("destination path escapes vault: %s", to)
	}
	if from == to {
		return "", "", fmt.Errorf("source and destination are the same: %s", from)
	}
	return from, to, nil
}
