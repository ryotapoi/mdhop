package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"
)

type buildSignature struct{ Mtime, Size int64 }

func (signature buildSignature) seconds() int64 { return time.Unix(0, signature.Mtime).Unix() }

func signatureOf(info os.FileInfo) buildSignature {
	return buildSignature{info.ModTime().UnixNano(), info.Size()}
}

type buildInputs struct {
	root, omitted, fingerprint string
	cfg                        *Config
	notes, assets              []string
	files                      map[string]buildSignature
	disk                       *vaultDiskPathResolver
}

func collectBuildInputs(vaultPath, omitted string, locations ...Locations) (*buildInputs, error) {
	root, err := canonicalVault(vaultPath)
	if err != nil {
		return nil, err
	}
	cfg, err := LoadConfig(root, locations...)
	if err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(cfg.Build.ExcludePaths); err != nil {
		return nil, err
	}
	notes, err := collectMarkdownFiles(root)
	if err != nil {
		return nil, err
	}
	assets, err := collectAssetFiles(root)
	if err != nil {
		return nil, err
	}
	matcher := newIndexFileMatcher(root, locations)
	notes, err = matcher.filter(notes)
	if err != nil {
		return nil, err
	}
	assets, err = matcher.filter(assets)
	if err != nil {
		return nil, err
	}
	notes = filterBuildExcludes(notes, cfg.Build.ExcludePaths)
	assets = filterBuildExcludes(assets, cfg.Build.ExcludePaths)
	if omitted != "" {
		kept := assets[:0]
		for _, path := range assets {
			if path != omitted {
				kept = append(kept, path)
			}
		}
		assets = kept
	}
	fingerprint, err := buildFingerprint(root, &cfg)
	if err != nil {
		return nil, err
	}
	inputs := &buildInputs{root: root, omitted: omitted, fingerprint: fingerprint, cfg: &cfg, notes: notes, assets: assets, files: make(map[string]buildSignature)}
	// Publication verification collects independently with a fresh resolver.
	disk := newReadVaultDiskPathResolver(root)
	inputs.disk = disk
	for _, paths := range [][]string{notes, assets} {
		for _, path := range paths {
			full, err := disk.existingPath(path)
			if err != nil {
				return nil, err
			}
			info, err := os.Stat(full)
			if err != nil {
				return nil, err
			}
			inputs.files[path] = signatureOf(info)
		}
	}
	return inputs, nil
}

// Only index-effective settings participate; JSON stabilizes map key order.
func buildFingerprint(root string, cfg *Config) (string, error) {
	excludes := append([]string(nil), cfg.Build.ExcludePaths...)
	links := append([]string(nil), cfg.Meta.LinkKeys...)
	sort.Strings(excludes)
	sort.Strings(links)
	encoded, err := json.Marshal(struct {
		Root               string
		Version            int
		Excludes, LinkKeys []string
		Types              map[string]MetaTypeInfo
	}{root, indexInterpretationVersion, excludes, links, cfg.Meta.Types})
	return string(encoded), err
}

func verifyBuildInputs(inputs *buildInputs, locations ...Locations) error {
	current, err := collectBuildInputs(inputs.root, inputs.omitted, locations...)
	if err != nil {
		return err
	}
	if current.fingerprint != inputs.fingerprint || !reflect.DeepEqual(current.files, inputs.files) {
		return buildInputChanged(inputs.root)
	}
	return nil
}

// BuildFull regenerates without opening or reusing the previous index.
func BuildFull(vaultPath string, locations ...Locations) (*BuildResult, error) {
	return build(vaultPath, true, locations...)
}

func build(vaultPath string, full bool, locations ...Locations) (*BuildResult, error) {
	dbp, err := resolveDBPath(vaultPath, locations...)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbp), 0o755); err != nil {
		return nil, err
	}
	if !full {
		inputs, err := collectBuildInputs(vaultPath, "", locations...)
		if err != nil {
			return nil, err
		}
		state, err := loadBuildState(dbp, inputs.fingerprint)
		if err != nil {
			return nil, err
		}
		if state != nil {
			return buildIncremental(vaultPath, dbp, inputs, state, locations...)
		}
	}
	prepared, err := prepareBuild(vaultPath, locations...)
	if err != nil {
		return nil, err
	}
	return buildPrepared(vaultPath, prepared, locations...)
}

