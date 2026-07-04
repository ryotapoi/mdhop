// meta_validate.go implements schema-conformance checks for frontmatter:
// required keys plus declared type and enum constraints. Reference-existence
// checks for frontmatter values belong to meta_check.go per ADR 0019.
package core

import (
	"fmt"
)

// MetaValidateOptions controls meta-validate behavior.
type MetaValidateOptions struct {
	Require []string // keys that must be present with a non-empty value
	Path    []string // include globs for source notes (empty = all)
	Exclude []string // exclude globs for source notes
}

// MetaViolationReason classifies a schema violation.
type MetaViolationReason string

const (
	// ReasonMissing: a --require key has no non-empty value on the note.
	ReasonMissing MetaViolationReason = "missing"
	// ReasonType: the value cannot be parsed as its declared meta.types type
	// (date/number/semver).
	ReasonType MetaViolationReason = "type"
	// ReasonEnum: the value is not one of the declared ordered values.
	ReasonEnum MetaViolationReason = "enum"
)

// MetaViolation is a single meta-validate finding.
type MetaViolation struct {
	SourcePath string              // note holding (or missing) the key
	Key        string              // frontmatter key
	Value      string              // offending value ("" for missing)
	Reason     MetaViolationReason // why it failed
}

// MetaValidateResult holds meta-validate findings.
type MetaValidateResult struct {
	Violations []MetaViolation
}

// MetaValidate checks that frontmatter conforms to the declared schema:
// required keys are present (--require) and typed keys (meta.types) hold values
// that parse as their declared type / belong to their ordered value list.
//
// Empty and null frontmatter values are dropped at index time, so a --require
// key written with no value is reported as "missing" (same defect as an absent
// key from the user's point of view).
//
// Type/enum checks read the value_type stored at index time, so the DB must be
// up to date with the current meta.types declarations: changing meta.types
// without rebuilding makes a newly typed key report every value as a type/enum
// violation. This follows mdhop's "rebuild the DB on change" model
// (docs/rules/03-data-model.md), the same assumption every read command relies on.
func MetaValidate(vaultPath string, opts MetaValidateOptions) (*MetaValidateResult, error) {
	if err := validateGlobPatterns(opts.Path); err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(opts.Exclude); err != nil {
		return nil, err
	}

	cfg, err := LoadConfig(vaultPath)
	if err != nil {
		return nil, err
	}

	// Typed keys worth checking: those whose declared type is not plain string.
	// String/undeclared keys carry no type or enum constraint.
	typedKeys := make(map[string]MetaTypeInfo)
	for key, info := range cfg.Meta.Types {
		if info.Name != MetaTypeString {
			typedKeys[key] = info
		}
	}
	if len(opts.Require) == 0 && len(cfg.Meta.Profiles) == 0 && len(typedKeys) == 0 {
		return nil, fmt.Errorf("meta-validate: nothing to check (give --require, declare meta.profiles, or declare meta.types)")
	}

	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ef := &ExcludeFilter{PathGlobs: opts.Exclude}
	inclSQL, inclArgs := pathIncludeSQL("n.path", opts.Path)
	exclSQL, exclArgs := ef.PathExcludeSQL("n.path")

	result := &MetaValidateResult{}

	if err := validateRequired(db, opts.Require, inclSQL, inclArgs, exclSQL, exclArgs, result); err != nil {
		return nil, err
	}
	if err := validateRequiredProfiles(db, cfg.Meta.Profiles, inclSQL, inclArgs, exclSQL, exclArgs, result); err != nil {
		return nil, err
	}
	if err := validateTypes(db, typedKeys, inclSQL, inclArgs, exclSQL, exclArgs, result); err != nil {
		return nil, err
	}
	deduplicateMetaViolations(result)
	return result, nil
}

// validateRequired reports a "missing" violation for every note (matching the
// path filter) that has no non-empty value for a required key.
func validateRequired(db dbExecer, require []string, inclSQL string, inclArgs []any, exclSQL string, exclArgs []any, result *MetaValidateResult) error {
	for _, key := range require {
		if err := validateRequiredKey(db, key, inclSQL, inclArgs, exclSQL, exclArgs, result); err != nil {
			return err
		}
	}
	return nil
}

func validateRequiredProfiles(db dbExecer, profiles []MetaRequireProfile, baseInclSQL string, baseInclArgs []any, exclSQL string, exclArgs []any, result *MetaValidateResult) error {
	for _, profile := range profiles {
		inclSQL := baseInclSQL
		inclArgs := append([]any{}, baseInclArgs...)
		if profile.Path != "" {
			profileSQL, profileArgs := pathIncludeSQL("n.path", []string{profile.Path})
			inclSQL += profileSQL
			inclArgs = append(inclArgs, profileArgs...)
		}
		if err := validateRequired(db, profile.Require, inclSQL, inclArgs, exclSQL, exclArgs, result); err != nil {
			return err
		}
	}
	return nil
}

func validateRequiredKey(db dbExecer, key string, inclSQL string, inclArgs []any, exclSQL string, exclArgs []any, result *MetaValidateResult) error {
	query := `SELECT n.path
		FROM nodes n
		WHERE n.type='note' AND n.exists_flag=1` + inclSQL + exclSQL + `
		  AND NOT EXISTS (SELECT 1 FROM meta m WHERE m.node_id = n.id AND m.key = ?)
		ORDER BY n.path`
	args := append(append(append([]any{}, inclArgs...), exclArgs...), key)
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return err
		}
		result.Violations = append(result.Violations, MetaViolation{
			SourcePath: path,
			Key:        key,
			Reason:     ReasonMissing,
		})
	}
	return rows.Err()
}

func deduplicateMetaViolations(result *MetaValidateResult) {
	if len(result.Violations) < 2 {
		return
	}
	type violationKey struct {
		sourcePath string
		key        string
		reason     MetaViolationReason
	}
	seen := make(map[violationKey]bool, len(result.Violations))
	deduped := result.Violations[:0]
	for _, v := range result.Violations {
		key := violationKey{sourcePath: v.SourcePath, key: v.Key, reason: v.Reason}
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, v)
	}
	result.Violations = deduped
}

// validateTypes reports type/enum violations for typed keys. A value violates
// its declared type when it fails normalization at index time: insertMetaEntries
// stores value_type=string on normalization failure, so a stored value_type that
// differs from the declared type marks the offending value.
func validateTypes(db dbExecer, typedKeys map[string]MetaTypeInfo, inclSQL string, inclArgs []any, exclSQL string, exclArgs []any, result *MetaValidateResult) error {
	if len(typedKeys) == 0 {
		return nil
	}
	query := `SELECT n.path, m.key, m.value, COALESCE(m.value_type,'')
		FROM meta m
		JOIN nodes n ON n.id = m.node_id
		WHERE n.type='note' AND n.exists_flag=1` + inclSQL + exclSQL + `
		ORDER BY n.path, m.key, m.value`
	args := append(append([]any{}, inclArgs...), exclArgs...)
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var path, key, value, storedType string
		if err := rows.Scan(&path, &key, &value, &storedType); err != nil {
			return err
		}
		info, ok := typedKeys[key]
		if !ok {
			continue
		}
		if storedType == string(info.Name) {
			continue // normalized successfully → conforms
		}
		reason := ReasonType
		if info.Name == MetaTypeOrdered {
			reason = ReasonEnum
		}
		result.Violations = append(result.Violations, MetaViolation{
			SourcePath: path,
			Key:        key,
			Value:      value,
			Reason:     reason,
		})
	}
	return rows.Err()
}
