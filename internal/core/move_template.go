package core

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// MoveTemplateOptions controls destination expansion for move --to-template.
type MoveTemplateOptions struct {
	From      string // vault-relative source note path or directory
	Template  string // destination template
	Directory bool   // expand every registered note under From as a directory prefix
}

// MoveTemplatePlanResult reports the move plan for a move --to-template run.
type MoveTemplatePlanResult struct {
	Moved []MovedFile
}

// ExpandMoveTemplate expands a move destination template from the source note's
// indexed frontmatter values and basename. The returned path is vault-relative
// and validated with the same containment rules as Move destinations.
func ExpandMoveTemplate(vaultPath string, opts MoveTemplateOptions) (string, error) {
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return "", err
	}
	defer db.Close()

	from, err := validateMoveTemplateOptions(opts)
	if err != nil {
		return "", err
	}

	nodeID, err := lookupTemplateSourceNote(db, from)
	if err != nil {
		return "", err
	}
	meta, err := queryMetaByNode(db, nodeID)
	if err != nil {
		return "", err
	}
	values := metaRowsByKey(meta)

	to, err := expandMoveTemplate(opts.Template, values, filepath.Base(from))
	if err != nil {
		return "", err
	}
	if err := validateExpandedMoveTemplatePath(to); err != nil {
		return "", err
	}
	to = NormalizePath(to)
	if err := validateExpandedMoveTemplatePath(to); err != nil {
		return "", err
	}
	return to, nil
}

// PlanMoveTemplate expands and validates move --to-template destinations without
// changing disk or DB state.
func PlanMoveTemplate(vaultPath string, opts MoveTemplateOptions) (*MoveTemplatePlanResult, error) {
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	prepared, err := prepareMoveTemplate(vaultPath, db, opts)
	if err != nil {
		return nil, err
	}
	return &MoveTemplatePlanResult{Moved: movedFilesFromMoves(prepared.moves)}, nil
}

// MoveTemplate expands move --to-template destinations and executes the planned
// note moves as a single all-or-nothing batch.
func MoveTemplate(vaultPath string, opts MoveTemplateOptions) (*MoveDirResult, error) {
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	cfg, err := LoadConfig(vaultPath)
	if err != nil {
		return nil, err
	}

	prepared, err := prepareMoveTemplate(vaultPath, db, opts)
	if err != nil {
		return nil, err
	}
	return executeMoves(vaultPath, db, cfg, prepared.moves, nil, prepared.needDiskMove)
}

type preparedMoveTemplate struct {
	moves        []moveInfo
	needDiskMove bool
}

func prepareMoveTemplate(vaultPath string, db dbExecer, opts MoveTemplateOptions) (*preparedMoveTemplate, error) {
	moves, err := loadMoveTemplateMovesFromDB(db, opts)
	if err != nil {
		return nil, err
	}
	if err := checkDuplicateMoveDestinations(moves); err != nil {
		return nil, err
	}
	if err := checkDestinationsFree(db, moves); err != nil {
		return nil, err
	}
	needDiskMove, err := classifyDiskState(vaultPath, moves)
	if err != nil {
		return nil, err
	}
	if err := checkMovedFilesNotStale(vaultPath, moves, needDiskMove); err != nil {
		return nil, err
	}
	return &preparedMoveTemplate{moves: moves, needDiskMove: needDiskMove}, nil
}

func validateMoveTemplateOptions(opts MoveTemplateOptions) (string, error) {
	from := NormalizePath(opts.From)
	if opts.Template == "" {
		return "", fmt.Errorf("--to-template is required")
	}
	if filepath.IsAbs(from) {
		return "", fmt.Errorf("source path must be vault-relative: %s", from)
	}
	if pathEscapesVault(from) {
		return "", fmt.Errorf("source path escapes vault: %s", from)
	}
	return from, nil
}

func loadMoveTemplateMovesFromDB(db dbExecer, opts MoveTemplateOptions) ([]moveInfo, error) {
	from, err := validateMoveTemplateOptions(opts)
	if err != nil {
		return nil, err
	}
	if opts.Directory {
		fromDir := strings.TrimSuffix(from, "/")
		paths, err := listDirNodesByType(db, fromDir, NodeTypeNote)
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("no notes registered under directory for --to-template: %s", fromDir)
		}
		sort.Strings(paths)
		moves := make([]moveInfo, 0, len(paths))
		for _, path := range paths {
			move, err := loadMoveTemplateMoveFromDB(db, path, opts.Template)
			if err != nil {
				return nil, err
			}
			moves = append(moves, move)
		}
		return moves, nil
	}
	move, err := loadMoveTemplateMoveFromDB(db, from, opts.Template)
	if err != nil {
		return nil, err
	}
	return []moveInfo{move}, nil
}

