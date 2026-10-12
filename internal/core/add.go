package core

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
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
	Warnings  []string        // meta normalization warnings
}

// Add inserts new Markdown notes into the existing index DB.
func Add(vaultPath string, opts AddOptions, locations ...Locations) (result *AddResult, resultErr error) {
	db, err := openDBChecked(vaultPath, locations...)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	diskPaths := newVaultDiskPathResolver(vaultPath)

	cfg, err := LoadConfig(vaultPath, locations...)
	if err != nil {
		return nil, err
	}

	// Normalize and deduplicate input paths.
	type addFile struct {
		path  string
		mtime int64
	}
	indexFiles := newIndexFileMatcher(vaultPath, locations)
	seen := make(map[string]bool)
	var files []addFile
	for _, f := range opts.Files {
		np := NormalizePath(f)
		if np == ".." || strings.HasPrefix(np, "../") {
			return nil, fmt.Errorf("path escapes vault: %s", f)
		}
		if err := indexFiles.reject(np, diskPaths); err != nil {
			return nil, err
		}
		if !strings.HasSuffix(strings.ToLower(np), ".md") {
			return nil, fmt.Errorf("add requires a Markdown note (.md): %s", np)
		}
		if seen[np] {
			continue
		}
		seen[np] = true
		files = append(files, addFile{path: np})
	}

	// Build maps from DB + save oldBasenameCounts copy. The maps normalize
	// existing DB paths, so they also catch pre-v0.12 NFD index rows.
	rm, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}

	// Check that no file is already registered.
	for _, f := range files {
		if _, ok := rm.pathToID[f.path]; ok {
			return nil, fmt.Errorf("%w: %s", ErrFileAlreadyRegistered, f.path)
		}
	}

	// Check disk existence and collect mtime.
	registeredEntries := make(map[string]bool, len(rm.pathToID))
	for path := range rm.pathToID {
		fullPath, err := diskPaths.existingPath(path)
		if os.IsNotExist(err) {
			continue // A stale registration has no disk entry to compare.
		}
		if err != nil {
			return nil, err
		}
		entryPath, err := addDiskEntryPath(vaultPath, fullPath)
		if err != nil {
			return nil, err
		}
		registeredEntries[entryPath] = true
	}
	for i := range files {
		fullPath, err := diskPaths.existingPath(files[i].path)
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, files[i].path)
		}
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, files[i].path)
		}
		if err != nil {
			return nil, err
		}
		entryPath, err := addDiskEntryPath(vaultPath, fullPath)
		if err != nil {
			return nil, err
		}
		if registeredEntries[entryPath] {
			return nil, fmt.Errorf("%w: %s", ErrFileAlreadyRegistered, files[i].path)
		}
		registeredEntries[entryPath] = true
		files[i].mtime = info.ModTime().Unix()
	}

	oldBasenameCounts := make(map[string]int, len(rm.basenameCounts))
	for k, v := range rm.basenameCounts {
		oldBasenameCounts[k] = v
	}
	// addNote changes basenameCounts but not basenameToPath; rebuilding below replaces the map.
	oldBasenameToPath := rm.basenameToPath

	// Adjust maps for post-add state.
	for _, f := range files {
		rm.addNote(f.path)
	}

	var allRewrites []rewriteEntry

	for bk, newCount := range rm.basenameCounts {
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
			oldTarget := oldBasenameToPath[bk]
			if isRootFile(oldTarget) {
				continue // Root-priority: [[A]] still resolves to root A.md → no ambiguity.
			}
			targetID = rm.pathToID[oldTarget]
			isPatternA = true
		} else {
			// Pattern B: phantom (oldCount == 0, adding 2+ files with same basename).
			// Check if any of the new files is at root → root priority resolves.
			hasNewRoot := false
			for _, f := range files {
				if basenameKey(f.path) == bk && isRootFile(f.path) {
					hasNewRoot = true
					break
				}
			}
			if hasNewRoot {
				continue // Root priority: [[A]] resolves to root file → not ambiguous.
			}
			pk := phantomKey(bk)
			err := db.QueryRow("SELECT id FROM nodes WHERE node_key = ?", pk).Scan(&targetID)
			if errors.Is(err, sql.ErrNoRows) {
				continue // no phantom, so no existing basename links
			}
			if err != nil {
				return nil, err
			}
		}

		// Query edges with source info for potential rewriting.
		rewriteLinkTypeSQL, rewriteLinkTypeArgs := linkTypeSQLIn("e.link_type", rewriteLinkTypes)
		rows, err := db.Query(fmt.Sprintf(
			`SELECT e.id, e.raw_link, e.link_type, e.line_start, sn.path, sn.id, e.in_table
			 FROM edges e JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
			 WHERE e.target_id = ?
			 AND %s`, rewriteLinkTypeSQL), append([]any{targetID}, rewriteLinkTypeArgs...)...)
		if err != nil {
			return nil, err
		}
		var basenameEdges []rewriteEntry
		for rows.Next() {
			var re rewriteEntry
			if err := rows.Scan(&re.edgeID, &re.rawLink, &re.linkType, &re.lineStart, &re.sourcePath, &re.sourceID, &re.inTable); err != nil {
				rows.Close()
				return nil, err
			}
			if isBasenameRawLink(re.rawLink, re.linkType, re.inTable) {
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
			oldTarget := oldBasenameToPath[bk]
			for i := range basenameEdges {
				basenameEdges[i].newRawLink = rewriteRawLink(basenameEdges[i].rawLink, basenameEdges[i].linkType, oldTarget, basenameEdges[i].inTable)
			}
			allRewrites = append(allRewrites, basenameEdges...)
		} else {
			// Pattern B or auto-disambiguate not enabled → error.
			if isPatternA {
				return nil, fmt.Errorf("%w: %s", ErrAddingMakesAmbiguous, basename(oldBasenameToPath[bk]))
			}
			return nil, fmt.Errorf("%w: %s", ErrAddingMakesAmbiguous, bk)
		}
	}

	// Stale check for source files that need rewriting.
	if err := checkRewriteSourcesFresh(db, diskPaths, allRewrites); err != nil {
		return nil, err
	}

	// Rebuild basenameToPath (includes both existing and new files).
	extraPaths := make([]string, len(files))
	for i, f := range files {
		extraPaths[i] = f.path
	}
	rm.rebuildBasenameToPath(extraPaths)

	// Existing frontmatter_path values and reference definitions must keep resolving to the same
	// target after the add (they cannot be rewritten).
	if err := validateFrontmatterPathEdges(db, rm, nil); err != nil {
		return nil, err
	}

	// Parse all new files and check for ambiguous links.
	type parsedFile struct {
		file  addFile
		lines int
		links []linkOccur
		meta  []FrontmatterEntry
	}
	var parsed []parsedFile
	for _, f := range files {
		fullPath, err := diskPaths.existingPath(f.path)
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		pr := parseLinksWithLinkKeys(string(content), cfg.Meta.LinkKeys)
		links := pr.Links

		if err := validateParsedLinks(f.path, links, rm); err != nil {
			return nil, err
		}

		parsed = append(parsed, parsedFile{file: f, lines: countLines(string(content)), links: links, meta: pr.Meta})
	}

	// Apply disk rewrites before transaction (so DB rollback is safe).
	// newMtimes maps sourceID → new mtime after file write.
	var newMtimes map[int64]int64
	var backups []rewriteBackup
	if len(allRewrites) > 0 {
		var rollbackFailures []rollbackFailure
		var applyErr error
		newMtimes, backups, rollbackFailures, applyErr = applyFileRewritesWithRollbackFailures(vaultPath, allRewrites)
		if applyErr != nil {
			return nil, wrapRollbackFailures(applyErr, rollbackFailures)
		}
	}

	// Begin transaction.
	tx, err := db.Begin()
	if err != nil {
		// Restore disk changes if transaction start fails.
		return nil, wrapRollbackFailures(err, restoreBackupFiles(vaultPath, backups))
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
			resultErr = wrapRollbackFailures(resultErr, restoreBackupFiles(vaultPath, backups))
		}
	}()

	result = &AddResult{}

	// Insert all note nodes.
	for _, pf := range parsed {
		name := basename(pf.file.path)
		id, err := upsertNote(tx, pf.file.path, name, pf.file.mtime, pf.lines)
		if err != nil {
			return nil, err
		}
		rm.registerNote(pf.file.path, id)
		result.Added = append(result.Added, pf.file.path)
	}

	// Resolve phantom edges after all notes are registered, so basename root
	// priority and explicit paths can select different same-name candidates.
	for _, pf := range parsed {
		promoted, err := promotePhantom(tx, basename(pf.file.path), rm.pathToID[pf.file.path], pf.file.path, rm)
		if err != nil {
			return nil, err
		}
		if !promoted {
			continue
		}
		result.Promoted = append(result.Promoted, pf.file.path)
	}

	// Resolve links and create edges.
	for _, pf := range parsed {
		sourceID := rm.pathToID[pf.file.path]
		if err := indexParsedLinks(tx, sourceID, pf.file.path, pf.links, rm); err != nil {
			return nil, err
		}

		// Insert meta entries.
		ws, err := insertMetaEntries(tx, rm.pathToID[pf.file.path], pf.file.path, pf.meta, cfg.Meta)
		if err != nil {
			return nil, err
		}
		result.Warnings = append(result.Warnings, ws...)
	}

	// Update DB for rewritten edges.
	if len(allRewrites) > 0 {
		rewritten, err := updateExternalEdgesAndMtimes(tx, vaultPath, cfg.Meta, allRewrites, newMtimes)
		if err != nil {
			return nil, err
		}
		result.Rewritten = append(result.Rewritten, rewritten...)
	}

	// Orphan cleanup.
	if err := cleanupOrphanedNodes(tx); err != nil {
		return nil, err
	}

	// Commit.
	if err := invalidateBuildState(tx); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return result, nil
}

