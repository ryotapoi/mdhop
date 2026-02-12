package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	ambiguous := detectAmbiguousLinks(vaultPath, files, basenameCounts)
	if len(ambiguous) > 0 {
		return fmt.Errorf("ambiguous links: %s", strings.Join(ambiguous, ", "))
	}

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

	// Pass 1: insert all note nodes.
	pathToID := make(map[string]int64)
	basenameToPath := make(map[string]string) // lower basename → path (only for unique basenames)
	for _, rel := range files {
		name := basename(rel)
		info, err := os.Stat(filepath.Join(vaultPath, rel))
		if err != nil {
			return err
		}
		mtime := info.ModTime().Unix()
		id, err := upsertNote(db, rel, name, mtime)
		if err != nil {
			return err
		}
		pathToID[rel] = id

		bk := basenameKey(rel)
		if basenameCounts[bk] == 1 {
			basenameToPath[bk] = rel
		}
	}

	// Build pathSet for path-based lookups (also with .md stripped).
	pathSet := make(map[string]string) // normalized lookup key → actual vault-relative path
	for _, rel := range files {
		pathSet[strings.ToLower(rel)] = rel
		noExt := strings.TrimSuffix(rel, filepath.Ext(rel))
		pathSet[strings.ToLower(noExt)] = rel
	}

	// Pass 2: parse links, resolve targets, create edges.
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(vaultPath, rel))
		if err != nil {
			return err
		}
		sourceID := pathToID[rel]
		links := parseLinks(string(content))

		for _, link := range links {
			targetID, subpath, err := resolveLink(db, rel, link, pathSet, basenameToPath, pathToID)
			if err != nil {
				return err
			}
			if targetID == 0 {
				continue
			}
			if err := insertEdge(db, sourceID, targetID, link.linkType, link.rawLink, subpath, link.lineStart, link.lineEnd); err != nil {
				return err
			}
		}
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

func detectAmbiguousLinks(vaultPath string, files []string, basenameCounts map[string]int) []string {
	ambiguous := make(map[string]bool)
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(vaultPath, rel))
		if err != nil {
			ambiguous["<read error>"] = true
			continue
		}
		for _, link := range parseLinks(string(content)) {
			// Only check wikilink/markdown for ambiguity (not tags/frontmatter).
			if link.linkType != "wikilink" && link.linkType != "markdown" {
				continue
			}
			if link.isRelative && escapesVault(rel, link.target) {
				ambiguous["<vault escape>"] = true
				continue
			}
			if !link.isBasename {
				continue
			}
			if basenameCounts[strings.ToLower(link.target)] > 1 {
				ambiguous[link.target] = true
			}
		}
	}
	if len(ambiguous) == 0 {
		return nil
	}
	out := make([]string, 0, len(ambiguous))
	for name := range ambiguous {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func escapesVault(fromPath, target string) bool {
	base := filepath.Dir(fromPath)
	joined := filepath.Clean(filepath.Join(base, target))
	if strings.HasPrefix(joined, "..") {
		return true
	}
	return false
}
