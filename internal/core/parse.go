package core

import (
	"strings"
	"unicode"
)

type linkOccur struct {
	target     string
	isBasename bool
	isRelative bool
	linkType   LinkType
	rawLink    string
	// frontmatterKey is set only for links parsed from YAML frontmatter.
	// Body links keep it empty and are stored as NULL in edges.frontmatter_key.
	frontmatterKey  string
	referenceTarget string
	subpath         string
	lineStart       int
	lineEnd         int
}

type parseResult struct {
	Links []linkOccur
	Meta  []FrontmatterEntry
}

// parseLinks parses wikilinks, inline and reference Markdown links, and body
// and frontmatter tags from content.
func parseLinks(content string) parseResult {
	var out []linkOccur
	lines := strings.Split(content, "\n")

	// Parse frontmatter first.
	var result parseResult
	fmEnd := frontmatterEnd(lines)
	if fmEnd > 0 {
		fm := parseFrontmatter(lines[:fmEnd+1])
		out = append(out, fm.links...)
		result.Meta = fm.meta
	}

	definitions := make(map[string]string)
	walkBodyLines(lines, fmEnd, func(_ int, raw, _ string) {
		label, destination, definition := referenceDefinition(raw)
		if definition && label != "" && destination != "" {
			if _, exists := definitions[label]; !exists {
				definitions[label] = destination
			}
		}
	})
	pendingReference := 0
	previousBodyLine := 0
	walkBodyLines(lines, fmEnd, func(lineNum int, raw, clean string) {
		// Blank paragraphs and skipped fenced blocks cannot continue a label.
		if strings.TrimSpace(raw) == "" || lineNum != previousBodyLine+1 {
			pendingReference = 0
		}
		previousBodyLine = lineNum
		if _, _, definition := referenceDefinition(raw); definition {
			pendingReference = 0
			return
		}
		out = append(out, parseWikiLinks(clean, lineNum)...)
		clean = maskWikiLinks(clean)
		clean = maskReferenceContinuation(clean, &pendingReference)
		links, tagLine, pending := parseBodyMarkdown(clean, lineNum, definitions)
		if pending > 0 {
			pendingReference = pending
		}
		// Suppress only a construct that actually closes within this paragraph.
		// A literal unmatched bracket must not hide later independent links.
		if pendingReference > 0 && !referenceContinuationCloses(lines, lineNum, pendingReference) {
			pendingReference = 0
		}
		out = append(out, links...)
		out = append(out, parseTags(tagLine, lineNum)...)
	})
	result.Links = out
	return result
}

// walkBodyLines iterates the body lines of a Markdown document, skipping the
// frontmatter block and fenced code blocks. fmEnd is the index of the closing
// "---" of frontmatter (-1 or 0 if none), as returned by frontmatterEnd. For
// each body line it calls fn with the 1-based line number, the raw line, and
// the line with inline code spans stripped. Link/tag parsing uses clean (so
// backticked text is ignored); heading collection uses raw (so backticked text
// inside a heading is preserved). This is the shared scan skeleton so
// fence/frontmatter handling stays in one place.
func walkBodyLines(lines []string, fmEnd int, fn func(lineNum int, raw, clean string)) {
	var fence fenceDelimiter
	startLine := 0
	if fmEnd > 0 {
		startLine = fmEnd + 1
	}
	for i := startLine; i < len(lines); i++ {
		lineNum := i + 1 // 1-based
		trim := strings.TrimSpace(lines[i])
		if fence.marker != 0 {
			if fence.closes(trim) {
				fence = fenceDelimiter{}
			}
			continue
		}
		if opening, ok := parseFenceOpening(trim); ok {
			fence = opening
			continue
		}
		fn(lineNum, lines[i], stripInlineCode(lines[i]))
	}
}

// fenceDelimiter records the marker and length of an opening fenced code
// block. A closing fence must use the same marker at least as many times and
// contain only whitespace after it.
type fenceDelimiter struct {
	marker byte
	length int
}

func parseFenceOpening(line string) (fenceDelimiter, bool) {
	if len(line) == 0 || (line[0] != '`' && line[0] != '~') {
		return fenceDelimiter{}, false
	}
	length := 0
	for length < len(line) && line[length] == line[0] {
		length++
	}
	if length < 3 {
		return fenceDelimiter{}, false
	}
	return fenceDelimiter{marker: line[0], length: length}, true
}

func (f fenceDelimiter) closes(line string) bool {
	length := 0
	for length < len(line) && line[length] == f.marker {
		length++
	}
	return length >= f.length && strings.TrimSpace(line[length:]) == ""
}

