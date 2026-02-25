package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxBuildErrors = 5

// resolveMaps holds in-memory lookup maps for link resolution.
type resolveMaps struct {
	// note
	pathSet            map[string]string // lower path → actual path
	basenameToPath     map[string]string // lower basename → path (unique only)
	rootBasenameToPath map[string]string // lower basename → root path
	pathToID           map[string]int64
	basenameCounts     map[string]int
	// asset
	assetPathSet            map[string]string // lower path → actual path
	assetBasenameToPath     map[string]string // lower asset basename → path (unique only)
	assetRootBasenameToPath map[string]string // lower asset basename → root path
	assetPathToID           map[string]int64
	assetBasenameCounts     map[string]int
}

// Build parses the vault and creates the index DB.
func Build(vaultPath string) error {
	if _, err := ensureDataDir(vaultPath); err != nil {
		return err
	}

	// Pass 0: collect .md files.
	files, err := collectMarkdownFiles(vaultPath)
	if err != nil {
		return err
	}

	cfg, err := LoadConfig(vaultPath)
	if err != nil {
		return err
	}
	if err := validateGlobPatterns(cfg.Build.ExcludePaths); err != nil {
		return err
	}
	files = filterBuildExcludes(files, cfg.Build.ExcludePaths)

	// Pass 0.5: collect asset files.
	assetFiles, err := collectAssetFiles(vaultPath)
	if err != nil {
		return err
	}
	assetFiles = filterBuildExcludes(assetFiles, cfg.Build.ExcludePaths)

	// Build resolve maps for notes.
	noteBasenameCounts := countBasenames(files)
	noteBasenameToPath := make(map[string]string)
	noteRootBasenameToPath := make(map[string]string)
	for _, rel := range files {
		bk := basenameKey(rel)
		if noteBasenameCounts[bk] == 1 {
			noteBasenameToPath[bk] = rel
		}
		if isRootFile(rel) {
			noteRootBasenameToPath[bk] = rel
		}
	}
	notePathSet := make(map[string]string)
	for _, rel := range files {
		notePathSet[strings.ToLower(rel)] = rel
		noExt := strings.TrimSuffix(rel, filepath.Ext(rel))
		notePathSet[strings.ToLower(noExt)] = rel
	}

	// Build resolve maps for assets.
	assetBasenameCounts := countAssetBasenames(assetFiles)
	assetBasenameToPath := make(map[string]string)
	assetRootBasenameToPath := make(map[string]string)
	for _, rel := range assetFiles {
		abk := assetBasenameKey(rel)
		if assetBasenameCounts[abk] == 1 {
			assetBasenameToPath[abk] = rel
		}
		if isRootFile(rel) {
			assetRootBasenameToPath[abk] = rel
		}
	}
	assetPathSet := make(map[string]string)
	for _, rel := range assetFiles {
		assetPathSet[strings.ToLower(rel)] = rel
	}

	rm := &resolveMaps{
		pathSet:                 notePathSet,
		basenameToPath:          noteBasenameToPath,
		rootBasenameToPath:      noteRootBasenameToPath,
		pathToID:                make(map[string]int64),
		basenameCounts:          noteBasenameCounts,
		assetPathSet:            assetPathSet,
		assetBasenameToPath:     assetBasenameToPath,
		assetRootBasenameToPath: assetRootBasenameToPath,
		assetPathToID:           make(map[string]int64),
		assetBasenameCounts:     assetBasenameCounts,
	}

	// Read all files, parse links, stat for mtime, and validate.
	// Done before DB creation so failures leave no temp file behind.
	type parsedFile struct {
		path  string
		mtime int64
		links []linkOccur
	}
	parsed := make([]parsedFile, 0, len(files))
	var userErrors []string
	for _, rel := range files {
		fullPath := filepath.Join(vaultPath, rel)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return err
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return err
		}
		links := parseLinks(string(content))

		// Validate links: collect user errors (ambiguous, vault-escape) up to maxBuildErrors.
		for _, link := range links {
			if link.linkType != "wikilink" && link.linkType != "markdown" {
				continue
			}
			if link.isRelative && escapesVault(rel, link.target) {
				userErrors = append(userErrors, fmt.Sprintf("link escapes vault: %s in %s", link.rawLink, rel))
			} else if !link.isRelative && !link.isBasename && pathEscapesVault(link.target) {
				userErrors = append(userErrors, fmt.Sprintf("link escapes vault: %s in %s", link.rawLink, rel))
			} else if link.isBasename && isAmbiguousBasenameLink(link.target, rm) {
				userErrors = append(userErrors, fmt.Sprintf("ambiguous link: %s in %s", link.target, rel))
			} else {
				continue
			}
			if len(userErrors) >= maxBuildErrors {
				break
			}
		}
		if len(userErrors) >= maxBuildErrors {
			break
		}

		parsed = append(parsed, parsedFile{
			path:  rel,
			mtime: info.ModTime().Unix(),
			links: links,
		})
	}
	if len(userErrors) > 0 {
		return formatBuildErrors(userErrors)
	}

	// Stat asset files for mtime.
	type assetInfo struct {
		path  string
		mtime int64
	}
	assetInfos := make([]assetInfo, 0, len(assetFiles))
	for _, rel := range assetFiles {
		fullPath := filepath.Join(vaultPath, rel)
		info, err := os.Stat(fullPath)
		if err != nil {
			return err
		}
		assetInfos = append(assetInfos, assetInfo{path: rel, mtime: info.ModTime().Unix()})
	}

	// Create temp DB.
	tmpPath := dbPath(vaultPath) + ".tmp"
	_ = os.Remove(tmpPath)
	defer os.Remove(tmpPath)

	db, err := openDBAt(tmpPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := initSchema(db); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Pass 1: insert all note nodes.
	for _, pf := range parsed {
		name := basename(pf.path)
		id, err := upsertNote(tx, pf.path, name, pf.mtime)
		if err != nil {
			return err
		}
		rm.pathToID[pf.path] = id
	}

	// Pass 1.5: insert all asset nodes.
	for _, ai := range assetInfos {
		name := filepath.Base(ai.path)
		id, err := upsertAsset(tx, ai.path, name, ai.mtime)
		if err != nil {
			return err
		}
		rm.assetPathToID[ai.path] = id
	}

	// Pass 2: resolve links and create edges (using cached parsed data).
	for _, pf := range parsed {
		sourceID := rm.pathToID[pf.path]
		for _, link := range pf.links {
			targetID, subpath, err := resolveLink(tx, pf.path, link, rm)
			if err != nil {
				return err
			}
			if targetID == 0 {
				continue
			}
			if err := insertEdge(tx, sourceID, targetID, link.linkType, link.rawLink, subpath, link.lineStart, link.lineEnd); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if err := db.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, dbPath(vaultPath)); err != nil {
		return err
	}
	return nil
}

// resolveLink resolves a linkOccur to a target node ID and subpath.
// Returns (0, "", nil) if the link should be skipped.
func resolveLink(db dbExecer, sourcePath string, link linkOccur, rm *resolveMaps) (int64, string, error) {
	// Self-link: [[#Heading]]
	if link.target == "" && link.subpath != "" {
		id := rm.pathToID[sourcePath]
		return id, link.subpath, nil
	}

	// Tag or frontmatter tag
	if link.linkType == "tag" || link.linkType == "frontmatter" {
		id, err := upsertTag(db, link.target)
		if err != nil {
			return 0, "", err
		}
		return id, "", nil
	}

	target := link.target

	// Relative path resolution: ./Target or ../Root
	if link.isRelative {
		resolved := NormalizePath(filepath.Join(filepath.Dir(sourcePath), target))
		if escapesVault(sourcePath, target) {
			return 0, "", fmt.Errorf("link escapes vault: %s in %s", link.rawLink, sourcePath)
		}
		return resolvePathTarget(db, resolved, link, rm)
	}

	// Vault-absolute path escape check (defense-in-depth).
	if !link.isBasename && pathEscapesVault(target) {
		return 0, "", fmt.Errorf("link escapes vault: %s in %s", link.rawLink, sourcePath)
	}

	// Absolute path (/ prefix, markdown link only): /sub/B.md → sub/B.md
	if strings.HasPrefix(target, "/") {
		stripped := strings.TrimPrefix(target, "/")
		return resolvePathTarget(db, stripped, link, rm)
	}

	// Wikilink with vault-relative path (contains /, not relative): [[path/to/Note]]
	if link.linkType == "wikilink" && !link.isBasename {
		return resolvePathTarget(db, target, link, rm)
	}

	// Basename resolution (wikilink and markdown)
	if link.isBasename {
		lower := strings.ToLower(target)
		// 1. note unique
		if path, ok := rm.basenameToPath[lower]; ok {
			id := rm.pathToID[path]
			return id, link.subpath, nil
		}
		// 2. note root-priority
		if path, ok := rm.rootBasenameToPath[lower]; ok {
			id := rm.pathToID[path]
			return id, link.subpath, nil
		}
		// 3. asset unique
		if path, ok := rm.assetBasenameToPath[lower]; ok {
			id := rm.assetPathToID[path]
			return id, link.subpath, nil
		}
		// 4. asset root-priority
		if path, ok := rm.assetRootBasenameToPath[lower]; ok {
			id := rm.assetPathToID[path]
			return id, link.subpath, nil
		}
		// 5. phantom fallback
		id, err := upsertPhantom(db, target)
		if err != nil {
			return 0, "", err
		}
		return id, link.subpath, nil
	}

	// Markdown link with path that is not relative and not / prefix
	return resolvePathTarget(db, target, link, rm)
}

// resolvePathTarget tries to find a file by path in pathSet, falling back to asset then phantom.
func resolvePathTarget(db dbExecer, resolved string, link linkOccur, rm *resolveMaps) (int64, string, error) {
	lower := strings.ToLower(resolved)
	// 1. note exact path
	if actualPath, ok := rm.pathSet[lower]; ok {
		id := rm.pathToID[actualPath]
		return id, link.subpath, nil
	}
	// 2. note with .md extension
	if actualPath, ok := rm.pathSet[lower+".md"]; ok {
		id := rm.pathToID[actualPath]
		return id, link.subpath, nil
	}
	// 3. asset exact path
	if actualPath, ok := rm.assetPathSet[lower]; ok {
		id := rm.assetPathToID[actualPath]
		return id, link.subpath, nil
	}
	// 4. phantom fallback (D10: only strip .md extension)
	name := filepath.Base(resolved)
	if strings.HasSuffix(strings.ToLower(name), ".md") {
		name = name[:len(name)-3]
	}
	id, err := upsertPhantom(db, name)
	if err != nil {
		return 0, "", err
	}
	return id, link.subpath, nil
}

func formatBuildErrors(errs []string) error {
	hasAmbiguous := false
	for _, e := range errs {
		if strings.HasPrefix(e, "ambiguous link:") {
			hasAmbiguous = true
			break
		}
	}

	if len(errs) == 1 {
		s := errs[0]
		if hasAmbiguous {
			s += "\nhint: run 'mdhop disambiguate --scan --name <basename>' to resolve ambiguous links"
		}
		return fmt.Errorf("%s", s)
	}
	var b strings.Builder
	for _, e := range errs {
		b.WriteString(e)
		b.WriteByte('\n')
	}
	if len(errs) >= maxBuildErrors {
		fmt.Fprintf(&b, "too many errors (first %d shown)", maxBuildErrors)
	} else {
		fmt.Fprintf(&b, "%d errors total", len(errs))
	}
	if hasAmbiguous {
		b.WriteString("\nhint: run 'mdhop disambiguate --scan --name <basename>' to resolve ambiguous links")
	}
	return fmt.Errorf("%s", b.String())
}

func collectMarkdownFiles(vaultPath string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(vaultPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == dataDirName {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			rel, err := filepath.Rel(vaultPath, path)
			if err != nil {
				return err
			}
			files = append(files, NormalizePath(rel))
		}
		return nil
	})
	return files, err
}

