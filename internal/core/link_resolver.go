package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

type basenameResolver interface {
	resolveBasename(target string, link linkOccur) (int64, string, error)
}

type linkResolverBackend interface {
	basenameResolver
	resolveSelf(sourcePath string, link linkOccur) (int64, string, error)
	resolveTag(link linkOccur) (int64, string, error)
	resolvePath(resolved string, link linkOccur) (int64, string, error)
}

// dryLinkResolver resolves only existing note and asset paths. It assigns
// ephemeral IDs so it can use the same dispatcher as index-backed resolvers
// without creating phantom or tag nodes.
type dryLinkResolver struct {
	rm       *resolveMaps
	pathToID map[string]int64
	idToPath map[int64]string
	nextID   int64
}

func newDryLinkResolver(rm *resolveMaps) *dryLinkResolver {
	r := &dryLinkResolver{
		rm:       rm,
		pathToID: make(map[string]int64),
		idToPath: make(map[int64]string),
		nextID:   1,
	}
	return r
}

func (r *dryLinkResolver) pathID(path string) int64 {
	if id, ok := r.pathToID[path]; ok {
		return id
	}
	r.pathToID[path] = r.nextID
	r.idToPath[r.nextID] = path
	r.nextID++
	return r.nextID - 1
}

func (r *dryLinkResolver) pathForID(id int64) string {
	return r.idToPath[id]
}

func (r *dryLinkResolver) resolveSelf(sourcePath string, link linkOccur) (int64, string, error) {
	return r.pathID(sourcePath), link.subpath, nil
}

func (r *dryLinkResolver) resolveTag(link linkOccur) (int64, string, error) {
	return 0, "", nil
}

func (r *dryLinkResolver) resolvePath(resolved string, link linkOccur) (int64, string, error) {
	lower := strings.ToLower(NormalizePath(resolved))
	if path, ok := r.rm.pathSet[lower]; ok {
		return r.pathID(path), link.subpath, nil
	}
	if path, ok := r.rm.pathSet[lower+".md"]; ok {
		return r.pathID(path), link.subpath, nil
	}
	if path, ok := r.rm.assetPathSet[lower]; ok {
		return r.pathID(path), link.subpath, nil
	}
	return 0, link.subpath, nil
}

func (r *dryLinkResolver) resolveBasename(target string, link linkOccur) (int64, string, error) {
	lower := strings.ToLower(normalizeTextNFC(target))
	if path, ok := r.rm.basenameToPath[lower]; ok {
		return r.pathID(path), link.subpath, nil
	}
	if path, ok := r.rm.rootBasenameToPath[lower]; ok {
		return r.pathID(path), link.subpath, nil
	}
	if path, ok := r.rm.assetBasenameToPath[lower]; ok {
		return r.pathID(path), link.subpath, nil
	}
	if path, ok := r.rm.assetRootBasenameToPath[lower]; ok {
		return r.pathID(path), link.subpath, nil
	}
	return 0, link.subpath, nil
}

// resolveLinkWithBackend owns the link-kind dispatch order. Backends provide
// storage-specific lookups while preserving the shared resolution semantics.
func resolveLinkWithBackend(sourcePath string, link linkOccur, backend linkResolverBackend) (int64, string, error) {
	// Self-link: [[#Heading]]
	if link.target == "" && link.subpath != "" {
		return backend.resolveSelf(sourcePath, link)
	}

	// Tag or frontmatter tag
	if isTagLinkType(link.linkType) {
		return backend.resolveTag(link)
	}

	target := link.target

	// Relative path resolution: ./Target or ../Root
	if link.isRelative {
		if escapesVault(sourcePath, target) {
			return 0, "", fmt.Errorf("%w: %s in %s", ErrLinkEscapesVault, link.rawLink, sourcePath)
		}
		resolved := NormalizePath(filepath.Join(filepath.Dir(sourcePath), target))
		return backend.resolvePath(resolved, link)
	}

	// Vault-absolute path escape check (defense-in-depth).
	if !link.isBasename && pathEscapesVault(target) {
		return 0, "", fmt.Errorf("%w: %s in %s", ErrLinkEscapesVault, link.rawLink, sourcePath)
	}

	// Absolute path (/ prefix, markdown link only): /sub/B.md → sub/B.md
	if strings.HasPrefix(target, "/") {
		stripped := strings.TrimPrefix(target, "/")
		return backend.resolvePath(stripped, link)
	}

	// Wikilink with vault-relative path (contains /, not relative): [[path/to/Note]]
	if (link.linkType == LinkTypeWikilink || link.linkType == LinkTypeFrontmatterWikilink) && !link.isBasename {
		return backend.resolvePath(target, link)
	}

	// Basename resolution (wikilink and markdown)
	if link.isBasename {
		return backend.resolveBasename(target, link)
	}

	// Markdown link with path that is not relative and not / prefix
	return backend.resolvePath(target, link)
}
