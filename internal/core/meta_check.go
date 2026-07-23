// meta_check.go implements reference-existence checks for frontmatter values.
// Schema-conformance checks for required keys, types, and enums belong to
// meta_validate.go per ADR 0019.
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MetaValueKind controls how meta-check interprets a frontmatter value.
type MetaValueKind string

const (
	// MetaKindPath treats values as raw path strings (markdown-link semantics:
	// "./"/"../" note-relative, "/" vault-relative, bare name basename).
	MetaKindPath MetaValueKind = "path"
	// MetaKindWikilink treats values as [[wikilink]] strings.
	MetaKindWikilink MetaValueKind = "wikilink"
)

// MetaCheckOptions controls meta-check behavior.
type MetaCheckOptions struct {
	Keys    []string      // frontmatter keys to check (required, non-empty)
	Kind    MetaValueKind // how to interpret values
	Path    []string      // include globs for source notes (empty = all)
	Exclude []string      // exclude globs for source notes
}

// MetaIssueReason classifies why a meta value failed the check.
type MetaIssueReason string

const (
	ReasonNotFound    MetaIssueReason = "not_found"    // path/wikilink does not resolve
	ReasonAmbiguous   MetaIssueReason = "ambiguous"    // basename resolves to multiple notes
	ReasonVaultEscape MetaIssueReason = "vault_escape" // path escapes the vault
	ReasonNotWikilink MetaIssueReason = "not_wikilink" // kind=wikilink but value is not [[...]]
)

// MetaIssue is a single meta-check finding.
type MetaIssue struct {
	SourcePath string          // note holding the frontmatter key
	Key        string          // frontmatter key
	Value      string          // the raw value that failed
	Reason     MetaIssueReason // why it failed
}

// MetaCheckResult holds meta-check findings.
type MetaCheckResult struct {
	Issues []MetaIssue
}

// MetaCheck verifies that the values of the given frontmatter keys resolve to
// existing vault paths. URL values (containing "://") are allowed and skipped.
func MetaCheck(vaultPath string, opts MetaCheckOptions) (*MetaCheckResult, error) {
	if len(opts.Keys) == 0 {
		return nil, fmt.Errorf("at least one --key is required")
	}
	if opts.Kind != MetaKindPath && opts.Kind != MetaKindWikilink {
		return nil, fmt.Errorf("invalid kind %q (must be path or wikilink)", opts.Kind)
	}
	if err := validateGlobPatterns(opts.Path); err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(opts.Exclude); err != nil {
		return nil, err
	}

	files, err := collectMarkdownFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	cfg, err := LoadConfig(vaultPath)
	if err != nil {
		return nil, err
	}
	files = filterBuildExcludes(files, cfg.Build.ExcludePaths)
	assetFiles, err := collectAssetFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	assetFiles = filterBuildExcludes(assetFiles, cfg.Build.ExcludePaths)
	rm := newResolveMaps(files, assetFiles)

	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	keySet := make(map[string]bool, len(opts.Keys))
	for _, k := range opts.Keys {
		keySet[k] = true
	}

	// Fetch (source path, key, value) rows for the requested keys from notes
	// matching the path filter.
	ef := &ExcludeFilter{PathGlobs: opts.Exclude}
	inclSQL, inclArgs := pathIncludeSQL("n.path", opts.Path)
	exclSQL, exclArgs := ef.PathExcludeSQL("n.path")
	query := `SELECT n.path, m.key, m.value
		FROM meta m
		JOIN nodes n ON n.id = m.node_id
		WHERE n.type='note' AND n.exists_flag=1` + inclSQL + exclSQL + `
		ORDER BY n.path, m.key, m.value`
	args := append(append([]any{}, inclArgs...), exclArgs...)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &MetaCheckResult{}
	for rows.Next() {
		var srcPath, key, value string
		if err := rows.Scan(&srcPath, &key, &value); err != nil {
			return nil, err
		}
		if !keySet[key] {
			continue
		}
		if issue, ok := checkMetaValue(vaultPath, srcPath, key, value, opts.Kind, rm); ok {
			result.Issues = append(result.Issues, issue)
		}
	}
	return result, rows.Err()
}

// checkMetaValue resolves a single value and returns a MetaIssue if it fails.
// ok is false when the value is valid or intentionally skipped (URL/empty).
func checkMetaValue(vaultPath, srcPath, key, value string, kind MetaValueKind, rm *resolveMaps) (MetaIssue, bool) {
	v := strings.TrimSpace(value)
	if v == "" || strings.Contains(v, "://") {
		return MetaIssue{}, false // empty or URL: allowed
	}

	issue := MetaIssue{SourcePath: srcPath, Key: key, Value: value}

	var occ linkOccur
	switch kind {
	case MetaKindWikilink:
		links := parseWikiLinks(v, 0)
		if len(links) == 0 {
			issue.Reason = ReasonNotWikilink
			return issue, true
		}
		occ = links[0]
	default: // MetaKindPath
		if strings.HasSuffix(v, "/") {
			if ok, escaped := directoryMetaValueExists(vaultPath, srcPath, v); ok {
				return MetaIssue{}, false
			} else if escaped {
				issue.Reason = ReasonVaultEscape
				return issue, true
			}
			issue.Reason = ReasonNotFound
			return issue, true
		}
		var ok bool
		occ, ok = frontmatterPathOccur(v, 0)
		if !ok {
			// frontmatterPathOccur skips URLs (handled above) and wikilinks.
			issue.Reason = ReasonNotFound
			return issue, true
		}
	}

	// Ambiguous basename: resolves to multiple notes.
	if occ.isBasename && isAmbiguousBasenameLink(occ.target, rm) {
		issue.Reason = ReasonAmbiguous
		return issue, true
	}

	resolved, err := resolveFrontmatterPathDry(srcPath, occ, rm)
	if err != nil {
		issue.Reason = ReasonVaultEscape
		return issue, true
	}
	if resolved == "" {
		issue.Reason = ReasonNotFound
		return issue, true
	}
	return MetaIssue{}, false
}

func directoryMetaValueExists(vaultPath, srcPath, value string) (exists bool, escaped bool) {
	target := normalizeTextNFC(value)
	var resolved string
	if isRelativePath(target) {
		if escapesVault(srcPath, target) {
			return false, true
		}
		resolved = NormalizePath(filepath.Join(filepath.Dir(srcPath), target))
	} else {
		if pathEscapesVault(target) {
			return false, true
		}
		resolved = NormalizePath(strings.TrimPrefix(target, "/"))
	}
	info, err := os.Stat(filepath.Join(vaultPath, resolved))
	return err == nil && info.IsDir(), false
}