func countBasenames(files []string) map[string]int {
	seen := make(map[string]int)
	for _, file := range files {
		seen[basenameKey(file)]++
	}
	return seen
}

func countAssetBasenames(files []string) map[string]int {
	seen := make(map[string]int)
	for _, file := range files {
		seen[assetBasenameKey(file)]++
	}
	return seen
}

// collectAssetFiles collects all non-.md files in the vault, skipping hidden
// files/directories and the .mdhop directory.
func collectAssetFiles(vaultPath string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(vaultPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if name == dataDirName || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip hidden files.
		if strings.HasPrefix(name, ".") {
			return nil
		}
		// Skip .md files (those are notes).
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			return nil
		}
		rel, err := filepath.Rel(vaultPath, path)
		if err != nil {
			return err
		}
		files = append(files, NormalizePath(rel))
		return nil
	})
	return files, err
}


func escapesVault(fromPath, target string) bool {
	base := filepath.Dir(fromPath)
	joined := filepath.Clean(filepath.Join(base, target))
	return joined == ".." || strings.HasPrefix(joined, "../")
}

// pathEscapesVault checks whether a vault-absolute path escapes the vault root.
// It strips a leading "/" before normalizing, so both "sub/../../X.md" and
// "/sub/../../X.md" are handled correctly.
func pathEscapesVault(target string) bool {
	stripped := strings.TrimPrefix(target, "/")
	n := NormalizePath(stripped)
	return n == ".." || strings.HasPrefix(n, "../")
}
