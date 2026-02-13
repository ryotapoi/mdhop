package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AddOptions controls which files to add to the index.
type AddOptions struct {
	Files            []string
	AutoDisambiguate bool
}

// RewrittenLink records a single link rewrite performed by auto-disambiguate.
type RewrittenLink struct {
	File    string
	OldLink string
	NewLink string
}

// AddResult reports the outcome of the add operation.
type AddResult struct {
	Added     []string        // files added as new notes
	Promoted  []string        // phantom nodes promoted to notes
	Rewritten []RewrittenLink // links rewritten by auto-disambiguate
}

// RewriteBackup holds original file content for rollback on failure.
type rewriteBackup struct {
	path    string
	content []byte
}

// rewriteEntry holds information needed to rewrite a single edge.
type rewriteEntry struct {
	edgeID     int64
	rawLink    string
	linkType   string
	lineStart  int
	sourcePath string
	sourceID   int64
	newRawLink string
}

// Add inserts new files into the existing index DB.
func Add(vaultPath string, opts AddOptions) (*AddResult, error) {
	// Step 1: DB existence check.
	dbp := dbPath(vaultPath)
	if _, err := os.Stat(dbp); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found: run 'mdhop build' first")
	}

	// Step 3: normalize and deduplicate input paths.
	type addFile struct {
		path  string
		mtime int64
	}
	seen := make(map[string]bool)
	var files []addFile
	for _, f := range opts.Files {
		np := normalizePath(f)
		if seen[np] {
			continue
		}
		seen[np] = true
		files = append(files, addFile{path: np})
	}

	db, err := openDBAt(dbp)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Step 4: check that no file is already registered.
	for _, f := range files {
		key := noteKey(f.path)
		var id int64
		err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ? AND type = 'note'", key).Scan(&id)
		if err == nil {
			return nil, fmt.Errorf("file already registered: %s", f.path)
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	// Step 5: check disk existence and collect mtime.
	for i := range files {
		info, err := os.Stat(filepath.Join(vaultPath, files[i].path))
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", files[i].path)
		}
		if err != nil {
			return nil, err
		}
		files[i].mtime = info.ModTime().Unix()
	}

	// Step 6: build maps from DB + save oldBasenameCounts copy.
	pathToID, pathSet, basenameCounts, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}
	oldBasenameCounts := make(map[string]int, len(basenameCounts))
	for k, v := range basenameCounts {
		oldBasenameCounts[k] = v
	}

	// Step 7: adjust maps for post-add state.
	for _, f := range files {
		rel := strings.ToLower(f.path)
		pathSet[rel] = f.path
		noExt := strings.TrimSuffix(f.path, filepath.Ext(f.path))
		pathSet[strings.ToLower(noExt)] = f.path
		bk := basenameKey(f.path)
		basenameCounts[bk]++
	}

	// Step 8: check if adding causes existing links to become ambiguous.
	// Build oldBasenameToPath for pattern A detection.
	oldBasenameToPath := make(map[string]string)
	for bk, count := range oldBasenameCounts {
		if count == 1 {
			for p := range pathToID {
				if basenameKey(p) == bk {
					oldBasenameToPath[bk] = p
					break
				}
			}
		}
	}

	var allRewrites []rewriteEntry

	for bk, newCount := range basenameCounts {
		if newCount <= 1 {
			continue
		}
		oldCount := oldBasenameCounts[bk]
		if oldCount >= 2 {
			continue // already ambiguous before add
		}

		// Find the target node that existing basename links pointed to.
		var targetID int64
		var isPatternA bool
		if oldCount == 1 {
			// Pattern A: existing unique note becomes ambiguous.
			targetID = pathToID[oldBasenameToPath[bk]]
			isPatternA = true
		} else {
			// Pattern B: phantom (oldCount == 0, adding 2+ files with same basename).
			phantomKey := fmt.Sprintf("phantom:name:%s", bk)
			err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", phantomKey).Scan(&targetID)
			if err == sql.ErrNoRows {
				continue // no phantom, so no existing basename links
			}
			if err != nil {
				return nil, err
			}
		}

		// Query edges with source info for potential rewriting.
		rows, err := db.Query(
			`SELECT e.id, e.raw_link, e.link_type, e.line_start, sn.path, sn.id
			 FROM edges e JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
			 WHERE e.target_id = ?
			 AND e.link_type IN ('wikilink', 'markdown')`, targetID)
		if err != nil {
			return nil, err
		}
		var basenameEdges []rewriteEntry
		for rows.Next() {
			var re rewriteEntry
			if err := rows.Scan(&re.edgeID, &re.rawLink, &re.linkType, &re.lineStart, &re.sourcePath, &re.sourceID); err != nil {
				rows.Close()
				return nil, err
			}
			if isBasenameRawLink(re.rawLink, re.linkType) {
				basenameEdges = append(basenameEdges, re)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}

		if len(basenameEdges) == 0 {
			continue
		}

		if isPatternA && opts.AutoDisambiguate {
			// Compute new raw links for each edge.
			targetPath := oldBasenameToPath[bk]
			for i := range basenameEdges {
				basenameEdges[i].newRawLink = rewriteRawLink(basenameEdges[i].rawLink, basenameEdges[i].linkType, basenameEdges[i].sourcePath, targetPath)
			}
			allRewrites = append(allRewrites, basenameEdges...)
		} else {
			// Pattern B or auto-disambiguate not enabled → error.
			return nil, fmt.Errorf("adding files would make existing links ambiguous")
		}
	}

	// Stale check for source files that need rewriting.
	if len(allRewrites) > 0 {
		sourceStaleChecked := make(map[int64]bool)
		for _, re := range allRewrites {
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
	}

	// Step 9: build basenameToPath (includes both existing and new files).
	basenameToPath := make(map[string]string)
	for bk, count := range basenameCounts {
		if count != 1 {
			continue
		}
		// Search existing notes.
		for p := range pathToID {
			if basenameKey(p) == bk {
				basenameToPath[bk] = p
				break
			}
		}
		// Search new files if not found.
		if _, ok := basenameToPath[bk]; !ok {
			for _, f := range files {
				if basenameKey(f.path) == bk {
					basenameToPath[bk] = f.path
					break
				}
			}
		}
	}

	// Step 10: parse all new files and check for ambiguous links.
	type parsedFile struct {
		file  addFile
		links []linkOccur
	}
	var parsed []parsedFile
	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(vaultPath, f.path))
		if err != nil {
			return nil, err
		}
		links := parseLinks(string(content))

		for _, link := range links {
			if link.linkType != "wikilink" && link.linkType != "markdown" {
				continue
			}
			if link.isRelative && escapesVault(f.path, link.target) {
				return nil, fmt.Errorf("link escapes vault: %s in %s", link.rawLink, f.path)
			}
			if link.isBasename && basenameCounts[strings.ToLower(link.target)] > 1 {
				return nil, fmt.Errorf("ambiguous link: %s in %s", link.target, f.path)
			}
		}

		parsed = append(parsed, parsedFile{file: f, links: links})
	}

	// Apply disk rewrites before transaction (so DB rollback is safe).
	// newMtimes maps sourceID → new mtime after file write.
	var newMtimes map[int64]int64
	var backups []rewriteBackup
	if len(allRewrites) > 0 {
		// Group rewrites by source file.
		groups := make(map[string][]rewriteEntry)
		for _, re := range allRewrites {
			groups[re.sourcePath] = append(groups[re.sourcePath], re)
		}
		var applyErr error
		newMtimes, backups, applyErr = applyFileRewrites(vaultPath, groups)
		if applyErr != nil {
			return nil, applyErr
		}
	}

	// Step 11: begin transaction.
	tx, err := db.Begin()
	if err != nil {
		// Restore disk changes if transaction start fails.
		restoreBackups(vaultPath, backups)
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
			// Restore disk changes on failure (best-effort).
			restoreBackups(vaultPath, backups)
		}
	}()

	result := &AddResult{}

	// Step 12: insert all note nodes.
	for _, pf := range parsed {
		name := basename(pf.file.path)
		id, err := upsertNote(tx, pf.file.path, name, pf.file.mtime)
		if err != nil {
			return nil, err
		}
		pathToID[pf.file.path] = id
		result.Added = append(result.Added, pf.file.path)
	}

	// Step 13: phantom → note promotion.
	for _, pf := range parsed {
		bk := basenameKey(pf.file.path)
		phantomKey := fmt.Sprintf("phantom:name:%s", bk)
		var phantomID int64
		err := tx.QueryRow("SELECT id FROM nodes WHERE node_key = ?", phantomKey).Scan(&phantomID)
		if err == sql.ErrNoRows {
			continue // no phantom to promote
		}
		if err != nil {
			return nil, err
		}

		noteID := pathToID[pf.file.path]

		// Reassign incoming edges from phantom to note.
		if _, err := tx.Exec("UPDATE edges SET target_id = ? WHERE target_id = ?", noteID, phantomID); err != nil {
			return nil, err
		}

		// Delete phantom node.
		if _, err := tx.Exec("DELETE FROM nodes WHERE id = ?", phantomID); err != nil {
			return nil, err
		}

		result.Promoted = append(result.Promoted, pf.file.path)
	}

	// Step 14: resolve links and create edges.
	for _, pf := range parsed {
		sourceID := pathToID[pf.file.path]
		for _, link := range pf.links {
			targetID, subpath, err := resolveLink(tx, pf.file.path, link, pathSet, basenameToPath, pathToID)
			if err != nil {
				return nil, err
			}
			if targetID == 0 {
				continue
			}
			if err := insertEdge(tx, sourceID, targetID, link.linkType, link.rawLink, subpath, link.lineStart, link.lineEnd); err != nil {
				return nil, err
			}
		}
	}

	// Update DB for rewritten edges.
	if len(allRewrites) > 0 {
		for _, re := range allRewrites {
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
		for _, re := range allRewrites {
			if mtimeUpdated[re.sourceID] {
				continue
			}
			mtimeUpdated[re.sourceID] = true
			mt := newMtimes[re.sourceID]
			if _, err := tx.Exec("UPDATE nodes SET mtime = ? WHERE id = ? AND type = 'note'", mt, re.sourceID); err != nil {
				return nil, err
			}
		}
	}

	// Step 15: orphan cleanup.
	if _, err := tx.Exec("DELETE FROM nodes WHERE type IN ('tag','phantom') AND id NOT IN (SELECT DISTINCT target_id FROM edges)"); err != nil {
		return nil, err
	}

	// Step 16: commit.
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return result, nil
}

