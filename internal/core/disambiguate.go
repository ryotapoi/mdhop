package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DisambiguateOptions controls which basename links to rewrite.
type DisambiguateOptions struct {
	Name   string   // basename to disambiguate (required)
	Target string   // target file path (required if multiple candidates)
	Files  []string // limit rewriting to these source files
}

// DisambiguateResult reports the outcome of the disambiguate operation.
type DisambiguateResult struct {
	Rewritten []RewrittenLink
}

// Disambiguate rewrites basename links to full paths for the given basename.
func Disambiguate(vaultPath string, opts DisambiguateOptions) (*DisambiguateResult, error) {
	// DB existence check.
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	// Open DB.
	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Find candidate notes matching the basename.
	nameKey := strings.TrimSuffix(strings.ToLower(opts.Name), ".md")

	rows, err := db.Query("SELECT id, path FROM nodes WHERE type='note' AND exists_flag=1")
	if err != nil {
		return nil, err
	}
	type candidate struct {
		id   int64
		path string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.path); err != nil {
			rows.Close()
			return nil, err
		}
		if basenameKey(c.path) == nameKey {
			candidates = append(candidates, c)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Determine target.
	var candidatePaths []string
	for _, c := range candidates {
		candidatePaths = append(candidatePaths, c.path)
	}
	targetPath, err := resolveDisambiguateTarget(opts.Name, candidatePaths, opts.Target)
	if err != nil {
		return nil, err
	}
	var target candidate
	for _, c := range candidates {
		if c.path == targetPath {
			target = c
			break
		}
	}

	// Validate --file flags.
	fileScope := make(map[string]bool)
	for _, f := range opts.Files {
		np := normalizePath(f)
		// Check that the file is registered as a note.
		var id int64
		err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ? AND type = 'note'",
			noteKey(np)).Scan(&id)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("file not registered: %s", np)
		}
		if err != nil {
			return nil, err
		}
		fileScope[np] = true
	}

	// Get incoming edges (basename links pointing to target).
	edgeRows, err := db.Query(
		`SELECT e.id, e.raw_link, e.link_type, e.line_start, sn.path, sn.id
		 FROM edges e JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
		 WHERE e.target_id = ? AND e.link_type IN ('wikilink', 'markdown')`, target.id)
	if err != nil {
		return nil, err
	}
	var rewrites []rewriteEntry
	for edgeRows.Next() {
		var re rewriteEntry
		if err := edgeRows.Scan(&re.edgeID, &re.rawLink, &re.linkType, &re.lineStart, &re.sourcePath, &re.sourceID); err != nil {
			edgeRows.Close()
			return nil, err
		}
		// Filter: basename links only.
		if !isBasenameRawLink(re.rawLink, re.linkType) {
			continue
		}
		// Filter: skip self-references.
		if re.sourcePath == target.path {
			continue
		}
		// Filter: --file scope.
		if len(fileScope) > 0 && !fileScope[re.sourcePath] {
			continue
		}
		// Compute new raw link.
		newRawLink := rewriteRawLink(re.rawLink, re.linkType, target.path)
		if newRawLink == re.rawLink {
			continue // no change needed
		}
		re.newRawLink = newRawLink
		rewrites = append(rewrites, re)
	}
	edgeRows.Close()
	if err := edgeRows.Err(); err != nil {
		return nil, err
	}

	// No rewrites needed.
	if len(rewrites) == 0 {
		return &DisambiguateResult{}, nil
	}

	// Stale check on source files.
	sourceStaleChecked := make(map[int64]bool)
	for _, re := range rewrites {
		if sourceStaleChecked[re.sourceID] {
			continue
		}
		sourceStaleChecked[re.sourceID] = true
		var dbMtime int64
		err := db.QueryRow("SELECT mtime FROM nodes WHERE id = ?", re.sourceID).Scan(&dbMtime)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(filepath.Join(vaultPath, re.sourcePath))
		if err != nil {
			return nil, err
		}
		if info.ModTime().Unix() != dbMtime {
			return nil, fmt.Errorf("source file is stale: %s", re.sourcePath)
		}
	}

	// Apply disk rewrites.
	groups := make(map[string][]rewriteEntry)
	for _, re := range rewrites {
		groups[re.sourcePath] = append(groups[re.sourcePath], re)
	}
	newMtimes, backups, applyErr := applyFileRewrites(vaultPath, groups)
	if applyErr != nil {
		return nil, applyErr
	}

	// DB transaction — update edges and mtimes.
	tx, err := db.Begin()
	if err != nil {
		restoreBackups(vaultPath, backups)
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
			restoreBackups(vaultPath, backups)
		}
	}()

	result := &DisambiguateResult{}
	for _, re := range rewrites {
		if _, err := tx.Exec("UPDATE edges SET raw_link = ? WHERE id = ?", re.newRawLink, re.edgeID); err != nil {
			return nil, err
		}
		result.Rewritten = append(result.Rewritten, RewrittenLink{
			File:    re.sourcePath,
			OldLink: re.rawLink,
			NewLink: re.newRawLink,
		})
	}

	// Update source mtime in DB.
	mtimeUpdated := make(map[int64]bool)
	for _, re := range rewrites {
		if mtimeUpdated[re.sourceID] {
			continue
		}
		mtimeUpdated[re.sourceID] = true
		mt := newMtimes[re.sourceID]
		if _, err := tx.Exec("UPDATE nodes SET mtime = ? WHERE id = ? AND type = 'note'", mt, re.sourceID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return result, nil
}

// resolveDisambiguateTarget picks the target path from candidates.
// If target is specified, it normalizes and matches against candidates.
// If target is empty and there is exactly one candidate, it auto-selects.
// Otherwise it returns an error listing candidates.
func resolveDisambiguateTarget(name string, candidates []string, target string) (string, error) {
	if len(candidates) == 0 {
		return "", fmt.Errorf("no note found with basename: %s", name)
	}
	if target != "" {
		normalized := normalizePath(target)
		normalizedMd := normalized
		if !strings.HasSuffix(strings.ToLower(normalized), ".md") {
			normalizedMd = normalized + ".md"
		}
		for _, c := range candidates {
			if c == normalized || c == normalizedMd {
				return c, nil
			}
		}
		return "", fmt.Errorf("target not found among candidates: %s", target)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	return "", fmt.Errorf("multiple candidates for basename %s, --target is required: %s",
		name, strings.Join(candidates, ", "))
}

// DisambiguateScan rewrites basename links to full paths without using the DB.
// It scans all .md files in the vault directly.
func DisambiguateScan(vaultPath string, opts DisambiguateOptions) (*DisambiguateResult, error) {
	// Collect all .md files.
	files, err := collectMarkdownFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}

	// Find candidates matching the basename.
	nameKey := strings.TrimSuffix(strings.ToLower(opts.Name), ".md")

	var candidates []string
	for _, f := range files {
		if basenameKey(f) == nameKey {
			candidates = append(candidates, f)
		}
	}

	// Determine target.
	targetPath, err := resolveDisambiguateTarget(opts.Name, candidates, opts.Target)
	if err != nil {
		return nil, err
	}

	// Validate --file flags.
	fileScope := make(map[string]bool)
	for _, f := range opts.Files {
		np := normalizePath(f)
		if !fileSet[np] {
			return nil, fmt.Errorf("file not found: %s", np)
		}
		fileScope[np] = true
	}

	// Scan files for basename links matching nameKey.
	scanFiles := files
	if len(fileScope) > 0 {
		scanFiles = nil
		for _, f := range files {
			if fileScope[f] {
				scanFiles = append(scanFiles, f)
			}
		}
	}

	var rewrites []rewriteEntry
	for _, sourcePath := range scanFiles {
		// Skip self-references (source is the target itself).
		if sourcePath == targetPath {
			continue
		}

		content, err := os.ReadFile(filepath.Join(vaultPath, sourcePath))
		if err != nil {
			return nil, err
		}

		links := parseLinks(string(content))
		for _, lo := range links {
			if lo.linkType != "wikilink" && lo.linkType != "markdown" {
				continue
			}
			if !lo.isBasename {
				continue
			}
			if basenameKey(lo.target) != nameKey {
				continue
			}

			newRawLink := rewriteRawLink(lo.rawLink, lo.linkType, targetPath)
			if newRawLink == lo.rawLink {
				continue
			}

			rewrites = append(rewrites, rewriteEntry{
				rawLink:    lo.rawLink,
				linkType:   lo.linkType,
				lineStart:  lo.lineStart,
				sourcePath: sourcePath,
				sourceID:   0,
				newRawLink: newRawLink,
			})
		}
	}

	if len(rewrites) == 0 {
		return &DisambiguateResult{}, nil
	}

	// Apply disk rewrites.
	groups := make(map[string][]rewriteEntry)
	for _, re := range rewrites {
		groups[re.sourcePath] = append(groups[re.sourcePath], re)
	}
	_, _, applyErr := applyFileRewrites(vaultPath, groups)
	if applyErr != nil {
		return nil, applyErr
	}

	// Build result.
	result := &DisambiguateResult{}
	for _, re := range rewrites {
		result.Rewritten = append(result.Rewritten, RewrittenLink{
			File:    re.sourcePath,
			OldLink: re.rawLink,
			NewLink: re.newRawLink,
		})
	}

	return result, nil
}
