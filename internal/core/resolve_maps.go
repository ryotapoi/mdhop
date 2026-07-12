package core

import (
	"path/filepath"
	"strings"
)

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

// addNote adds path to pathSet (2 entries: lowercase, lowercase without ext),
// increments basenameCounts, and updates rootBasenameToPath if root file.
// pathToID is registered separately by registerNote after DB insert.
// path must be NormalizePath'd. Maps must be initialized (non-nil).
func (rm *resolveMaps) addNote(path string) {
	path = NormalizePath(path)
	rel := strings.ToLower(path)
	rm.pathSet[rel] = path
	noExt := strings.TrimSuffix(path, filepath.Ext(path))
	rm.pathSet[strings.ToLower(noExt)] = path
	bk := basenameKey(path)
	rm.basenameCounts[bk]++
	if isRootFile(path) {
		rm.rootBasenameToPath[bk] = path
	}
}

// registerNote records the DB node ID for a note path.
func (rm *resolveMaps) registerNote(path string, id int64) {
	path = NormalizePath(path)
	rm.pathToID[path] = id
}

// removeNote removes path from pathToID, pathSet (2 entries),
// decrements basenameCounts (deletes entry if zero), and removes rootBasenameToPath if root file.
// path must be NormalizePath'd. Maps must be initialized (non-nil).
func (rm *resolveMaps) removeNote(path string) {
	path = NormalizePath(path)
	delete(rm.pathToID, path)
	rel := strings.ToLower(path)
	delete(rm.pathSet, rel)
	noExt := strings.TrimSuffix(path, filepath.Ext(path))
	delete(rm.pathSet, strings.ToLower(noExt))
	bk := basenameKey(path)
	rm.basenameCounts[bk]--
	if rm.basenameCounts[bk] <= 0 {
		delete(rm.basenameCounts, bk)
	}
	if isRootFile(path) {
		delete(rm.rootBasenameToPath, bk)
	}
}

// addAsset adds path to assetPathSet (1 entry: lowercase with ext),
// increments assetBasenameCounts, and updates assetRootBasenameToPath if root file.
// assetPathToID is registered separately by registerAsset after DB insert.
func (rm *resolveMaps) addAsset(path string) {
	path = NormalizePath(path)
	rm.assetPathSet[strings.ToLower(path)] = path
	abk := assetBasenameKey(path)
	rm.assetBasenameCounts[abk]++
	if isRootFile(path) {
		rm.assetRootBasenameToPath[abk] = path
	}
}

// registerAsset records the DB node ID for an asset path.
func (rm *resolveMaps) registerAsset(path string, id int64) {
	path = NormalizePath(path)
	rm.assetPathToID[path] = id
}

// removeAsset removes path from assetPathToID, assetPathSet,
// decrements assetBasenameCounts (deletes entry if zero), and removes assetRootBasenameToPath if root file.
func (rm *resolveMaps) removeAsset(path string) {
	path = NormalizePath(path)
	delete(rm.assetPathToID, path)
	delete(rm.assetPathSet, strings.ToLower(path))
	abk := assetBasenameKey(path)
	rm.assetBasenameCounts[abk]--
	if rm.assetBasenameCounts[abk] <= 0 {
		delete(rm.assetBasenameCounts, abk)
	}
	if isRootFile(path) {
		delete(rm.assetRootBasenameToPath, abk)
	}
}

// rebuildBasenameToPath rebuilds basenameToPath from pathToID + extraPaths (count==1 only).
// extraPaths provides additional paths not yet in pathToID (used by Add).
func (rm *resolveMaps) rebuildBasenameToPath(extraPaths []string) {
	rm.basenameToPath = make(map[string]string)
	for p := range rm.pathToID {
		bk := basenameKey(p)
		if rm.basenameCounts[bk] == 1 {
			rm.basenameToPath[bk] = p
		}
	}
	for _, p := range extraPaths {
		bk := basenameKey(p)
		if rm.basenameCounts[bk] == 1 {
			rm.basenameToPath[bk] = p
		}
	}
}