// buildRewritePath constructs the rewritten path for a link target.
// If targetPath contains "/" (subdirectory), returns vault-relative path.
// If targetPath has no "/" (root), returns source-relative path with ./ or ../ prefix.
func buildRewritePath(sourcePath, targetPath, linkType string) string {
	targetNoExt := strings.TrimSuffix(targetPath, filepath.Ext(targetPath))

	if strings.Contains(targetPath, "/") {
		// Subdirectory target → vault-relative path.
		if linkType == "wikilink" {
			return targetNoExt
		}
		// markdown: determine if original had .md extension (handled by caller).
		return targetNoExt
	}

	// Root target → source-relative path.
	sourceDir := filepath.Dir(sourcePath)
	rel, _ := filepath.Rel(sourceDir, ".")
	relDir := filepath.ToSlash(filepath.Clean(rel))
	if relDir == "." {
		// Same directory: use "./" prefix.
		return "./" + targetNoExt
	}
	return relDir + "/" + targetNoExt
}

// rewriteRawLink replaces the target in a raw link with the rewritten path.
func rewriteRawLink(rawLink, linkType, sourcePath, targetPath string) string {
	switch linkType {
	case "wikilink":
		// rawLink: [[Target]], [[Target|alias]], [[Target#Heading]], [[Target#Heading|alias]]
		inner := strings.TrimPrefix(rawLink, "[[")
		inner = strings.TrimSuffix(inner, "]]")

		var alias, subpath string
		// Extract alias (after |).
		if idx := strings.Index(inner, "|"); idx >= 0 {
			alias = inner[idx:] // includes |
			inner = inner[:idx]
		}
		// Extract subpath (after #).
		if idx := strings.Index(inner, "#"); idx >= 0 {
			subpath = inner[idx:] // includes #
		}

		newPath := buildRewritePath(sourcePath, targetPath, linkType)
		return "[[" + newPath + subpath + alias + "]]"

	case "markdown":
		// rawLink: [text](url), [text](url#frag)
		start := strings.Index(rawLink, "](")
		if start < 0 {
			return rawLink
		}
		textPart := rawLink[:start+2] // "[text]("
		urlPart := rawLink[start+2:]
		urlPart = strings.TrimSuffix(urlPart, ")")

		// Extract fragment.
		var frag string
		if idx := strings.Index(urlPart, "#"); idx >= 0 {
			frag = urlPart[idx:] // includes #
			urlPart = urlPart[:idx]
		}

		// Check if original URL had .md extension.
		hasMdExt := strings.HasSuffix(strings.ToLower(urlPart), ".md")

		newPath := buildRewritePath(sourcePath, targetPath, linkType)
		if hasMdExt {
			newPath += ".md"
		}

		return textPart + newPath + frag + ")"
	}
	return rawLink
}

