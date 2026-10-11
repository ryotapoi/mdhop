package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxBuildErrors = 5

// BuildResult contains the result of a Build operation.
type BuildResult struct {
	Warnings []string
}

// Build parses the vault and creates the index DB.
func Build(vaultPath string, locations ...Locations) (*BuildResult, error) {
	dbp, err := resolveDBPath(vaultPath, locations...)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbp), 0o755); err != nil {
		return nil, err
	}

	prepared, err := prepareBuild(vaultPath, locations...)
	if err != nil {
		return nil, err
	}
	return buildPrepared(vaultPath, prepared, locations...)
}

func buildPrepared(vaultPath string, prepared *preparedBuild, locations ...Locations) (*BuildResult, error) {
	dbp, err := resolveDBPath(vaultPath, locations...)
	if err != nil {
		return nil, err
	}

	// Reserve a private DB path without releasing its file to another build.
	tmpFile, err := os.CreateTemp(filepath.Dir(dbp), filepath.Base(dbp)+".tmp-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if err := tmpFile.Close(); err != nil {
		return nil, err
	}

	db, err := openDBAt(tmpPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := initSchema(db); err != nil {
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Pass 1: insert all note nodes.
	for _, pf := range prepared.notes {
		name := basename(pf.path)
		id, err := upsertNote(tx, pf.path, name, pf.mtime, pf.lines)
		if err != nil {
			return nil, err
		}
		prepared.resolveMaps.registerNote(pf.path, id)
	}

	// Pass 1.5: insert all asset nodes.
	for _, ai := range prepared.assets {
		name := filepath.Base(ai.path)
		id, err := upsertAsset(tx, ai.path, name, ai.mtime)
		if err != nil {
			return nil, err
		}
		prepared.resolveMaps.registerAsset(ai.path, id)
	}

	// Pass 2: resolve links and create edges (using cached parsed data).
	for _, pf := range prepared.notes {
		sourceID := prepared.resolveMaps.pathToID[pf.path]
		for _, link := range pf.links {
			targetID, subpath, err := resolveLink(tx, pf.path, link, prepared.resolveMaps)
			if err != nil {
				return nil, err
			}
			if targetID == 0 {
				continue
			}
			if err := insertEdge(tx, sourceID, targetID, link.linkType, link.rawLink, link.frontmatterKey, link.referenceTarget, subpath, link.lineStart, link.lineEnd, link.inTable); err != nil {
				return nil, err
			}
		}
	}

	// Pass 3: insert frontmatter metadata.
	var metaWarnings []string
	for _, pf := range prepared.notes {
		nodeID := prepared.resolveMaps.pathToID[pf.path]
		ws, err := insertMetaEntries(tx, nodeID, pf.path, pf.meta, MetaConfig{Types: prepared.metaTypes})
		if err != nil {
			return nil, err
		}
		metaWarnings = append(metaWarnings, ws...)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if err := db.Close(); err != nil {
		return nil, err
	}

	if err := os.Rename(tmpPath, dbp); err != nil {
		return nil, err
	}
	return &BuildResult{Warnings: metaWarnings}, nil
}

// resolveLink resolves a linkOccur to a target node ID and subpath.
// Returns (0, "", nil) if the link should be skipped.
func resolveLink(db dbExecer, sourcePath string, link linkOccur, rm *resolveMaps) (int64, string, error) {
	return resolveLinkWithBackend(sourcePath, link, mapLinkResolver{db: db, rm: rm})
}

type mapLinkResolver struct {
	db dbExecer
	rm *resolveMaps
}

func (r mapLinkResolver) resolveSelf(sourcePath string, link linkOccur) (int64, string, error) {
	return r.rm.pathToID[sourcePath], link.subpath, nil
}

func (r mapLinkResolver) resolveTag(link linkOccur) (int64, string, error) {
	id, err := upsertTag(r.db, link.target)
	if err != nil {
		return 0, "", err
	}
	return id, "", nil
}

func (r mapLinkResolver) resolvePath(resolved string, link linkOccur) (int64, string, error) {
	return resolvePathTarget(r.db, resolved, link, r.rm)
}

func (r mapLinkResolver) resolveBasename(target string, link linkOccur) (int64, string, error) {
	if path, nodeType, ok := r.rm.lookupBasename(target); ok {
		if nodeType == NodeTypeNote {
			return r.rm.pathToID[path], link.subpath, nil
		}
		return r.rm.assetPathToID[path], link.subpath, nil
	}
	// Unresolved basename fallback.
	id, err := upsertPhantom(r.db, target)
	if err != nil {
		return 0, "", err
	}
	return id, link.subpath, nil
}

// resolvePathTarget tries to find a file by path in pathSet, falling back to asset then phantom.
func resolvePathTarget(db dbExecer, resolved string, link linkOccur, rm *resolveMaps) (int64, string, error) {
	normalized := NormalizePath(resolved)
	if actualPath, nodeType, ok := rm.lookupPath(normalized); ok {
		if nodeType == NodeTypeNote {
			return rm.pathToID[actualPath], link.subpath, nil
		}
		return rm.assetPathToID[actualPath], link.subpath, nil
	}
	// D10: only strip .md extension from unresolved path names.
	name := filepath.Base(normalized)
	if strings.HasSuffix(strings.ToLower(name), ".md") {
		name = name[:len(name)-3]
	}
	id, err := upsertPhantom(db, name)
	if err != nil {
		return 0, "", err
	}
	return id, link.subpath, nil
}

func formatBuildErrors(errs []error) error {
	hasAmbiguous := false
	for _, e := range errs {
		if errors.Is(e, ErrAmbiguousLink) {
			hasAmbiguous = true
			break
		}
	}

	if len(errs) == 1 {
		s := errs[0].Error()
		if hasAmbiguous {
			s += "\nhint: run 'mdhop disambiguate --scan --name <basename>' to resolve ambiguous links"
		}
		return errors.New(s)
	}
	var b strings.Builder
	for _, e := range errs {
		b.WriteString(e.Error())
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
	return errors.New(b.String())
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
			// Never skip the walk root: with vaultPath "." its entry name
			// is "." and would match the hidden-directory check.
			if path == vaultPath {
				return nil
			}
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