// The stored representation includes skipped/phantom links and reference
// destinations, rather than attempting to reconstruct parsing from edges.
type savedBuildLink struct {
	Target                                        string
	Basename, Relative                            bool
	Type                                          LinkType
	Raw, FrontmatterKey, ReferenceTarget, Subpath string
	Start, End                                    int
	InTable                                       bool
}

func saveLink(link linkOccur) savedBuildLink {
	return savedBuildLink{link.target, link.isBasename, link.isRelative, link.linkType, link.rawLink, link.frontmatterKey, link.referenceTarget, link.subpath, link.lineStart, link.lineEnd, link.inTable}
}
func (link savedBuildLink) occurrence() linkOccur {
	return linkOccur{target: link.Target, isBasename: link.Basename, isRelative: link.Relative, linkType: link.Type, rawLink: link.Raw, frontmatterKey: link.FrontmatterKey, referenceTarget: link.ReferenceTarget, subpath: link.Subpath, lineStart: link.Start, lineEnd: link.End, inTable: link.InTable}
}

// Classification follows the actual dispatch, including self/tag precedence.
type buildDependencyResolver struct{}

func (buildDependencyResolver) resolveSelf(_ string, _ linkOccur) (string, string, error) {
	return "", "", nil
}
func (buildDependencyResolver) resolveTag(_ linkOccur) (string, string, error) { return "", "", nil }
func (buildDependencyResolver) resolvePath(path string, _ linkOccur) (string, string, error) {
	return "path:" + strings.ToLower(NormalizePath(path)), "", nil
}
func (buildDependencyResolver) resolveBasename(target string, _ linkOccur) (string, string, error) {
	return "basename:" + strings.ToLower(normalizeTextNFC(target)), "", nil
}
func buildDependency(source string, link linkOccur) (string, error) {
	dependency, _, err := resolveLinkWithBackend(source, link, buildDependencyResolver{})
	return dependency, err
}

func dependencyResolution(dependency string, rm *resolveMaps) string {
	var path string
	var typ NodeType
	var found bool
	if target, ok := strings.CutPrefix(dependency, "basename:"); ok {
		if isAmbiguousBasenameLink(target, rm) {
			return "ambiguous"
		}
		path, typ, found = rm.lookupBasename(target)
	} else if target, ok := strings.CutPrefix(dependency, "path:"); ok {
		path, typ, found = rm.lookupPath(target)
	}
	if !found {
		return ""
	}
	return string(typ) + ":" + path
}

type savedBuildFile struct {
	signature buildSignature
	kind      NodeType
	warnings  []string
}
type savedBuildState struct{ files map[string]savedBuildFile }

