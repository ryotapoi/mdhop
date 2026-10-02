package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validateFrontmatterPathEdges re-resolves frontmatter_path and reference edges against
// the post-mutation resolve maps and returns an error if any raw value would
// resolve to a different target. Raw path values are not link syntax and
// cannot be rewritten (ADR 0014); reference definitions are also not rewritten.
// Mutations that would change their
// resolution must fail instead of leaving the index inconsistent with the
// next build.
//
// movedFromTo maps old paths to new paths for nodes moved by the mutation
// (nil for add). It is applied to both source paths (relative resolution
// base) and expected target paths (DB still holds pre-move paths).
func validateFrontmatterPathEdges(db dbExecer, rm *resolveMaps, movedFromTo map[string]string) error {
	// Phantom targets (exists_flag = 0) stay in: their basename raws must be
	// checked for ambiguity below. Other exists_flag = 0 targets (deleted
	// but still referenced) are excluded: their raw values are already stale
	// and re-resolve from scratch on the next build.
	rows, err := db.Query(`SELECT e.raw_link, sn.path, tn.type, COALESCE(tn.path,''), e.link_type, COALESCE(e.reference_target,'')
		FROM edges e
		JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
		JOIN nodes tn ON tn.id = e.target_id AND (tn.exists_flag = 1 OR tn.type = ?)
		WHERE e.link_type IN (?, ?)`, NodeTypePhantom, LinkTypeFrontmatterPath, LinkTypeMarkdownReference)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pathEdge struct {
		rawLink         string
		sourcePath      string
		targetType      NodeType
		targetPath      string
		linkType        LinkType
		referenceTarget string
	}
	var edges []pathEdge
	for rows.Next() {
		var e pathEdge
		if err := rows.Scan(&e.rawLink, &e.sourcePath, &e.targetType, &e.targetPath, &e.linkType, &e.referenceTarget); err != nil {
			return err
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, e := range edges {
		occ, ok := frontmatterPathOccur(e.rawLink, 0)
		if e.linkType == LinkTypeMarkdownReference {
			occ, ok = markdownDestinationOccur(e.referenceTarget, e.rawLink, e.linkType, 0)
		}
		if !ok {
			continue
		}
		if e.targetType == NodeTypePhantom {
			if e.linkType == LinkTypeMarkdownReference && occ.isRelative {
				if to, moved := movedFromTo[e.sourcePath]; moved && filepath.Clean(filepath.Join(filepath.Dir(to), occ.target)) != filepath.Clean(filepath.Join(filepath.Dir(e.sourcePath), occ.target)) {
					return fmt.Errorf("reference %q in %s would change destination after this operation; reference definitions cannot be rewritten", e.rawLink, e.sourcePath)
				}
			}
			// Unresolved values stay unresolved or get promoted to a unique
			// node, unless the mutation makes the basename ambiguous
			// (Pattern B): a full build would then fail, so stop here.
			if occ.isBasename && isAmbiguousBasenameLink(occ.target, rm) {
				candidates := ambiguousCandidates(occ.target, rm)
				return fmt.Errorf("%w: non-rewritable link %q in %s (candidates: %s; frontmatter_path values and reference definitions cannot be rewritten)", ErrAmbiguousLink, e.rawLink, e.sourcePath, strings.Join(candidates, ", "))
			}
			continue
		}
		sourcePath := e.sourcePath
		if to, moved := movedFromTo[sourcePath]; moved {
			sourcePath = to
		}
		expected := e.targetPath
		if to, moved := movedFromTo[expected]; moved {
			expected = to
		}
		resolved, err := resolveFrontmatterPathDry(sourcePath, occ, rm)
		if err != nil {
			return fmt.Errorf("%w (non-rewritable link %q in %s cannot be rewritten; update it first)", err, e.rawLink, sourcePath)
		}
		if resolved != expected {
			return fmt.Errorf("non-rewritable link %q in %s would no longer resolve to %s after this operation (frontmatter_path values and reference definitions cannot be rewritten); update the value or definition first", e.rawLink, sourcePath, expected)
		}
	}
	return nil
}

// resolveFrontmatterPathDry resolves a frontmatter_path link against rm
// without creating phantom nodes, returning the resolved vault path ("" if
// the value would become a phantom).
func resolveFrontmatterPathDry(sourcePath string, link linkOccur, rm *resolveMaps) (string, error) {
	resolved, _, err := resolveLinkWithBackend(sourcePath, link, dryLinkResolver{rm: rm})
	if err != nil {
		return "", err
	}
	return resolved, nil
}
