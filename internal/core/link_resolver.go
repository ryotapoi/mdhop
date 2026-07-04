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