func loadBuildState(path, fingerprint string) (*savedBuildState, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	db, err := openDBAt(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return nil, err
	}
	if version != indexInterpretationVersion {
		return nil, nil
	}
	columns, err := db.Query("PRAGMA table_info(edges)")
	if err != nil {
		return nil, err
	}
	present := make(map[string]bool)
	for columns.Next() {
		var ordinal, notNull, pk int
		var name, typ string
		var defaultValue any
		if err := columns.Scan(&ordinal, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			columns.Close()
			return nil, err
		}
		present[name] = true
	}
	err = errors.Join(columns.Err(), columns.Close())
	if err != nil {
		return nil, err
	}
	if !present["reference_target"] || !present["in_table"] {
		return nil, nil
	}
	var tables int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('build_state','build_files','build_links')").Scan(&tables); err != nil {
		return nil, err
	}
	if tables != 3 {
		return nil, nil
	}
	var saved string
	if err := db.QueryRow("SELECT fingerprint FROM build_state WHERE id=1").Scan(&saved); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if saved != fingerprint {
		return nil, nil
	}
	// Missing per-file or link information cannot be safely reused.
	var missing int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nodes n LEFT JOIN build_files f ON f.path=n.path
  WHERE n.type IN ('note','asset') AND (f.path IS NULL OR f.kind!=n.type)`).Scan(&missing); err != nil {
		return nil, err
	}
	if missing != 0 {
		return nil, nil
	}
	rows, err := db.Query(`SELECT f.path,f.kind,f.mtime_ns,f.size,f.warnings,f.link_count,
  (SELECT COUNT(*) FROM build_links l WHERE l.source_path=f.path),
  (SELECT COUNT(*) FROM nodes n WHERE n.path=f.path AND n.type=f.kind) FROM build_files f ORDER BY f.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	state := &savedBuildState{files: make(map[string]savedBuildFile)}
	for rows.Next() {
		var path, warnings string
		var file savedBuildFile
		var want, count, nodes int
		if err := rows.Scan(&path, &file.kind, &file.signature.Mtime, &file.signature.Size, &warnings, &want, &count, &nodes); err != nil {
			return nil, err
		}
		if nodes != 1 || want != count || (file.kind != NodeTypeNote && file.kind != NodeTypeAsset) {
			return nil, nil
		}
		if err := json.Unmarshal([]byte(warnings), &file.warnings); err != nil {
			return nil, nil
		}
		state.files[path] = file
	}
	return state, rows.Err()
}

func invalidateBuildState(tx dbExecer) error {
	_, err := tx.Exec("DELETE FROM build_state")
	return err
}

func saveBuildFile(tx dbExecer, inputs *buildInputs, path string, kind NodeType, links []linkOccur, warnings []string) error {
	encodedWarnings, err := json.Marshal(warnings)
	if err != nil {
		return err
	}
	sig := inputs.files[path]
	if _, err := tx.Exec("INSERT OR REPLACE INTO build_files(path,kind,mtime_ns,size,link_count,warnings) VALUES(?,?,?,?,?,?)", path, kind, sig.Mtime, sig.Size, len(links), string(encodedWarnings)); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM build_links WHERE source_path=?", path); err != nil {
		return err
	}
	for ordinal, link := range links {
		dependency, err := buildDependency(path, link)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(saveLink(link))
		if err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO build_links(source_path,ordinal,dependency,link) VALUES(?,?,?,?)", path, ordinal, dependency, string(encoded)); err != nil {
			return err
		}
	}
	return nil
}

func saveFullBuildState(tx dbExecer, prepared *preparedBuild, warnings map[string][]string) error {
	for _, note := range prepared.notes {
		if err := saveBuildFile(tx, prepared.inputs, note.path, NodeTypeNote, note.links, warnings[note.path]); err != nil {
			return err
		}
	}
	for _, asset := range prepared.assets {
		if err := saveBuildFile(tx, prepared.inputs, asset.path, NodeTypeAsset, nil, nil); err != nil {
			return err
		}
	}
	_, err := tx.Exec("INSERT INTO build_state(id,fingerprint) VALUES(1,?)", prepared.inputs.fingerprint)
	return err
}

func statePaths(state *savedBuildState) (notes, assets []string) {
	for path, file := range state.files {
		if file.kind == NodeTypeNote {
			notes = append(notes, path)
		} else {
			assets = append(assets, path)
		}
	}
	sort.Strings(notes)
	sort.Strings(assets)
	return
}

func copyBuildDB(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return errors.Join(err, out.Close())
}

