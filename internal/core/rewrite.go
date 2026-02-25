package core

import (
	"os"
	"path/filepath"
	"strings"
)

// rewriteBackup holds original file content for rollback on failure.
type rewriteBackup struct {
	path    string
	content []byte
	perm    os.FileMode
}

// rewriteEntry holds information needed to rewrite a single edge.
type rewriteEntry struct {
	edgeID     int64
	rawLink    string
	linkType   string
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
func rewriteRawLink(rawLink, linkType, targetPath string) string {
	switch linkType {
	case "wikilink":
		// rawLink: [[Target]], [[Target|alias]], [[Target#Heading]], [[Target#Heading|alias]]
		inner := strings.TrimPrefix(rawLink, "[[")
		inner = strings.TrimSuffix(inner, "]]")

		var alias, subpath string
		// Extract alias (after |).
		if idx := strings.Index(inner, "|"); idx >= 0 {
			alias = inner[idx:] // includes |
			inner = inner[:idx]
		}
		// Extract subpath (after #).
		if idx := strings.Index(inner, "#"); idx >= 0 {
			subpath = inner[idx:] // includes #
		}

		newPath := buildRewritePath(targetPath)
		return "[[" + newPath + subpath + alias + "]]"

	case "markdown":
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

// restoreBackups restores files to their original content (best-effort).
func restoreBackups(vaultPath string, backups []rewriteBackup) {
	for _, fb := range backups {
		_ = writeFilePreservePerm(filepath.Join(vaultPath, fb.path), fb.content, fb.perm)
	}
}

// applyFileRewrites applies rewrite entries to source files on disk.
// Returns a map of sourceID → new mtime after writing, and backups for rollback.
// On error during write, restores already-written files (best-effort).
func applyFileRewrites(vaultPath string, groups map[string][]rewriteEntry) (map[int64]int64, []rewriteBackup, error) {
	newMtimes := make(map[int64]int64)

	// Phase 1: read all originals before any writes.
	originals := make(map[string][]byte, len(groups))
	perms := make(map[string]os.FileMode, len(groups))
	for sourcePath := range groups {
		fullPath := filepath.Join(vaultPath, sourcePath)
		info, err := os.Stat(fullPath)
		if err != nil {
			return nil, nil, err
		}
		perms[sourcePath] = info.Mode().Perm()
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, nil, err
		}
		originals[sourcePath] = content
	}

	// Phase 2: compute new content and write files.
	var written []rewriteBackup

	restore := func() {
		for _, fb := range written {
			_ = writeFilePreservePerm(filepath.Join(vaultPath, fb.path), fb.content, fb.perm)
		}
	}

	for sourcePath, entries := range groups {
		fullPath := filepath.Join(vaultPath, sourcePath)
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
		if err := writeFilePreservePerm(fullPath, newContent, perms[sourcePath]); err != nil {
			restore()
			return nil, nil, err
		}
		written = append(written, rewriteBackup{path: sourcePath, content: original, perm: perms[sourcePath]})

		// Collect new mtime.
		info, err := os.Stat(fullPath)
		if err != nil {
			restore()
			return nil, nil, err
		}
		sourceID := entries[0].sourceID
		newMtimes[sourceID] = info.ModTime().Unix()
	}

	return newMtimes, written, nil
}

// isBasenameRawLink checks if a raw_link represents a basename link (no path separators).
func isBasenameRawLink(rawLink, linkType string) bool {
	switch linkType {
	case "wikilink":
		// raw_link is like "[[Target]]" or "[[Target|alias]]" or "[[Target#heading]]"
		inner := strings.TrimPrefix(rawLink, "[[")
		inner = strings.TrimSuffix(inner, "]]")
		// Remove alias part.
		if idx := strings.Index(inner, "|"); idx >= 0 {
			inner = inner[:idx]
		}
		// Remove subpath (heading).
		if idx := strings.Index(inner, "#"); idx >= 0 {
			inner = inner[:idx]
		}
		// Empty target means self-link like [[#Heading]], not a basename link.
		if inner == "" {
			return false
		}
		return !strings.Contains(inner, "/")
	case "markdown":
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
	}
	return false
}