// parseLinksWithLinkKeys parses links like parseLinks and additionally turns
// raw path values of the configured meta.link_keys into frontmatter_path
// links. Use this at edge-generation sites (build/update/add/move) so the
// graph reflects link-key values; plain parseLinks never emits them.
func parseLinksWithLinkKeys(content string, linkKeys []string) parseResult {
	pr := parseLinks(content)
	pr.Links = append(pr.Links, frontmatterPathLinks(pr.Meta, linkKeys)...)
	return pr
}

// inlineCodeEnd returns the end of a code span opened at start. Only a
// backtick run of the same length closes it; an unclosed span consumes the line.
func inlineCodeEnd(line string, start int) int {
	openingEnd := start
	for openingEnd < len(line) && line[openingEnd] == '`' {
		openingEnd++
	}
	length := openingEnd - start
	for i := openingEnd; i < len(line); {
		if line[i] != '`' {
			i++
			continue
		}
		end := i
		for end < len(line) && line[end] == '`' {
			end++
		}
		if end-i == length {
			return end
		}
		i = end
	}
	return len(line)
}

func stripInlineCode(line string) string {
	var out strings.Builder
	for i := 0; i < len(line); {
		if line[i] == '`' {
			i = inlineCodeEnd(line, i)
			continue
		}
		out.WriteByte(line[i])
		i++
	}
	return out.String()
}

// stripWikiLinks removes [[...]] from a line to avoid tag false positives.
func stripWikiLinks(line string) string {
	for {
		start := strings.Index(line, "[[")
		if start == -1 {
			break
		}
		end := strings.Index(line[start+2:], "]]")
		if end == -1 {
			break
		}
		end = start + 2 + end + 2
		line = line[:start] + line[end:]
	}
	return line
}

// stripMarkdownLinks masks inline Markdown links to avoid tag false positives.
func stripMarkdownLinks(line string) string {
	_, masked, _ := parseBodyMarkdown(line, 0, nil)
	return masked
}

func parseWikiLinks(line string, lineNum int) []linkOccur {
	var out []linkOccur
	for _, span := range wikiLinkSpans(line) {
		rawLink := span.raw
		inner := rawLink[2 : len(rawLink)-2]

		name := splitAlias(inner)
		target, subpath := extractSubpath(name)

		if target == "" && subpath != "" {
			// [[#Heading]] — self-link
			out = append(out, linkOccur{
				target:     "",
				isBasename: false,
				isRelative: false,
				linkType:   LinkTypeWikilink,
				rawLink:    rawLink,
				subpath:    subpath,
				lineStart:  lineNum,
				lineEnd:    lineNum,
			})
		} else if target != "" {
			out = append(out, linkOccur{
				target:     normalizeBasename(target),
				isBasename: isBasenameLink(target),
				isRelative: isRelativePath(target),
				linkType:   LinkTypeWikilink,
				rawLink:    rawLink,
				subpath:    subpath,
				lineStart:  lineNum,
				lineEnd:    lineNum,
			})
		}
	}
	return out
}

type wikiLinkSpan struct {
	raw        string
	start, end int
}

func wikiLinkSpans(line string) []wikiLinkSpan {
	var spans []wikiLinkSpan
	for offset := 0; ; {
		start := strings.Index(line[offset:], "[[")
		if start < 0 {
			return spans
		}
		start += offset
		close := strings.Index(line[start+2:], "]]")
		if close < 0 {
			return spans
		}
		end := start + 2 + close + 2
		inner := line[start+2 : end-2]
		name := splitAlias(inner)
		target, subpath := extractSubpath(name)
		if target != "" || subpath != "" {
			spans = append(spans, wikiLinkSpan{raw: line[start:end], start: start, end: end})
		}
		offset = end
	}
}