// replaceOutsideInlineCode replaces occurrences of old with new in line,
// but only outside backtick-delimited inline code spans.
func replaceOutsideInlineCode(line, old, new string) string {
	var result strings.Builder
	i := 0
	for i < len(line) {
		if line[i] == '`' {
			// Find the closing backtick.
			end := strings.IndexByte(line[i+1:], '`')
			if end < 0 {
				// No closing backtick — rest of line is code.
				result.WriteString(line[i:])
				return result.String()
			}
			// Copy the inline code span verbatim.
			span := line[i : i+1+end+1]
			result.WriteString(span)
			i += len(span)
			continue
		}
		// Check for old string match.
		if strings.HasPrefix(line[i:], old) {
			result.WriteString(new)
			i += len(old)
			continue
		}
		result.WriteByte(line[i])
		i++
	}
	return result.String()
}

// restoreBackups restores files to their original content (best-effort).
func restoreBackups(vaultPath string, backups []rewriteBackup) {
	for _, fb := range backups {
		_ = os.WriteFile(filepath.Join(vaultPath, fb.path), fb.content, 0o644)
	}
}

// applyFileRewrites applies rewrite entries to source files on disk.
// Returns a map of sourceID → new mtime after writing, and backups for rollback.
// On error during write, restores already-written files (best-effort).
func applyFileRewrites(vaultPath string, groups map[string][]rewriteEntry) (map[int64]int64, []rewriteBackup, error) {
	newMtimes := make(map[int64]int64)

	// Phase 1: read all originals before any writes.
	originals := make(map[string][]byte, len(groups))
	for sourcePath := range groups {
		fullPath := filepath.Join(vaultPath, sourcePath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, nil, err
		}
		originals[sourcePath] = content
	}

	// Phase 2: compute new content and write files.
	var written []rewriteBackup

	restore := func() {
		for _, fb := range written {
			_ = os.WriteFile(filepath.Join(vaultPath, fb.path), fb.content, 0o644)
		}
	}

	for sourcePath, entries := range groups {
		fullPath := filepath.Join(vaultPath, sourcePath)
		original := originals[sourcePath]
		lines := strings.Split(string(original), "\n")

		// Group entries by line number.
		lineEntries := make(map[int][]rewriteEntry)
		for _, re := range entries {
			lineEntries[re.lineStart] = append(lineEntries[re.lineStart], re)
		}

		// Apply replacements line by line.
		for lineNum, res := range lineEntries {
			if lineNum < 1 || lineNum > len(lines) {
				continue
			}
			idx := lineNum - 1 // convert 1-based to 0-based
			for _, re := range res {
				lines[idx] = replaceOutsideInlineCode(lines[idx], re.rawLink, re.newRawLink)
			}
		}

		newContent := []byte(strings.Join(lines, "\n"))
		if err := os.WriteFile(fullPath, newContent, 0o644); err != nil {
			restore()
			return nil, nil, err
		}
		written = append(written, rewriteBackup{path: sourcePath, content: original})

		// Collect new mtime.
		info, err := os.Stat(fullPath)
		if err != nil {
			restore()
			return nil, nil, err
		}
		sourceID := entries[0].sourceID
		newMtimes[sourceID] = info.ModTime().Unix()
	}

	return newMtimes, written, nil
}

