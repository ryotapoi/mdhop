package core

import (
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

type linkOccur struct {
	target     string
	isBasename bool
	isRelative bool
	linkType   string // "wikilink", "markdown", "tag", "frontmatter", "frontmatter_wikilink"
	rawLink    string
	subpath    string
	lineStart  int
	lineEnd    int
}

type FrontmatterEntry struct {
	Key   string
	Value string
	Line  int
}

type parseResult struct {
	Links []linkOccur
	Meta  []FrontmatterEntry
}

// parseLinks parses all links (wikilinks, markdown links, tags, frontmatter tags) from content.
func parseLinks(content string) parseResult {
	var out []linkOccur
	lines := strings.Split(content, "\n")

	// Parse frontmatter first.
	var result parseResult
	fmEnd := frontmatterEnd(lines)
	if fmEnd > 0 {
		links, meta := parseFrontmatter(lines[:fmEnd+1])
		out = append(out, links...)
		result.Meta = meta
	}

	inFence := false
	startLine := 0
	if fmEnd > 0 {
		startLine = fmEnd + 1
	}
	for i := startLine; i < len(lines); i++ {
		lineNum := i + 1 // 1-based
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		clean := stripInlineCode(lines[i])
		out = append(out, parseWikiLinks(clean, lineNum)...)
		out = append(out, parseMarkdownLinks(clean, lineNum)...)
		// Parse tags on a line with wikilinks/markdown links removed.
		tagLine := stripWikiLinks(stripMarkdownLinks(clean))
		out = append(out, parseTags(tagLine, lineNum)...)
	}
	result.Links = out
	return result
}

func stripInlineCode(line string) string {
	var out strings.Builder
	inCode := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '`' {
			inCode = !inCode
			continue
		}
		if !inCode {
			out.WriteByte(ch)
		}
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

// stripMarkdownLinks removes [text](url) from a line to avoid tag false positives.
func stripMarkdownLinks(line string) string {
	for {
		open := strings.Index(line, "[")
		if open == -1 {
			break
		}
		mid := strings.Index(line[open:], "](")
		if mid == -1 {
			break
		}
		mid = open + mid
		close := strings.Index(line[mid+2:], ")")
		if close == -1 {
			break
		}
		close = mid + 2 + close + 1
		line = line[:open] + line[close:]
	}
	return line
}

func parseWikiLinks(line string, lineNum int) []linkOccur {
	var out []linkOccur
	remaining := line
	for {
		start := strings.Index(remaining, "[[")
		if start == -1 {
			break
		}
		end := strings.Index(remaining[start+2:], "]]")
		if end == -1 {
			break
		}
		end = start + 2 + end
		inner := remaining[start+2 : end]
		rawLink := "[[" + inner + "]]"

		name := splitAlias(inner)
		target, subpath := extractSubpath(name)

		if target == "" && subpath != "" {
			// [[#Heading]] — self-link
			out = append(out, linkOccur{
				target:     "",
				isBasename: false,
				isRelative: false,
				linkType:   "wikilink",
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
				linkType:   "wikilink",
				rawLink:    rawLink,
				subpath:    subpath,
				lineStart:  lineNum,
				lineEnd:    lineNum,
			})
		}
		remaining = remaining[end+2:]
	}
	return out
}

func parseMarkdownLinks(line string, lineNum int) []linkOccur {
	var out []linkOccur
	remaining := line
	for {
		open := strings.Index(remaining, "[")
		if open == -1 {
			break
		}
		// Skip if this is actually a wikilink "[[".
		if open+1 < len(remaining) && remaining[open+1] == '[' {
			remaining = remaining[open+2:]
			continue
		}
		mid := strings.Index(remaining[open:], "](")
		if mid == -1 {
			break
		}
		mid = open + mid
		close := strings.Index(remaining[mid+2:], ")")
		if close == -1 {
			break
		}
		close = mid + 2 + close
		rawTarget := strings.TrimSpace(remaining[mid+2 : close])
		rawLink := remaining[open : close+1]

		target, subpath := extractSubpath(rawTarget)
		if target != "" && !isURL(rawTarget) {
			out = append(out, linkOccur{
				target:     normalizeBasename(target),
				isBasename: isBasenameLink(target),
				isRelative: isRelativePath(target),
				linkType:   "markdown",
				rawLink:    rawLink,
				subpath:    subpath,
				lineStart:  lineNum,
				lineEnd:    lineNum,
			})
		}
		remaining = remaining[close+1:]
	}
	return out
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
				linkType:   "tag",
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

// frontmatterEnd returns the line index of the closing "---" of frontmatter.
// Returns -1 if no valid frontmatter is found.
func frontmatterEnd(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return -1
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i
		}
	}
	return -1
}

// parseFrontmatter extracts tags and metadata from YAML frontmatter.
// lines should include the opening and closing "---".
func parseFrontmatter(lines []string) ([]linkOccur, []FrontmatterEntry) {
	if len(lines) < 3 {
		return nil, nil
	}
	// Extract YAML content between --- markers.
	yamlContent := strings.Join(lines[1:len(lines)-1], "\n")

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		return nil, nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, nil
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil, nil
	}

	// frontmatter offset: line 1 in the file is the "---", yaml line 1 = file line 2.
	// So yaml Node.Line + offset = file line number.
	// lines[0] is "---" at file line 1. YAML content starts at file line 2.
	// yaml.Node.Line is 1-based relative to the yaml content.
	// File line = yaml.Node.Line + 1 (since yaml starts at file line 2, and yaml line 1 = file line 2).
	offset := 1 // lines[0] is "---"

	var out []linkOccur
	var meta []FrontmatterEntry
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		key := mapping.Content[i]
		val := mapping.Content[i+1]
		if key.Value == "tags" {
			out, meta = parseFrontmatterTags(val, offset, out, meta)
			continue
		}
		meta = collectMeta(key.Value, val, offset, meta)
		// Extract wikilinks from the file-line range covering this key/val pair.
		// We rely on raw text scanning rather than yaml.Node interpretation to
		// handle bare [[note]] (parsed as nested flow seq), quoted "[[note]]"
		// (parsed as scalar), and array forms uniformly.
		startIdx, endIdx := frontmatterEntryRange(mapping.Content, i, len(lines))
		out = append(out, parseFrontmatterWikilinks(lines, startIdx, endIdx)...)
	}
	return out, meta
}