// rebuildAssetBasenameToPath rebuilds assetBasenameToPath from assetPathToID (count==1 only).
func (rm *resolveMaps) rebuildAssetBasenameToPath() {
	rm.assetBasenameToPath = make(map[string]string)
	for p := range rm.assetPathToID {
		abk := assetBasenameKey(p)
		if rm.assetBasenameCounts[abk] == 1 {
			rm.assetBasenameToPath[abk] = p
		}
	}
}

// noteResolveMaps holds in-memory lookup maps for note link resolution (scan-mode).
type noteResolveMaps struct {
	basenameCounts     map[string]int
	basenameToPath     map[string]string // lower basename → path (count==1 only)
	rootBasenameToPath map[string]string // lower basename → root path
	pathSet            map[string]string // lower path → actual path
}

// newResolveMaps assembles a resolveMaps from note and asset file lists. The
// pathToID / assetPathToID maps are initialized empty; callers that need node
// IDs (build/add) register them as they insert nodes. Read-only resolvers
// (meta-check) leave them empty.
func newResolveMaps(files, assetFiles []string) *resolveMaps {
	nm := buildNoteResolveMaps(files)
	am := buildAssetResolveMaps(assetFiles)
	return &resolveMaps{
		pathSet:                 nm.pathSet,
		basenameToPath:          nm.basenameToPath,
		rootBasenameToPath:      nm.rootBasenameToPath,
		pathToID:                make(map[string]int64),
		basenameCounts:          nm.basenameCounts,
		assetPathSet:            am.pathSet,
		assetBasenameToPath:     am.basenameToPath,
		assetRootBasenameToPath: am.rootBasenameToPath,
		assetPathToID:           make(map[string]int64),
		assetBasenameCounts:     am.basenameCounts,
	}
}

// buildNoteResolveMaps builds note resolve maps from a list of vault-relative .md file paths.
func buildNoteResolveMaps(files []string) noteResolveMaps {
	counts := countBasenames(files)
	btp := make(map[string]string)
	rbtp := make(map[string]string)
	for _, rel := range files {
		rel = NormalizePath(rel)
		bk := basenameKey(rel)
		if counts[bk] == 1 {
			btp[bk] = rel
		}
		if isRootFile(rel) {
			rbtp[bk] = rel
		}
	}
	ps := make(map[string]string)
	for _, rel := range files {
		rel = NormalizePath(rel)
		ps[strings.ToLower(rel)] = rel
		noExt := strings.TrimSuffix(rel, filepath.Ext(rel))
		ps[strings.ToLower(noExt)] = rel
	}
	return noteResolveMaps{
		basenameCounts:     counts,
		basenameToPath:     btp,
		rootBasenameToPath: rbtp,
		pathSet:            ps,
	}
}

// assetResolveMaps holds in-memory lookup maps for asset link resolution (scan-mode).
type assetResolveMaps struct {
	basenameCounts     map[string]int
	basenameToPath     map[string]string // lower asset basename → path (count==1 only)
	rootBasenameToPath map[string]string // lower asset basename → root path
	pathSet            map[string]string // lower path → actual path
}

// buildAssetResolveMaps builds asset resolve maps from a list of vault-relative asset file paths.
func buildAssetResolveMaps(assetFiles []string) assetResolveMaps {
	counts := countAssetBasenames(assetFiles)
	btp := make(map[string]string)
	rbtp := make(map[string]string)
	for _, rel := range assetFiles {
		rel = NormalizePath(rel)
		abk := assetBasenameKey(rel)
		if counts[abk] == 1 {
			btp[abk] = rel
		}
		if isRootFile(rel) {
			rbtp[abk] = rel
		}
	}
	ps := make(map[string]string)
	for _, rel := range assetFiles {
		rel = NormalizePath(rel)
		ps[strings.ToLower(rel)] = rel
	}
	return assetResolveMaps{
		basenameCounts:     counts,
		basenameToPath:     btp,
		rootBasenameToPath: rbtp,
		pathSet:            ps,
	}
}
