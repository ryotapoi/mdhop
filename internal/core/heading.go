package core

import (
	"strings"
	"unicode"
)

// collectHeadings extracts ATX heading texts (lines starting with 1-6 '#'
// followed by a space) from a Markdown document, skipping frontmatter and
// fenced code blocks. The returned slice preserves document order and holds
// the raw heading text (without the leading '#'s), trimmed. Inline-code
// stripping by walkBodyLines does not affect the leading '#' that marks a
// heading, so it is reused as the scan skeleton.
func collectHeadings(content string) []string {
	lines := strings.Split(content, "\n")
	fmEnd := frontmatterEnd(lines)
	var headings []string
	walkBodyLines(lines, fmEnd, func(_ int, clean string) {
		if h, ok := atxHeadingText(clean); ok {
			headings = append(headings, h)
		}
	})
	return headings
}

// atxHeadingText returns the heading text of an ATX heading line (e.g.
// "## My Heading" → "My Heading"). Returns ("", false) for non-heading lines.
func atxHeadingText(line string) (string, bool) {
	s := strings.TrimLeft(line, " ")
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return "", false
	}
	if n >= len(s) || s[n] != ' ' {
		return "", false
	}
	text := strings.TrimSpace(s[n+1:])
	if text == "" {
		return "", false
	}
	return text, true
}

// normalizeAnchor normalizes a heading or link fragment for anchor matching,
// following Obsidian's behavior: strip a leading '#', drop punctuation, and
// collapse whitespace. Case and accents are preserved (Obsidian-compatible).
// Returns ("", false) for block references (#^id), which are not heading anchors.
func normalizeAnchor(s string) (string, bool) {
	s = strings.TrimPrefix(s, "#")
	if strings.HasPrefix(s, "^") {
		return "", false // block reference, not a heading anchor
	}
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			// drop punctuation/symbols (Obsidian strips these from anchors)
		default:
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String()), true
}