func loadMoveTemplateMoveFromDB(db dbExecer, from, template string) (moveInfo, error) {
	nodeID, err := lookupTemplateSourceNote(db, from)
	if err != nil {
		return moveInfo{}, err
	}
	meta, err := queryMetaByNode(db, nodeID)
	if err != nil {
		return moveInfo{}, err
	}
	to, err := expandMoveTemplate(template, metaRowsByKey(meta), filepath.Base(from))
	if err != nil {
		return moveInfo{}, err
	}
	if err := validateExpandedMoveTemplatePath(to); err != nil {
		return moveInfo{}, err
	}
	to = NormalizePath(to)
	if err := validateExpandedMoveTemplatePath(to); err != nil {
		return moveInfo{}, err
	}
	if from == to {
		return moveInfo{}, fmt.Errorf("source and destination are the same: %s", from)
	}
	var dbMtime int64
	if err := db.QueryRow("SELECT mtime FROM nodes WHERE id = ?", nodeID).Scan(&dbMtime); err != nil {
		return moveInfo{}, err
	}
	return moveInfo{from: from, to: to, nodeID: nodeID, dbMtime: dbMtime}, nil
}

func lookupTemplateSourceNote(db dbExecer, from string) (int64, error) {
	var id int64
	err := db.QueryRow(
		"SELECT id FROM nodes WHERE path = ? AND type = 'note' AND exists_flag = 1",
		from,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	return 0, templateSourceNoteError(err, from)
}

func templateSourceNoteError(err error, from string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("source must be a registered note for --to-template: %s", from)
	}
	return err
}

func metaRowsByKey(rows []MetaRow) map[string][]MetaRow {
	values := make(map[string][]MetaRow, len(rows))
	for _, row := range rows {
		values[row.Key] = append(values[row.Key], row)
	}
	return values
}

func expandMoveTemplate(template string, values map[string][]MetaRow, basenameValue string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(template); {
		switch template[i] {
		case '{':
			end := strings.IndexByte(template[i+1:], '}')
			if end < 0 {
				return "", fmt.Errorf("unterminated template placeholder")
			}
			expr := template[i+1 : i+1+end]
			expanded, err := expandMoveTemplatePlaceholder(expr, values, basenameValue)
			if err != nil {
				return "", err
			}
			out.WriteString(expanded)
			i += end + 2
		case '}':
			return "", fmt.Errorf("unmatched template placeholder close")
		default:
			out.WriteByte(template[i])
			i++
		}
	}
	return out.String(), nil
}

func expandMoveTemplatePlaceholder(expr string, values map[string][]MetaRow, basenameValue string) (string, error) {
	if expr == "" {
		return "", fmt.Errorf("empty template placeholder")
	}
	if strings.Contains(expr, "{") {
		return "", fmt.Errorf("nested template placeholder")
	}
	fieldSpec, fallback, hasFallback := strings.Cut(expr, "|")
	field, part, hasPart := strings.Cut(fieldSpec, ":")
	if field == "" {
		return "", fmt.Errorf("empty template field")
	}
	if field == "basename" {
		if hasFallback || hasPart {
			return "", fmt.Errorf("{basename} does not support fallback or date extraction")
		}
		return basenameValue, nil
	}
	fieldRows, ok := values[field]
	if !ok {
		if hasFallback {
			if err := validateExpandedMoveTemplatePlaceholderValue(field, fallback); err != nil {
				return "", err
			}
			return fallback, nil
		}
		return "", fmt.Errorf("template field missing: %s", field)
	}
	if len(fieldRows) != 1 {
		return "", fmt.Errorf("template field has multiple values: %s", field)
	}
	value := fieldRows[0].Value
	if !hasPart {
		if err := validateExpandedMoveTemplatePlaceholderValue(field, value); err != nil {
			return "", err
		}
		return value, nil
	}
	if part != "year" && part != "month" && part != "day" {
		return "", fmt.Errorf("unsupported template date extraction: %s", part)
	}
	dateValue := fieldRows[0].SortValue
	if dateValue == "" {
		dateValue = value
	}
	normalized, warning := normalizeDate(dateValue)
	if warning != "" {
		return "", fmt.Errorf("template field %s cannot be parsed as date for %s extraction: %q", field, part, value)
	}
	switch part {
	case "year":
		return normalized[:4], nil
	case "month":
		return normalized[5:7], nil
	default:
		return normalized[8:10], nil
	}
}

func validateExpandedMoveTemplatePlaceholderValue(field, value string) error {
	if strings.Contains(value, "/") {
		return fmt.Errorf("expanded template placeholder value contains /: %s", field)
	}
	return nil
}

func validateExpandedMoveTemplatePath(path string) error {
	if path == "" || path == "." {
		return fmt.Errorf("expanded destination path is empty")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("expanded destination path must be vault-relative: %s", path)
	}
	if pathEscapesVault(path) {
		return fmt.Errorf("expanded destination path escapes vault: %s", path)
	}
	if strings.HasSuffix(path, "/") {
		return fmt.Errorf("expanded destination path must be a file path: %s", path)
	}
	return nil
}

func checkDuplicateMoveDestinations(moves []moveInfo) error {
	seen := make(map[string]string, len(moves))
	for _, m := range moves {
		if previous, ok := seen[m.to]; ok {
			return fmt.Errorf("duplicate expanded destination path: %s (from %s and %s)", m.to, previous, m.from)
		}
		seen[m.to] = m.from
	}
	return nil
}

func movedFilesFromMoves(moves []moveInfo) []MovedFile {
	moved := make([]MovedFile, 0, len(moves))
	for _, m := range moves {
		moved = append(moved, MovedFile{From: m.from, To: m.to})
	}
	return moved
}