// frontmatterEntryRange returns the index range [startIdx, endIdx) within
// frontmatter `lines` that covers the key at mapping.Content[i] and its value.
// Indices are 0-based into `lines` (so `lines[0]` is the opening "---" and
// `lines[totalLines-1]` is the closing "---").
//
// yaml.Node.Line is 1-based against the yaml body which starts at lines[1],
// so lines[node.Line] is the file line for that node.
func frontmatterEntryRange(content []*yaml.Node, i, totalLines int) (int, int) {
	startIdx := content[i].Line
	endIdx := totalLines - 1 // exclude closing "---"
	if i+2 < len(content) {
		endIdx = content[i+2].Line
	}
	if startIdx < 1 {
		startIdx = 1
	}
	if endIdx > totalLines-1 {
		endIdx = totalLines - 1
	}
	return startIdx, endIdx
}

// parseFrontmatterWikilinks scans rawLines[startIdx:endIdx] for [[...]]
// occurrences and returns them as linkOccur entries with linkType
// "frontmatter_wikilink". Each occurrence's lineStart/lineEnd is the file line
// number (1-based, with rawLines[0] as file line 1).
//
// rawLines includes the opening and closing "---" of frontmatter. startIdx /
// endIdx are 0-based indices into rawLines.
func parseFrontmatterWikilinks(rawLines []string, startIdx, endIdx int) []linkOccur {
	var out []linkOccur
	for j := startIdx; j < endIdx; j++ {
		lineNum := j + 1 // 1-based file line
		line := rawLines[j]
		for _, l := range parseWikiLinks(line, lineNum) {
			l.linkType = "frontmatter_wikilink"
			out = append(out, l)
		}
	}
	return out
}

// parseFrontmatterTags handles the "tags" key, producing both linkOccur (with nested expansion)
// and FrontmatterEntry (normalized: # prefix removed, no nested expansion).
func parseFrontmatterTags(val *yaml.Node, offset int, out []linkOccur, meta []FrontmatterEntry) ([]linkOccur, []FrontmatterEntry) {
	if val.Tag == "!!null" {
		return out, meta
	}
	switch val.Kind {
	case yaml.SequenceNode:
		for _, item := range val.Content {
			if item.Kind != yaml.ScalarNode || item.Value == "" || item.Tag == "!!null" {
				continue
			}
			fileLine := item.Line + offset
			normalized := strings.TrimPrefix(item.Value, "#")
			meta = append(meta, FrontmatterEntry{Key: "tags", Value: normalized, Line: fileLine})
			out = expandFrontmatterTag(normalized, fileLine, out)
		}
	case yaml.ScalarNode:
		if val.Value != "" && val.Tag != "!!null" {
			fileLine := val.Line + offset
			for _, tag := range strings.Split(val.Value, ",") {
				tag = strings.TrimSpace(tag)
				if tag == "" {
					continue
				}
				normalized := strings.TrimPrefix(tag, "#")
				if normalized == "" {
					continue
				}
				meta = append(meta, FrontmatterEntry{Key: "tags", Value: normalized, Line: fileLine})
				out = expandFrontmatterTag(normalized, fileLine, out)
			}
		}
	}
	return out, meta
}

// expandFrontmatterTag expands a normalized tag name (without #) into nested linkOccur entries.
func expandFrontmatterTag(normalized string, fileLine int, out []linkOccur) []linkOccur {
	parts := strings.Split(normalized, "/")
	for j := range parts {
		prefix := "#" + strings.Join(parts[:j+1], "/")
		out = append(out, linkOccur{
			target:    prefix,
			linkType:  "frontmatter",
			rawLink:   prefix,
			lineStart: fileLine,
			lineEnd:   fileLine,
		})
	}
	return out
}

// collectMeta appends FrontmatterEntry items for non-tags keys.
func collectMeta(key string, val *yaml.Node, offset int, meta []FrontmatterEntry) []FrontmatterEntry {
	if val.Tag == "!!null" {
		return meta
	}
	switch val.Kind {
	case yaml.ScalarNode:
		if val.Value != "" && val.Tag != "!!null" {
			meta = append(meta, FrontmatterEntry{
				Key:   key,
				Value: val.Value,
				Line:  val.Line + offset,
			})
		}
	case yaml.SequenceNode:
		for _, item := range val.Content {
			if item.Kind == yaml.ScalarNode && item.Value != "" && item.Tag != "!!null" {
				meta = append(meta, FrontmatterEntry{
					Key:   key,
					Value: item.Value,
					Line:  item.Line + offset,
				})
			}
		}
	}
	return meta
}

func splitAlias(input string) string {
	if idx := strings.Index(input, "|"); idx != -1 {
		return input[:idx]
	}
	return input
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
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
}
