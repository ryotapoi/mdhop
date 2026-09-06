package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

type linkResolverBackend[T any] interface {
	resolveSelf(sourcePath string, link linkOccur) (T, string, error)
	resolveTag(link linkOccur) (T, string, error)
	resolvePath(resolved string, link linkOccur) (T, string, error)
	resolveBasename(target string, link linkOccur) (T, string, error)
}

// dryLinkResolver resolves only existing note and asset paths without creating
// phantom or tag nodes.
type dryLinkResolver struct {
	rm *resolveMaps
}

func (r dryLinkResolver) resolveSelf(sourcePath string, link linkOccur) (string, string, error) {
	return sourcePath, link.subpath, nil
}

func (r dryLinkResolver) resolveTag(link linkOccur) (string, string, error) {
	return "", "", nil
}

func (r dryLinkResolver) resolvePath(resolved string, link linkOccur) (string, string, error) {
	lower := strings.ToLower(NormalizePath(resolved))
	if path, ok := r.rm.pathSet[lower]; ok {
		return path, link.subpath, nil
	}
	if path, ok := r.rm.pathSet[lower+".md"]; ok {
		return path, link.subpath, nil
	}
	if path, ok := r.rm.assetPathSet[lower]; ok {
		return path, link.subpath, nil
	}
	return "", link.subpath, nil
}

func (r dryLinkResolver) resolveBasename(target string, link linkOccur) (string, string, error) {
	lower := strings.ToLower(normalizeTextNFC(target))
	if path, ok := r.rm.basenameToPath[lower]; ok {
		return path, link.subpath, nil
	}
	if path, ok := r.rm.rootBasenameToPath[lower]; ok {
		return path, link.subpath, nil
	}
	if path, ok := r.rm.assetBasenameToPath[lower]; ok {
		return path, link.subpath, nil
	}
	if path, ok := r.rm.assetRootBasenameToPath[lower]; ok {
		return path, link.subpath, nil
	}
	return "", link.subpath, nil
}

// resolveLinkWithBackend owns the link-kind dispatch order. Backends provide
// storage-specific lookups while preserving the shared resolution semantics.
func resolveLinkWithBackend[T any](sourcePath string, link linkOccur, backend linkResolverBackend[T]) (T, string, error) {
	var zero T
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
			return zero, "", fmt.Errorf("%w: %s in %s", ErrLinkEscapesVault, link.rawLink, sourcePath)
		}
		resolved := NormalizePath(filepath.Join(filepath.Dir(sourcePath), target))
		return backend.resolvePath(resolved, link)
	}

	// Vault-absolute path escape check (defense-in-depth).
	if !link.isBasename && pathEscapesVault(target) {
		return zero, "", fmt.Errorf("%w: %s in %s", ErrLinkEscapesVault, link.rawLink, sourcePath)
	}

	// Absolute path (/ prefix, markdown link only): /sub/B.md → sub/B.md
	if strings.HasPrefix(target, "/") {
		stripped := strings.TrimPrefix(target, "/")
		return backend.resolvePath(stripped, link)
	}

	// Basename resolution (wikilink and markdown)
	if link.isBasename {
		return backend.resolveBasename(target, link)
	}

	// Remaining vault-relative paths.
	return backend.resolvePath(target, link)
}
