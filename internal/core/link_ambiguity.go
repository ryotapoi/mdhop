package core

import (
	"fmt"
	"sort"
	"strings"
)

// isAmbiguousBasenameLink checks if a basename link is ambiguous.
// Returns true if the basename has multiple files AND there is no root-level file.
// When a root-level file exists, the basename link resolves to it (root-priority rule).
// Checks note basenames first, then asset basenames (separate key spaces).
func isAmbiguousBasenameLink(target string, rm *resolveMaps) bool {
	lower := strings.ToLower(normalizeTextNFC(target))
	// Check note namespace.
	if rm.basenameCounts[lower] > 1 {
		return !hasRootInPathSet(lower, rm.pathSet)
	}
	if rm.basenameCounts[lower] == 1 {
		return false // unique note match
	}
	// Check asset namespace.
	if rm.assetBasenameCounts[lower] > 1 {
		return !hasRootInPathSet(lower, rm.assetPathSet)
	}
	return false
}

// ambiguousCandidates returns sorted candidate paths for an ambiguous basename link.
// Must only be called when isAmbiguousBasenameLink returns true.
// Same collection pattern as simplify.go:collectNoteBasenameFiles but for resolveMaps.
func ambiguousCandidates(target string, rm *resolveMaps) []string {
	lower := strings.ToLower(normalizeTextNFC(target))
	var paths []string
	if rm.basenameCounts[lower] > 1 {
		// Note namespace.
		for lp, actual := range rm.pathSet {
			if basenameKey(actual) == lower && strings.HasSuffix(lp, ".md") {
				paths = append(paths, actual)
			}
		}
	} else {
		// Asset namespace.
		for _, actual := range rm.assetPathSet {
			if assetBasenameKey(actual) == lower {
				paths = append(paths, actual)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

// validateParsedLinks checks path links for vault escape and ambiguous
// basenames against rm, returning the first error (same logic as build's
// inline validation). Used by add/update/move/move_dir before edge creation;
// the map-based resolveLink falls back to phantom on unresolved basenames,
// so ambiguity must be rejected here.
func validateParsedLinks(sourcePath string, links []linkOccur, rm *resolveMaps) error {
	for _, link := range links {
		if !isPathLinkType(link.linkType) {
			continue
		}
		if link.isRelative && escapesVault(sourcePath, link.target) {
			return fmt.Errorf("%w: %s in %s", ErrLinkEscapesVault, link.rawLink, sourcePath)
		}
		if !link.isRelative && !link.isBasename && pathEscapesVault(link.target) {
			return fmt.Errorf("%w: %s in %s", ErrLinkEscapesVault, link.rawLink, sourcePath)
		}
		if link.isBasename && isAmbiguousBasenameLink(link.target, rm) {
			candidates := ambiguousCandidates(link.target, rm)
			return fmt.Errorf("%w: %s in %s (candidates: %s)", ErrAmbiguousLink, link.target, sourcePath, strings.Join(candidates, ", "))
		}
	}
	return nil
}
