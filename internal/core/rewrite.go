package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// rewriteLinkTypes lists every link type whose target can be rewritten by
// rewrite/move/disambiguate operations.
// frontmatter_path is intentionally absent: raw path values are not link
// syntax and cannot be rewritten.
var rewriteLinkTypes = []LinkType{
	LinkTypeWikilink,
	LinkTypeMarkdown,
	LinkTypeFrontmatterWikilink,
}

// isPathLinkType reports whether linkType resolves to a vault path and is
// subject to escape/ambiguity validation. Unlike rewriteLinkTypes, this
// includes frontmatter_path (validated but not rewritable).
func isPathLinkType(linkType LinkType) bool {
	switch linkType {
	case LinkTypeWikilink, LinkTypeMarkdown, LinkTypeFrontmatterWikilink, LinkTypeFrontmatterPath:
		return true
	}
	return false
}

// rewriteBackup holds original file content for rollback on failure.
type rewriteBackup struct {
	path    string
	content []byte
	perm    os.FileMode
}

type rollbackFailure struct {
	action string
	path   string
	err    error
}

var rewriteWriteFile = writeFilePreservePerm
var rollbackWriteFile = writeFilePreservePerm
var rewriteTxExec = func(tx dbExecer, query string, args ...any) (sql.Result, error) {
	return tx.Exec(query, args...)
}

// rewriteEntry holds information needed to rewrite a single edge.
type rewriteEntry struct {
	edgeID     int64
	rawLink    string
	linkType   LinkType
	lineStart  int
	sourcePath string
	sourceID   int64
	newRawLink string
}

// buildRewritePath constructs the vault-relative rewritten path for a link target.
// Only .md extension is removed (e.g. "A.md" → "A", "image.png" → "image.png").
func buildRewritePath(targetPath string) string {
	if strings.HasSuffix(strings.ToLower(targetPath), ".md") {
		return targetPath[:len(targetPath)-3]
	}
	return targetPath
}

// rewriteRawLink replaces the target in a raw link with the rewritten path.
func rewriteRawLink(rawLink string, linkType LinkType, targetPath string) string {
	switch linkType {
	case LinkTypeWikilink, LinkTypeFrontmatterWikilink:
		// rawLink: [[Target]], [[Target|alias]], [[Target#Heading]], [[Target#Heading|alias]]
		parts := splitWikilinkParts(rawLink)

		newPath := buildRewritePath(targetPath)
		return "[[" + newPath + parts.subpath + parts.alias + "]]"

	case LinkTypeMarkdown:
		// rawLink: [text](url), [text](url#frag)
		start := strings.Index(rawLink, "](")
		if start < 0 {
			return rawLink
		}
		textPart := rawLink[:start+2] // "[text]("
		urlPart := rawLink[start+2:]
		urlPart = strings.TrimSuffix(urlPart, ")")

		// Extract fragment.
		var frag string
		if idx := strings.Index(urlPart, "#"); idx >= 0 {
			frag = urlPart[idx:] // includes #
			urlPart = urlPart[:idx]
		}

		// Check if original URL had .md extension.
		hasMdExt := strings.HasSuffix(strings.ToLower(urlPart), ".md")

		newPath := buildRewritePath(targetPath)
		if hasMdExt {
			newPath += ".md"
		}

		return textPart + newPath + frag + ")"
	}
	return rawLink
}

// replaceOutsideInlineCode replaces occurrences of old with new in line,
// but only outside backtick-delimited inline code spans.
func replaceOutsideInlineCode(line, old, new string) string {
	var result strings.Builder
	i := 0
	for i < len(line) {
		if line[i] == '`' {
			// Find the closing backtick.
			end := strings.IndexByte(line[i+1:], '`')
			if end < 0 {
				// No closing backtick — rest of line is code.
				result.WriteString(line[i:])
				return result.String()
			}
			// Copy the inline code span verbatim.
			span := line[i : i+1+end+1]
			result.WriteString(span)
			i += len(span)
			continue
		}
		// Check for old string match.
		if strings.HasPrefix(line[i:], old) {
			result.WriteString(new)
			i += len(old)
			continue
		}
		result.WriteByte(line[i])
		i++
	}
	return result.String()
}

// writeFilePreservePerm writes data to path with the given permission bits.
// os.WriteFile applies umask on file creation, so os.Chmod is called to
// ensure the exact permission bits are set.
func writeFilePreservePerm(path string, data []byte, perm os.FileMode) error {
	if err := os.WriteFile(path, data, perm); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}

func restoreBackupFiles(vaultPath string, backups []rewriteBackup) []rollbackFailure {
	diskPaths := newVaultDiskPathResolver(vaultPath)
	var failures []rollbackFailure
	for _, fb := range backups {
		fullPath, err := diskPaths.existingPath(fb.path)
		if err != nil {
			fullPath = filepath.Join(vaultPath, fb.path)
		}
		if err := rollbackWriteFile(fullPath, fb.content, fb.perm); err != nil {
			failures = append(failures, rollbackFailure{
				action: "restore",
				path:   fb.path,
				err:    err,
			})
		}
	}
	return failures
}

