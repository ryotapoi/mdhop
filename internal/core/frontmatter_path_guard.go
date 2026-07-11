package core

import (
	"fmt"
	"strings"
)

// validateFrontmatterPathEdges re-resolves every frontmatter_path edge against
// the post-mutation resolve maps and returns an error if any raw value would
// resolve to a different target. Raw path values are not link syntax and
// cannot be rewritten (ADR 0014), so mutations that would change their
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
	rows, err := db.Query(`SELECT e.raw_link, sn.path, tn.type, COALESCE(tn.path,'')
		FROM edges e
		JOIN nodes sn ON sn.id = e.source_id AND sn.exists_flag = 1
		JOIN nodes tn ON tn.id = e.target_id AND (tn.exists_flag = 1 OR tn.type = 'phantom')
		WHERE e.link_type = 'frontmatter_path'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pathEdge struct {
		rawLink    string
		sourcePath string
		targetType NodeType
		targetPath string
	}
	var edges []pathEdge
	for rows.Next() {
		var e pathEdge
		if err := rows.Scan(&e.rawLink, &e.sourcePath, &e.targetType, &e.targetPath); err != nil {
			return err
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, e := range edges {
		occ, ok := frontmatterPathOccur(e.rawLink, 0)
		if !ok {
			continue
		}
		if e.targetType == NodeTypePhantom {
			// Unresolved values stay unresolved or get promoted to a unique
			// node, unless the mutation makes the basename ambiguous
			// (Pattern B): a full build would then fail, so stop here.
			if occ.isBasename && isAmbiguousBasenameLink(occ.target, rm) {
				candidates := ambiguousCandidates(occ.target, rm)
				return fmt.Errorf("%w: frontmatter value %q in %s (candidates: %s; frontmatter_path values cannot be rewritten)", ErrAmbiguousLink, e.rawLink, e.sourcePath, strings.Join(candidates, ", "))
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
			return fmt.Errorf("%w (frontmatter value %q in %s cannot be rewritten; update it first)", err, e.rawLink, sourcePath)
		}
		if resolved != expected {
			return fmt.Errorf("frontmatter value %q in %s would no longer resolve to %s after this operation (frontmatter_path values cannot be rewritten); update the frontmatter value first", e.rawLink, sourcePath, expected)
		}
	}
	return nil
}

// resolveFrontmatterPathDry resolves a frontmatter_path link against rm
// without creating phantom nodes, returning the resolved vault path ("" if
// the value would become a phantom).
func resolveFrontmatterPathDry(sourcePath string, link linkOccur, rm *resolveMaps) (string, error) {
	backend := newDryLinkResolver(rm)
	id, _, err := resolveLinkWithBackend(sourcePath, link, backend)
	if err != nil {
		return "", err
	}
	return backend.pathForID(id), nil
}