// markdownDestinationEnd finds the closing delimiter after a destination,
// keeping balanced parentheses inside the destination.
func markdownDestinationEnd(line string, start int) int {
	depth := 0
	for i := start; i < len(line); i++ {
		switch line[i] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

func parseMarkdownLinks(line string, lineNum int) []linkOccur {
	links, _, _ := parseBodyMarkdown(maskWikiLinks(line), lineNum, nil)
	return links
}

func maskWikiLinks(line string) string {
	masked := []byte(line)
	for _, span := range wikiLinkSpans(line) {
		for i := span.start; i < span.end; i++ {
			masked[i] = ' '
		}
	}
	return string(masked)
}

// isTagRune reports whether r is allowed in a tag body (blacklist approach, Obsidian-compatible).
func isTagRune(r rune) bool {
	if r <= 0x20 || unicode.IsSpace(r) {
		return false
	}
	switch r {
	case '\'', '"', '!', '#', '$', '%', '&', '(', ')', '*', '+', ',', '.', ':', ';',
		'<', '=', '>', '?', '@', '^', '{', '|', '}', '~', '[', ']', '\\', '`':
		return false
	}
	if r >= 0x2000 && r <= 0x206F {
		return false
	}
	if r >= 0x2E00 && r <= 0x2E7F {
		return false
	}
	return true
}

// isTagFirstRune reports whether r is allowed as the first character of a tag.
// Digits and '/' are not allowed at the start.
func isTagFirstRune(r rune) bool {
	return isTagRune(r) && !unicode.IsDigit(r) && r != '/'
}

func parseTags(line string, lineNum int) []linkOccur {
	// Skip heading lines (lines starting with # ).
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "# ") || trimmed == "#" {
		return nil
	}

	var out []linkOccur
	runes := []rune(line)
	n := len(runes)

	for i := 0; i < n; i++ {
		if runes[i] != '#' {
			continue
		}
		// '#' must be at start of line or preceded by a space character.
		if i > 0 && !unicode.IsSpace(runes[i-1]) {
			continue
		}
		// Read tag body.
		start := i + 1
		if start >= n || !isTagFirstRune(runes[start]) {
			continue
		}
		end := start + 1
		for end < n && isTagRune(runes[end]) {
			end++
		}
		// Trim trailing slashes.
		for end > start && runes[end-1] == '/' {
			end--
		}
		if end <= start {
			continue
		}
		tagName := string(runes[start:end])
		// Expand nested tags: #a/b/c → #a, #a/b, #a/b/c
		// Filter out empty segments (from "//") before expansion.
		rawParts := strings.Split(tagName, "/")
		parts := rawParts[:0]
		for _, p := range rawParts {
			if p != "" {
				parts = append(parts, p)
			}
		}
		for j := range parts {
			prefix := strings.Join(parts[:j+1], "/")
			out = append(out, linkOccur{
				target:     "#" + prefix,
				isBasename: false,
				isRelative: false,
				linkType:   LinkTypeTag,
				rawLink:    "#" + prefix,
				subpath:    "",
				lineStart:  lineNum,
				lineEnd:    lineNum,
			})
		}
		// Advance past the tag.
		i = end - 1
	}
	return out
}

func splitAlias(input string) string {
	if idx := strings.Index(input, "|"); idx != -1 {
		return input[:idx]
	}
	return input
}

// wikilinkParts contains the structural pieces of a raw wikilink.
// alias retains the leading "|" so callers can preserve an explicitly empty
// alias (for example, [[Target|]]) when reconstructing their own syntax.
type wikilinkParts struct {
	target  string
	subpath string
	alias   string
}

// splitWikilinkParts strips wikilink wrappers and splits its target, subpath,
// and alias. It intentionally does not apply any path or extension policy.
func splitWikilinkParts(rawLink string) wikilinkParts {
	inner := strings.TrimSuffix(strings.TrimPrefix(rawLink, "[["), "]]")
	withoutAlias := splitAlias(inner)
	target, subpath := extractSubpath(withoutAlias)
	return wikilinkParts{
		target:  target,
		subpath: subpath,
		alias:   inner[len(withoutAlias):],
	}
}

// extractSubpath splits "target#subpath" into (target, "#subpath").
// Returns (input, "") if no subpath.
func extractSubpath(input string) (string, string) {
	if idx := strings.Index(input, "#"); idx != -1 {
		return input[:idx], input[idx:]
	}
	return input, ""
}

func normalizeBasename(input string) string {
	lower := strings.ToLower(input)
	if strings.HasSuffix(lower, ".md") && len(input) >= 3 {
		return input[:len(input)-3]
	}
	return input
}

func isBasenameLink(target string) bool {
	if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") || strings.HasPrefix(target, "/") {
		return false
	}
	return !strings.Contains(target, "/")
}

func isRelativePath(target string) bool {
	return strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../")
}

func isURL(target string) bool {
	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "ftp:") {
		return true
	}

	// A hierarchical URI has a valid scheme followed by ://. Keep opaque
	// schemes other than the explicitly supported mailto and ftp internal so
	// colon-containing note names retain their existing interpretation.
	schemeEnd := strings.Index(lower, "://")
	if schemeEnd <= 0 || !isURIScheme(lower[:schemeEnd]) {
		return false
	}
	return true
}

func isURIScheme(s string) bool {
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && ((r >= '0' && r <= '9') || r == '+' || r == '-' || r == '.')) {
			continue
		}
		return false
	}
	return true
}