func buildIncremental(vaultPath, dbp string, inputs *buildInputs, state *savedBuildState, locations ...Locations) (*BuildResult, error) {
	oldNotes, oldAssets := statePaths(state)
	oldMaps := newResolveMaps(oldNotes, oldAssets)
	rm := newResolveMaps(inputs.notes, inputs.assets)
	changed := make(map[string]bool)
	deleted := make(map[string]bool)
	for path, signature := range inputs.files {
		previous, ok := state.files[path]
		if !ok || previous.signature != signature {
			changed[path] = true
		}
	}
	for path := range state.files {
		if _, ok := inputs.files[path]; !ok {
			deleted[path] = true
		}
	}
	if len(changed) == 0 && len(deleted) == 0 {
		if err := verifyBuildInputs(inputs, locations...); err != nil {
			return nil, err
		}
		if selectedLocations(locations).DBPath == "" {
			if err := writeCacheOrigin(vaultPath, filepath.Dir(dbp)); err != nil {
				return nil, err
			}
		}
		return &BuildResult{Warnings: buildWarnings(inputs.notes, state.files)}, nil
	}
	// Only changed notes are read and parsed.
	prepared := make(map[string]preparedNote)
	for _, path := range inputs.notes {
		if !changed[path] {
			continue
		}
		note, err := readBuildNote(inputs, path)
		if err != nil {
			return nil, err
		}
		prepared[path] = note
	}
	file, err := os.CreateTemp(filepath.Dir(dbp), filepath.Base(dbp)+".tmp-*")
	if err != nil {
		return nil, err
	}
	tmpPath := file.Name()
	defer os.Remove(tmpPath)
	if err := file.Close(); err != nil {
		return nil, err
	}
	// Writers must be serialized with build. The completed source uses DELETE
	// journaling, so copying it preserves its generation and existing readers.
	if err := copyBuildDB(dbp, tmpPath); err != nil {
		return nil, err
	}
	db, err := openDBAt(tmpPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	links := make(map[string][]linkOccur)
	for path, note := range prepared {
		links[path] = note.links
	}
	// Dependency work is unnecessary for body-only edits. Distinct dependencies
	// are evaluated once; basename resolution is never called without a stored
	// basename dependency.
	if !slices.Equal(oldNotes, inputs.notes) || !slices.Equal(oldAssets, inputs.assets) {
		affected, err := affectedBuildSources(db, oldMaps, rm)
		if err != nil {
			return nil, err
		}
		for path := range affected {
			if changed[path] || deleted[path] {
				continue
			}
			stored, err := loadBuildLinks(db, path)
			if err != nil {
				return nil, err
			}
			links[path] = stored
		}
	}
	var userErrors []error
	for _, path := range inputs.notes {
		for _, link := range links[path] {
			if err := validateParsedLink(path, link, rm); err != nil {
				userErrors = append(userErrors, err)
			}
			if len(userErrors) >= maxBuildErrors {
				break
			}
		}
		if len(userErrors) >= maxBuildErrors {
			break
		}
	}
	if len(userErrors) != 0 {
		return nil, formatBuildErrors(userErrors)
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	changedSources := make(map[string]bool)
	for path := range deleted {
		changedSources[path] = true
	}
	for path := range links {
		changedSources[path] = true
	}
	virtualKeys, err := buildSourceVirtualKeys(tx, changedSources)
	if err != nil {
		return nil, err
	}
	for path := range deleted {
		if _, err := tx.Exec("DELETE FROM edges WHERE source_id IN (SELECT id FROM nodes WHERE path=?)", path); err != nil {
			return nil, err
		}
		if _, err := tx.Exec("DELETE FROM meta WHERE node_id IN (SELECT id FROM nodes WHERE path=?)", path); err != nil {
			return nil, err
		}
		if _, err := tx.Exec("DELETE FROM nodes WHERE path=?", path); err != nil {
			return nil, err
		}
		if _, err := tx.Exec("DELETE FROM build_links WHERE source_path=?", path); err != nil {
			return nil, err
		}
		if _, err := tx.Exec("DELETE FROM build_files WHERE path=?", path); err != nil {
			return nil, err
		}
		delete(state.files, path)
	}
	// Register the final node set before resolving any links.
	for _, path := range inputs.notes {
		if note, ok := prepared[path]; ok {
			if _, err := upsertNote(tx, path, basename(path), note.mtime, note.lines); err != nil {
				return nil, err
			}
		}
		id, err := getNodeID(tx, noteKey(path))
		if err != nil {
			return nil, err
		}
		rm.registerNote(path, id)
	}
	for _, path := range inputs.assets {
		if changed[path] {
			if _, err := upsertAsset(tx, path, filepath.Base(path), inputs.files[path].seconds()); err != nil {
				return nil, err
			}
			if err := saveBuildFile(tx, inputs, path, NodeTypeAsset, nil, nil); err != nil {
				return nil, err
			}
			state.files[path] = savedBuildFile{signature: inputs.files[path], kind: NodeTypeAsset}
		}
		id, err := getNodeID(tx, assetKey(path))
		if err != nil {
			return nil, err
		}
		rm.registerAsset(path, id)
	}
	for _, path := range inputs.notes {
		sourceID := rm.pathToID[path]
		sourceLinks, reevaluate := links[path]
		if reevaluate {
			if _, err := tx.Exec("DELETE FROM edges WHERE source_id=?", sourceID); err != nil {
				return nil, err
			}
			if err := indexParsedLinks(tx, sourceID, path, sourceLinks, rm); err != nil {
				return nil, err
			}
		}
		if note, ok := prepared[path]; ok {
			if err := deleteMetaByNode(tx, sourceID); err != nil {
				return nil, err
			}
			warnings, err := insertMetaEntries(tx, sourceID, path, note.meta, inputs.cfg.Meta)
			if err != nil {
				return nil, err
			}
			if err := saveBuildFile(tx, inputs, path, NodeTypeNote, note.links, warnings); err != nil {
				return nil, err
			}
			state.files[path] = savedBuildFile{signature: inputs.files[path], kind: NodeTypeNote, warnings: warnings}
		}
	}
	newVirtualKeys, err := buildSourceVirtualKeys(tx, changedSources)
	if err != nil {
		return nil, err
	}
	for key := range newVirtualKeys {
		virtualKeys[key] = true
	}
	if err := refreshBuildVirtualNames(tx, inputs.notes, rm, links, virtualKeys); err != nil {
		return nil, err
	}
	// Assets exist independently of references; only virtual nodes are pruned.
	if _, err := tx.Exec("DELETE FROM nodes WHERE type IN ('tag','phantom') AND id NOT IN (SELECT target_id FROM edges)"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if err := db.Close(); err != nil {
		return nil, err
	}
	if err := verifyBuildInputs(inputs, locations...); err != nil {
		return nil, err
	}
	if selectedLocations(locations).DBPath == "" {
		if err := writeCacheOrigin(vaultPath, filepath.Dir(dbp)); err != nil {
			return nil, err
		}
	}
	if err := os.Rename(tmpPath, dbp); err != nil {
		return nil, err
	}
	return &BuildResult{Warnings: buildWarnings(inputs.notes, state.files)}, nil
}

func buildWarnings(notes []string, files map[string]savedBuildFile) []string {
	var warnings []string
	for _, path := range notes {
		warnings = append(warnings, files[path].warnings...)
	}
	return warnings
}

func affectedBuildSources(db dbExecer, oldMaps, newMaps *resolveMaps) (map[string]bool, error) {
	rows, err := db.Query("SELECT DISTINCT dependency FROM build_links WHERE dependency!=''")
	if err != nil {
		return nil, err
	}
	var affectedDependencies []string
	for rows.Next() {
		var dependency string
		if err := rows.Scan(&dependency); err != nil {
			rows.Close()
			return nil, err
		}
		if dependencyResolution(dependency, oldMaps) != dependencyResolution(dependency, newMaps) {
			affectedDependencies = append(affectedDependencies, dependency)
		}
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return nil, err
	}
	affected := make(map[string]bool)
	for _, dependency := range affectedDependencies {
		rows, err := db.Query("SELECT DISTINCT source_path FROM build_links WHERE dependency=?", dependency)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				rows.Close()
				return nil, err
			}
			affected[path] = true
		}
		err = errors.Join(rows.Err(), rows.Close())
		if err != nil {
			return nil, err
		}
	}
	return affected, nil
}

func loadBuildLinks(db dbExecer, path string) ([]linkOccur, error) {
	rows, err := db.Query("SELECT link FROM build_links WHERE source_path=? ORDER BY ordinal", path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []linkOccur
	for rows.Next() {
		var encoded string
		var saved savedBuildLink
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(encoded), &saved); err != nil {
			return nil, err
		}
		links = append(links, saved.occurrence())
	}
	return links, rows.Err()
}

// Changing an earlier reference can change the spelling of a still-referenced
// virtual node. Track only virtual targets of the old/new changed edge sets.
func buildSourceVirtualKeys(db dbExecer, sources map[string]bool) (map[string]bool, error) {
	keys := make(map[string]bool)
	for path := range sources {
		rows, err := db.Query(`SELECT DISTINCT t.node_key FROM edges e
   JOIN nodes s ON s.id=e.source_id JOIN nodes t ON t.id=e.target_id
   WHERE s.path=? AND t.type IN ('tag','phantom')`, path)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				rows.Close()
				return nil, err
			}
			keys[key] = true
		}
		err = errors.Join(rows.Err(), rows.Close())
		if err != nil {
			return nil, err
		}
	}
	return keys, nil
}