// addDiskEntryPath keeps symlink/hardlink aliases distinct while identifying
// alternate casing of the same directory entry, including parent directories.
func addDiskEntryPath(vaultPath, fullPath string) (string, error) {
	rel, err := filepath.Rel(vaultPath, fullPath)
	if err != nil {
		return "", err
	}
	parent := vaultPath
	var names []string
	for _, component := range strings.Split(filepath.ToSlash(rel), "/") {
		entries, err := os.ReadDir(parent)
		if err != nil {
			return "", err
		}
		name := component
		exact := false
		for _, entry := range entries {
			if normalizeTextNFC(entry.Name()) == normalizeTextNFC(component) {
				name, exact = entry.Name(), true
				break
			}
		}
		if !exact {
			info, err := os.Lstat(filepath.Join(parent, component))
			if err != nil {
				return "", err
			}
			var matches []string
			for _, entry := range entries {
				entryInfo, err := entry.Info()
				if err != nil {
					return "", err
				}
				if os.SameFile(info, entryInfo) {
					matches = append(matches, entry.Name())
				}
			}
			if len(matches) == 1 {
				name = matches[0]
			} else if len(matches) > 1 {
				// Hardlinks can share an inode. Full Unicode folding only
				// distinguishes these filesystem-confirmed candidates; it
				// never makes different filesystem entries identical.
				fold := cases.Fold()
				for _, candidate := range matches {
					if fold.String(normalizeTextNFC(candidate)) == fold.String(normalizeTextNFC(component)) {
						name = candidate
						break
					}
				}
			}
		}
		parent = filepath.Join(parent, name)
		names = append(names, name)
	}
	return NormalizePath(filepath.Join(names...)), nil
}
