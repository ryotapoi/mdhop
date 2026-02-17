package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Build parses the vault and creates the index DB.
func Build(vaultPath string) error {
	if _, err := ensureDataDir(vaultPath); err != nil {
		return err
	}

	files, err := collectMarkdownFiles(vaultPath)
	if err != nil {
		return err
	}

	basenameCounts := countBasenames(files)

	// Build lookup maps (no DB needed).
	basenameToPath := make(map[string]string) // lower basename → path (only for unique basenames)
	for _, rel := range files {
		bk := basenameKey(rel)
		if basenameCounts[bk] == 1 {
			basenameToPath[bk] = rel
		}
	}
	pathSet := make(map[string]string) // normalized lookup key → actual vault-relative path
	for _, rel := range files {
		pathSet[strings.ToLower(rel)] = rel
		noExt := strings.TrimSuffix(rel, filepath.Ext(rel))
		pathSet[strings.ToLower(noExt)] = rel
	}

	// Read all files, parse links, stat for mtime, and validate.
	// Done before DB creation so failures leave no temp file behind.
	type parsedFile struct {
		path  string
		mtime int64
		links []linkOccur
	}
	parsed := make([]parsedFile, 0, len(files))
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

		// Validate links (fail-fast on ambiguous or vault-escape).
		for _, link := range links {
			if link.linkType != "wikilink" && link.linkType != "markdown" {
				continue
			}
			if link.isRelative && escapesVault(rel, link.target) {
				return fmt.Errorf("link escapes vault: %s in %s", link.rawLink, rel)
			}
			if !link.isRelative && !link.isBasename && pathEscapesVault(link.target) {
				return fmt.Errorf("link escapes vault: %s in %s", link.rawLink, rel)
			}
			if link.isBasename && basenameCounts[strings.ToLower(link.target)] > 1 {
				return fmt.Errorf("ambiguous link: %s in %s", link.target, rel)
			}
		}

		parsed = append(parsed, parsedFile{
			path:  rel,
			mtime: info.ModTime().Unix(),
			links: links,
		})
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
	pathToID := make(map[string]int64)
	for _, pf := range parsed {
		name := basename(pf.path)
		id, err := upsertNote(tx, pf.path, name, pf.mtime)
		if err != nil {
			return err
		}
		pathToID[pf.path] = id
	}

	// Pass 2: resolve links and create edges (using cached parsed data).
	for _, pf := range parsed {
		sourceID := pathToID[pf.path]
		for _, link := range pf.links {
			targetID, subpath, err := resolveLink(tx, pf.path, link, pathSet, basenameToPath, pathToID)
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
func resolveLink(db dbExecer, sourcePath string, link linkOccur, pathSet map[string]string, basenameToPath map[string]string, pathToID map[string]int64) (int64, string, error) {
	// Self-link: [[#Heading]]
	if link.target == "" && link.subpath != "" {
		id := pathToID[sourcePath]
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
		resolved := normalizePath(filepath.Join(filepath.Dir(sourcePath), target))
		if escapesVault(sourcePath, target) {
			return 0, "", fmt.Errorf("link escapes vault: %s in %s", link.rawLink, sourcePath)
		}
		return resolvePathTarget(db, resolved, link, pathSet, pathToID)
	}

	// Vault-absolute path escape check (defense-in-depth).
	if !link.isBasename && pathEscapesVault(target) {
		return 0, "", fmt.Errorf("link escapes vault: %s in %s", link.rawLink, sourcePath)
	}

	// Absolute path (/ prefix, markdown link only): /sub/B.md → sub/B.md
	if strings.HasPrefix(target, "/") {
		stripped := strings.TrimPrefix(target, "/")
		return resolvePathTarget(db, stripped, link, pathSet, pathToID)
	}

	// Wikilink with vault-relative path (contains /, not relative): [[path/to/Note]]
	if link.linkType == "wikilink" && !link.isBasename {
		return resolvePathTarget(db, target, link, pathSet, pathToID)
	}

	// Markdown link with path (not ./ ../ /): treated as basename resolution (same as wikilink)
	// Basename resolution (wikilink and markdown)
	if link.isBasename {
		lower := strings.ToLower(target)
		if path, ok := basenameToPath[lower]; ok {
			id := pathToID[path]
			return id, link.subpath, nil
		}
		// Not found → phantom
		id, err := upsertPhantom(db, target)
		if err != nil {
			return 0, "", err
		}
		return id, link.subpath, nil
	}

	// Markdown link with path that is not relative and not / prefix → basename resolution
	return resolvePathTarget(db, target, link, pathSet, pathToID)
}

// resolvePathTarget tries to find a file by path in pathSet, falling back to phantom.
func resolvePathTarget(db dbExecer, resolved string, link linkOccur, pathSet map[string]string, pathToID map[string]int64) (int64, string, error) {
	lower := strings.ToLower(resolved)
	if actualPath, ok := pathSet[lower]; ok {
		id := pathToID[actualPath]
		return id, link.subpath, nil
	}
	// Try with .md extension
	if actualPath, ok := pathSet[lower+".md"]; ok {
		id := pathToID[actualPath]
		return id, link.subpath, nil
	}
	// Not found → phantom
	name := filepath.Base(resolved)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	id, err := upsertPhantom(db, name)
	if err != nil {
		return 0, "", err
	}
	return id, link.subpath, nil
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
			files = append(files, normalizePath(rel))
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
	n := normalizePath(stripped)
	return n == ".." || strings.HasPrefix(n, "../")
}
