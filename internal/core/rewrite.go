package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// rewriteLinkTypes lists every link type whose target can be rewritten by
// rewrite/move/disambiguate operations.
// frontmatter_path and markdown_reference are intentionally absent: raw path
// values are not link syntax, and reference definitions are not rewritten.
var rewriteLinkTypes = []LinkType{
	LinkTypeWikilink,
	LinkTypeMarkdown,
	LinkTypeFrontmatterWikilink,
}

// isPathLinkType reports whether linkType resolves to a vault path and is
// subject to escape/ambiguity validation. Unlike rewriteLinkTypes, this
// includes frontmatter_path and markdown_reference (validated but not rewritable).
func isPathLinkType(linkType LinkType) bool {
	switch linkType {
	case LinkTypeWikilink, LinkTypeMarkdown, LinkTypeMarkdownReference, LinkTypeFrontmatterWikilink, LinkTypeFrontmatterPath:
		return true
	}
	return false
}

// rewriteBackup holds original file content and metadata for rollback on failure.
type rewriteBackup struct {
	path    string
	content []byte
	perm    os.FileMode
	mtime   time.Time
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
	inTable    bool
	edgeID     int64
	rawLink    string
	linkType   LinkType
	lineStart  int
	sourcePath string
	sourceID   int64
	newRawLink string
}

func checkRewriteSourcesFresh(db dbExecer, diskPaths *vaultDiskPathResolver, rewrites []rewriteEntry) error {
	checked := make(map[int64]bool)
	for _, re := range rewrites {
		if checked[re.sourceID] {
			continue
		}
		checked[re.sourceID] = true
		var dbMtime int64
		if err := db.QueryRow("SELECT mtime FROM nodes WHERE id = ?", re.sourceID).Scan(&dbMtime); err != nil {
			return err
		}
		fullPath, err := diskPaths.existingPath(re.sourcePath)
		if err != nil {
			return err
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return err
		}
		if info.ModTime().Unix() != dbMtime {
			return fmt.Errorf("%w: %s", ErrSourceStale, re.sourcePath)
		}
	}
	return nil
}