// isBasenameRawLink checks if a raw_link represents a basename link (no path separators).
func isBasenameRawLink(rawLink, linkType string) bool {
	switch linkType {
	case "wikilink":
		// raw_link is like "[[Target]]" or "[[Target|alias]]" or "[[Target#heading]]"
		inner := strings.TrimPrefix(rawLink, "[[")
		inner = strings.TrimSuffix(inner, "]]")
		// Remove alias part.
		if idx := strings.Index(inner, "|"); idx >= 0 {
			inner = inner[:idx]
		}
		// Remove subpath (heading).
		if idx := strings.Index(inner, "#"); idx >= 0 {
			inner = inner[:idx]
		}
		// Empty target means self-link like [[#Heading]], not a basename link.
		if inner == "" {
			return false
		}
		return !strings.Contains(inner, "/")
	case "markdown":
		// raw_link is like "[text](url)" or "[text](url#heading)"
		start := strings.Index(rawLink, "](")
		if start < 0 {
			return false
		}
		url := rawLink[start+2:]
		url = strings.TrimSuffix(url, ")")
		// Remove fragment.
		if idx := strings.Index(url, "#"); idx >= 0 {
			url = url[:idx]
		}
		// Empty url means self-link like [text](#heading), not a basename link.
		if url == "" {
			return false
		}
		return !strings.Contains(url, "/")
	}
	return false
}
