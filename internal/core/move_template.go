package core

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
)

// MoveTemplateOptions controls destination expansion for move --to-template.
type MoveTemplateOptions struct {
	From     string // vault-relative source note path
	Template string // destination template
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
	values := metaValuesByKey(meta)

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

func lookupTemplateSourceNote(db dbExecer, from string) (int64, error) {
	var id int64
	err := db.QueryRow(
		"SELECT id FROM nodes WHERE path = ? AND type = 'note' AND exists_flag = 1",
		from,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("source must be a registered note for --to-template: %s", from)
	}
	return 0, err
}

func metaValuesByKey(rows []MetaRow) map[string][]string {
	values := make(map[string][]string, len(rows))
	for _, row := range rows {
		values[row.Key] = append(values[row.Key], row.Value)
	}
	return values
}

func expandMoveTemplate(template string, values map[string][]string, basenameValue string) (string, error) {
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

func expandMoveTemplatePlaceholder(expr string, values map[string][]string, basenameValue string) (string, error) {
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
	fieldValues, ok := values[field]
	if !ok {
		if hasFallback {
			return fallback, nil
		}
		return "", fmt.Errorf("template field missing: %s", field)
	}
	if len(fieldValues) != 1 {
		return "", fmt.Errorf("template field has multiple values: %s", field)
	}
	value := fieldValues[0]
	if !hasPart {
		return value, nil
	}
	if part != "year" {
		return "", fmt.Errorf("unsupported template date extraction: %s", part)
	}
	normalized, warning := normalizeDate(value)
	if warning != "" {
		return "", fmt.Errorf("template field %s cannot be parsed as date for year extraction: %q", field, value)
	}
	return normalized[:4], nil
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