type preparedFileRewrite struct {
	vaultPath string
	path      string
	fullPath  string
	original  []byte
	candidate []byte
	perm      os.FileMode
	mtime     time.Time
	entries   []rewriteEntry
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
func rewriteRawLink(rawLink string, linkType LinkType, targetPath string, table ...bool) string {
	switch linkType {
	case LinkTypeWikilink, LinkTypeFrontmatterWikilink:
		// rawLink: [[Target]], [[Target|alias]], [[Target#Heading]], [[Target#Heading|alias]]
		parts := splitWikilinkParts(rawLink, table...)

		newPath := buildRewritePath(targetPath)
		if !wikilinkRepresentable(newPath, parts.subpath, linkType) || len(table) > 0 && table[0] && parts.alias != "" && !tableWikiAliasSafe(newPath+parts.subpath) {
			return ""
		}
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

		target, frag, _ := markdownDestination(urlPart)
		hasMdExt := strings.HasSuffix(strings.ToLower(target), ".md")
		newPath := buildRewritePath(targetPath)
		if hasMdExt {
			newPath += ".md"
		}
		return textPart + encodeMarkdownDestination(newPath, frag) + ")"
	}
	return rawLink
}

// replaceBodyLink rewrites matching raw links outside inline code. Markdown
// matches must start at a disjoint bracket span, outside wikilinks and labels.
func replaceBodyLink(line, old, new string, linkType LinkType) string {
	clean := line
	if linkType == LinkTypeMarkdown {
		clean = maskWikiLinks(stripInlineCode(line))
	}
	var result strings.Builder
	for i := 0; i < len(line); {
		if linkType == LinkTypeMarkdown {
			if clean[i] != '[' {
				result.WriteByte(line[i])
				i++
				continue
			}
			end, _ := referenceBracketEnd(clean, i)
			if end < 0 {
				result.WriteString(line[i:])
				break
			}
			end++
			if end < len(clean) && clean[end] == '(' {
				close := markdownDestinationEnd(clean, end+1)
				if close < 0 {
					result.WriteString(line[i:])
					break
				}
				end = close + 1
			}
			if line[i:end] == old {
				result.WriteString(new)
			} else {
				result.WriteString(line[i:end])
			}
			i = end
			continue
		}
		if line[i] == '`' {
			backslashes := 0
			for j := i - 1; j >= 0 && line[j] == '\\'; j-- {
				backslashes++
			}
			if backslashes%2 == 0 {
				end := inlineCodeEnd(line, i)
				result.WriteString(line[i:end])
				i = end
				continue
			}
		}
		if strings.HasPrefix(clean[i:], old) {
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
		if os.IsNotExist(err) {
			fullPath = filepath.Join(vaultPath, fb.path)
			err = nil
		}
		if err == nil {
			err = validateVaultWritePath(vaultPath, fullPath)
		}
		if err == nil {
			err = rollbackWriteFile(fullPath, fb.content, fb.perm)
		}
		if err == nil {
			err = os.Chtimes(fullPath, time.Time{}, fb.mtime)
		}
		if err != nil {
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
func applyFileRewritesWithRollbackFailures(vaultPath string, rewrites []rewriteEntry) (map[int64]int64, []rewriteBackup, []rollbackFailure, error) {
	if len(rewrites) == 0 {
		return nil, nil, nil, nil
	}
	prepared, err := prepareFileRewrites(vaultPath, rewrites)
	if err != nil {
		return nil, nil, nil, err
	}
	return applyPreparedFileRewrites(prepared)
}

// prepareFileRewrites reads and validates every candidate before a caller can
// write any file. The candidate is also the exact content later written.
func prepareFileRewrites(vaultPath string, rewrites []rewriteEntry) ([]preparedFileRewrite, error) {
	groups := make(map[string][]rewriteEntry)
	for _, re := range rewrites {
		if re.newRawLink == "" {
			return nil, fmt.Errorf("cannot preserve wikilink destination while rewriting %q", re.rawLink)
		}
		groups[re.sourcePath] = append(groups[re.sourcePath], re)
	}
	diskPaths := newVaultDiskPathResolver(vaultPath)
	sourcePaths := make([]string, 0, len(groups))
	for sourcePath := range groups {
		sourcePaths = append(sourcePaths, sourcePath)
	}
	sort.Strings(sourcePaths)

	prepared := make([]preparedFileRewrite, 0, len(sourcePaths))
	for _, sourcePath := range sourcePaths {
		fullPath, err := diskPaths.writablePath(sourcePath)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		candidate, err := rewriteContentCandidate(content, groups[sourcePath])
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedFileRewrite{vaultPath: vaultPath, path: sourcePath, fullPath: fullPath, original: content, candidate: candidate, perm: info.Mode().Perm(), mtime: info.ModTime(), entries: groups[sourcePath]})
	}
	return prepared, nil
}

func rewriteContentCandidate(content []byte, rewrites []rewriteEntry) ([]byte, error) {
	for _, re := range rewrites {
		if re.newRawLink == "" {
			return nil, fmt.Errorf("cannot preserve wikilink destination while rewriting %q", re.rawLink)
		}
	}
	candidate, err := rewriteFrontmatterCandidate(content, rewrites)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(candidate), "\n")
	lineEntries := make(map[int][]rewriteEntry)
	for _, re := range rewrites {
		if re.linkType != LinkTypeFrontmatterWikilink {
			lineEntries[re.lineStart] = append(lineEntries[re.lineStart], re)
		}
	}
	for lineNum, res := range lineEntries {
		if lineNum < 1 || lineNum > len(lines) {
			continue
		}
		for _, re := range res {
			lines[lineNum-1] = replaceBodyLink(lines[lineNum-1], re.rawLink, re.newRawLink, re.linkType)
		}
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func applyPreparedFileRewrites(prepared []preparedFileRewrite) (map[int64]int64, []rewriteBackup, []rollbackFailure, error) {
	newMtimes := make(map[int64]int64)
	files := make(map[string]preparedFileRewrite, len(prepared))
	for _, file := range prepared {
		if err := validateVaultWritePath(file.vaultPath, file.fullPath); err != nil {
			return nil, nil, nil, err
		}
		files[file.path] = file
	}

	var written []rewriteBackup

	restore := func() []rollbackFailure {
		var failures []rollbackFailure
		for _, fb := range written {
			file := files[fb.path]
			err := validateVaultWritePath(file.vaultPath, file.fullPath)
			if err == nil {
				err = rollbackWriteFile(file.fullPath, fb.content, fb.perm)
			}
			if err == nil {
				err = os.Chtimes(file.fullPath, time.Time{}, fb.mtime)
			}
			if err != nil {
				failures = append(failures, rollbackFailure{
					action: "restore",
					path:   fb.path,
					err:    err,
				})
			}
		}
		return failures
	}

	for _, file := range prepared {
		written = append(written, rewriteBackup{path: file.path, content: file.original, perm: file.perm, mtime: file.mtime})
		if err := rewriteWriteFile(file.fullPath, file.candidate, file.perm); err != nil {
			restoreFailures := restore()
			return nil, nil, restoreFailures, err
		}

		// Collect new mtime.
		info, err := os.Stat(file.fullPath)
		if err != nil {
			restoreFailures := restore()
			return nil, nil, restoreFailures, err
		}
		sourceID := file.entries[0].sourceID
		newMtimes[sourceID] = info.ModTime().Unix()
	}

	return newMtimes, written, nil, nil
}

// isBasenameRawLink checks if a raw_link represents a basename link (no path separators).
func isBasenameRawLink(rawLink string, linkType LinkType, table ...bool) bool {
	switch linkType {
	case LinkTypeWikilink, LinkTypeFrontmatterWikilink:
		// raw_link is like "[[Target]]" or "[[Target|alias]]" or "[[Target#heading]]"
		inner := splitWikilinkParts(rawLink, table...).target
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
		target, _, external := markdownDestination(url)
		return target != "" && !external && isBasenameLink(target)
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