type buildVirtualName struct{ key, name string }
type buildVirtualNameResolver struct{ rm *resolveMaps }

func (buildVirtualNameResolver) resolveSelf(_ string, _ linkOccur) (buildVirtualName, string, error) {
	return buildVirtualName{}, "", nil
}
func (buildVirtualNameResolver) resolveTag(link linkOccur) (buildVirtualName, string, error) {
	return buildVirtualName{tagKey(link.target), link.target}, "", nil
}
func (r buildVirtualNameResolver) resolvePath(path string, _ linkOccur) (buildVirtualName, string, error) {
	if _, _, ok := r.rm.lookupPath(path); ok {
		return buildVirtualName{}, "", nil
	}
	name := unresolvedPathName(path)
	return buildVirtualName{phantomKey(name), name}, "", nil
}
func (r buildVirtualNameResolver) resolveBasename(target string, _ linkOccur) (buildVirtualName, string, error) {
	if _, _, ok := r.rm.lookupBasename(target); ok {
		return buildVirtualName{}, "", nil
	}
	return buildVirtualName{phantomKey(target), target}, "", nil
}

func refreshBuildVirtualNames(db dbExecer, notes []string, rm *resolveMaps, links map[string][]linkOccur, keys map[string]bool) error {
	order := make(map[string]int, len(notes))
	for ordinal, path := range notes {
		order[path] = ordinal
	}
	for key := range keys {
		// Full build visits notes in inventory order and links in parser order.
		// Select the first surviving source without reparsing or rewriting edges.
		rows, err := db.Query(`SELECT DISTINCT s.path FROM edges e
   JOIN nodes t ON t.id=e.target_id JOIN nodes s ON s.id=e.source_id
   WHERE t.node_key=?`, key)
		if err != nil {
			return err
		}
		first := ""
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				rows.Close()
				return err
			}
			if first == "" || order[path] < order[first] {
				first = path
			}
		}
		err = errors.Join(rows.Err(), rows.Close())
		if err != nil {
			return err
		}
		if first == "" {
			continue
		}
		occurrences, ok := links[first]
		if !ok {
			occurrences, err = loadBuildLinks(db, first)
			if err != nil {
				return err
			}
			links[first] = occurrences
		}
		for _, link := range occurrences {
			named, _, err := resolveLinkWithBackend(first, link, buildVirtualNameResolver{rm})
			if err != nil {
				return err
			}
			if named.key != key {
				continue
			}
			if _, err := db.Exec("UPDATE nodes SET name=? WHERE node_key=? AND name!=?", named.name, key, named.name); err != nil {
				return err
			}
			break
		}
	}
	return nil
}
