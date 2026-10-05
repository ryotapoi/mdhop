package core

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// movedLinkMaps is the post-move resolution information needed to decide an
// outgoing link rewrite. Its maps are read-only inputs.
type movedLinkMaps struct {
	movedFromTo        map[string]string
	basenameToPath     map[string]string
	rootBasenameToPath map[string]string
	basenameCounts     map[string]int
}

// rewriteMovedOutgoingLink decides the rewrite for an outgoing link in a moved
// note using only already-collected paths and post-move resolution information.
// preMoveTargetPath is empty when the caller could not resolve an edge target.
func rewriteMovedOutgoingLink(link linkOccur, from, to, preMoveTargetPath string, maps movedLinkMaps) (outgoingRewrite, bool, error) {
	if !isPathLinkType(link.linkType) || link.linkType == LinkTypeMarkdownReference {
		return outgoingRewrite{}, false, nil
	}
	if link.isBasename {
		if preMoveTargetPath == "" {
			return outgoingRewrite{}, false, nil
		}

		postMoveTargetPath := preMoveTargetPath
		if newPath, ok := maps.movedFromTo[preMoveTargetPath]; ok {
			postMoveTargetPath = newPath
		}

		bk := basenameKey(link.target)
		needRewrite := false
		if basenameKey(postMoveTargetPath) != bk {
			needRewrite = true
		} else if path, ok := maps.basenameToPath[bk]; ok {
			needRewrite = path != postMoveTargetPath
		} else if path, ok := maps.rootBasenameToPath[bk]; ok {
			needRewrite = path != postMoveTargetPath
		} else if maps.basenameCounts[bk] > 1 {
			needRewrite = true
		}
		if !needRewrite {
			return outgoingRewrite{}, false, nil
		}
		return newOutgoingRewrite(link, rewriteRawLink(link.rawLink, link.linkType, postMoveTargetPath, link.inTable)), true, nil
	}

	// Move preserves existing source-relative outgoing links as an intentional
	// exception to vault-relative rewrites.
	if link.isRelative {
		newRawLink, err := rewriteOutgoingRelativeLink(link.rawLink, link.linkType, from, to, maps.movedFromTo, preMoveTargetPath, link.inTable)
		if err != nil {
			return outgoingRewrite{}, false, err
		}
		if newRawLink == link.rawLink {
			return outgoingRewrite{}, false, nil
		}
		return newOutgoingRewrite(link, newRawLink), true, nil
	}

	if link.target == "" || preMoveTargetPath == "" {
		return outgoingRewrite{}, false, nil
	}
	if newPath, ok := maps.movedFromTo[preMoveTargetPath]; ok {
		return newOutgoingRewrite(link, rewriteRawLink(link.rawLink, link.linkType, newPath, link.inTable)), true, nil
	}
	return outgoingRewrite{}, false, nil
}

func newOutgoingRewrite(link linkOccur, newRawLink string) outgoingRewrite {
	return outgoingRewrite{
		rawLink:    link.rawLink,
		newRawLink: newRawLink,
		linkType:   link.linkType,
		lineStart:  link.lineStart,
	}
}

// relativeLinkParts contains syntax-specific pieces needed by the common
// relative-link rewrite procedure. The target itself is always rewritten by
// the shared path calculation below; only wrappers and extension policy vary by type.
type relativeLinkParts struct {
	prefix       string
	target       string
	suffix       string
	preserveMD   bool
	stripMovedMD bool
}

func parseRelativeLink(rawLink string, linkType LinkType, table ...bool) (relativeLinkParts, bool) {
	switch linkType {
	case LinkTypeWikilink, LinkTypeFrontmatterWikilink:
		parts := splitWikilinkParts(rawLink, table...)
		return relativeLinkParts{prefix: "[[", target: parts.target, suffix: parts.subpath + parts.alias + "]]", stripMovedMD: true}, true
	case LinkTypeMarkdown:
		start := strings.Index(rawLink, "](")
		if start < 0 {
			return relativeLinkParts{}, false
		}
		prefix := rawLink[:start+2]
		urlPart := strings.TrimSuffix(rawLink[start+2:], ")")
		trimmedLeft := strings.TrimLeftFunc(urlPart, unicode.IsSpace)
		prefix += urlPart[:len(urlPart)-len(trimmedLeft)]
		trimmed := strings.TrimRightFunc(trimmedLeft, unicode.IsSpace)
		trailing := trimmedLeft[len(trimmed):]
		target, fragment, _ := markdownDestination(trimmed)
		return relativeLinkParts{prefix: prefix, target: target, suffix: encodeMarkdownDestination("", fragment) + trailing + ")", preserveMD: strings.HasSuffix(strings.ToLower(target), ".md")}, true
	default:
		return relativeLinkParts{}, false
	}
}

func resolveMovedRelativeTarget(target string, movedFromTo map[string]string, stripMovedMD bool) string {
	if movedFromTo == nil {
		return target
	}
	keys := []string{target, target + ".md"}
	if !stripMovedMD {
		keys = append(keys, strings.TrimSuffix(target, ".md")+".md")
	}
	for _, key := range keys {
		if newTarget, ok := movedFromTo[key]; ok {
			if stripMovedMD {
				return strings.TrimSuffix(newTarget, ".md")
			}
			return newTarget
		}
	}
	return target
}

// rewriteOutgoingRelativeLink rewrites a relative link in the moved file
// from the old path perspective to the new path perspective.
// If movedFromTo is non-nil, it also checks whether the target was moved.
func rewriteOutgoingRelativeLink(rawLink string, linkType LinkType, from, to string, movedFromTo map[string]string, preMoveTargetPath string, table ...bool) (string, error) {
	parts, ok := parseRelativeLink(rawLink, linkType, table...)
	if !ok {
		return rawLink, nil
	}
	resolvedTarget := NormalizePath(filepath.Join(filepath.Dir(from), parts.target))
	if movedTarget, ok := movedFromTo[preMoveTargetPath]; preMoveTargetPath != "" && ok {
		resolvedTarget = movedTarget
	} else {
		resolvedTarget = resolveMovedRelativeTarget(resolvedTarget, movedFromTo, parts.stripMovedMD)
	}

	rel, err := filepath.Rel(filepath.Dir(to), resolvedTarget)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if strings.HasPrefix(NormalizePath(filepath.Join(filepath.Dir(to), rel)), "..") {
		return "", fmt.Errorf("rewritten link would escape vault: %s", rawLink)
	}
	if !strings.HasPrefix(rel, "..") {
		rel = "./" + rel
	}
	if parts.preserveMD {
		if !strings.HasSuffix(strings.ToLower(rel), ".md") {
			rel += ".md"
		}
	} else {
		rel = strings.TrimSuffix(rel, ".md")
	}
	if linkType == LinkTypeMarkdown {
		rel = encodeMarkdownComponent(rel)
	} else if !wikilinkRepresentable(rel, "", linkType) || len(table) > 0 && table[0] && strings.Contains(parts.suffix, `\|`) && !tableWikiAliasSafe(rel+strings.Split(parts.suffix, `\|`)[0]) {
		return "", fmt.Errorf("cannot preserve wikilink destination while rewriting %q", rawLink)
	}
	return parts.prefix + rel + parts.suffix, nil
}