func wrapRollbackFailures(primary error, failures []rollbackFailure) error {
	if primary == nil || len(failures) == 0 {
		return primary
	}

	parts := make([]string, 0, len(failures))
	for _, failure := range failures {
		parts = append(parts, fmt.Sprintf("could not %s %s: %v", failure.action, failure.path, failure.err))
	}
	return fmt.Errorf("%w; rollback failed: %s. Manually resolve vault state, then run `mdhop build` to rebuild the index", primary, strings.Join(parts, "; "))
}

// applyFileRewritesWithRollbackFailures applies rewrite entries to source files.
// On a write or stat error it restores already-written files and returns both
// the primary error and any rollback failures for the caller to report together.
// When entries carry sourceID=0 (scan-mode callers), their mtimes collapse onto
// key 0; this is safe only for callers that discard the mtime map.
func applyFileRewritesWithRollbackFailures(vaultPath string, groups map[string][]rewriteEntry) (map[int64]int64, []rewriteBackup, []rollbackFailure, error) {
	newMtimes := make(map[int64]int64)
	diskPaths := newVaultDiskPathResolver(vaultPath)
	sourcePaths := make([]string, 0, len(groups))
	for sourcePath := range groups {
		sourcePaths = append(sourcePaths, sourcePath)
	}
	sort.Strings(sourcePaths)

	// Phase 1: read all originals before any writes.
	originals := make(map[string][]byte, len(groups))
	perms := make(map[string]os.FileMode, len(groups))
	fullPaths := make(map[string]string, len(groups))
	for _, sourcePath := range sourcePaths {
		fullPath, err := diskPaths.existingPath(sourcePath)
		if err != nil {
			return nil, nil, nil, err
		}
		fullPaths[sourcePath] = fullPath
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, nil, nil, err
		}
		perms[sourcePath] = info.Mode().Perm()
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, nil, nil, err
		}
		originals[sourcePath] = content
	}

	// Phase 2: compute new content and write files.
	var written []rewriteBackup

	restore := func() []rollbackFailure {
		var failures []rollbackFailure
		for _, fb := range written {
			fullPath := fullPaths[fb.path]
			if fullPath == "" {
				fullPath = filepath.Join(vaultPath, fb.path)
			}
			if err := rollbackWriteFile(fullPath, fb.content, fb.perm); err != nil {
				failures = append(failures, rollbackFailure{
					action: "restore",
					path:   fb.path,
					err:    err,
				})
			}
		}
		return failures
	}

	for _, sourcePath := range sourcePaths {
		entries := groups[sourcePath]
		fullPath := fullPaths[sourcePath]
		original := originals[sourcePath]
		lines := strings.Split(string(original), "\n")

		// Group entries by line number.
		lineEntries := make(map[int][]rewriteEntry)
		for _, re := range entries {
			lineEntries[re.lineStart] = append(lineEntries[re.lineStart], re)
		}

		// Apply replacements line by line.
		for lineNum, res := range lineEntries {
			if lineNum < 1 || lineNum > len(lines) {
				continue
			}
			idx := lineNum - 1 // convert 1-based to 0-based
			for _, re := range res {
				lines[idx] = replaceOutsideInlineCode(lines[idx], re.rawLink, re.newRawLink)
			}
		}

		newContent := []byte(strings.Join(lines, "\n"))
		if err := rewriteWriteFile(fullPath, newContent, perms[sourcePath]); err != nil {
			restoreFailures := restore()
			return nil, nil, restoreFailures, err
		}
		written = append(written, rewriteBackup{path: sourcePath, content: original, perm: perms[sourcePath]})

		// Collect new mtime.
		info, err := os.Stat(fullPath)
		if err != nil {
			restoreFailures := restore()
			return nil, nil, restoreFailures, err
		}
		sourceID := entries[0].sourceID
		newMtimes[sourceID] = info.ModTime().Unix()
	}

	return newMtimes, written, nil, nil
}

// isBasenameRawLink checks if a raw_link represents a basename link (no path separators).
func isBasenameRawLink(rawLink string, linkType LinkType) bool {
	switch linkType {
	case LinkTypeWikilink, LinkTypeFrontmatterWikilink:
		// raw_link is like "[[Target]]" or "[[Target|alias]]" or "[[Target#heading]]"
		inner := splitWikilinkParts(rawLink).target
		// Empty target means self-link like [[#Heading]], not a basename link.
		if inner == "" {
			return false
		}
		return !strings.Contains(inner, "/")
	case LinkTypeMarkdown:
		// raw_link is like "[text](url)" or "[text](url#heading)"
		start := strings.Index(rawLink, "](")
		if start < 0 {
			return false
		}
		url := rawLink[start+2:]
		url = strings.TrimSuffix(url, ")")
		// Remove fragment.
		if idx := strings.Index(url, "#"); idx >= 0 {
			url = url[:idx]
		}
		// Empty url means self-link like [text](#heading), not a basename link.
		if url == "" {
			return false
		}
		return !strings.Contains(url, "/")
	case LinkTypeFrontmatterPath:
		// raw_link is the raw frontmatter value; reuse the parser's
		// classification so both stay in sync. Only diagnose reaches this
		// case: rewrite-side callers filter edges by rewriteLinkTypes,
		// which excludes frontmatter_path (raw values are not rewritable).
		occ, ok := frontmatterPathOccur(rawLink, 0)
		return ok && occ.isBasename
	}
	return false
}
