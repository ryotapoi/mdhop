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
	linkType   string // "wikilink", "markdown", "tag", "frontmatter"
	rawLink    string
	subpath    string
	lineStart  int
	lineEnd    int
}

// parseLinks parses all links (wikilinks, markdown links, tags, frontmatter tags) from content.
func parseLinks(content string) []linkOccur {
	var out []linkOccur
	lines := strings.Split(content, "\n")

	// Parse frontmatter first.
	fmEnd := frontmatterEnd(lines)
	if fmEnd > 0 {
		out = append(out, parseFrontmatter(lines[:fmEnd+1])...)
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
	return out
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

// parseFrontmatter extracts tags from YAML frontmatter.
// lines should include the opening and closing "---".
func parseFrontmatter(lines []string) []linkOccur {
	if len(lines) < 3 {
		return nil
	}
	// Extract YAML content between --- markers.
	yamlContent := strings.Join(lines[1:len(lines)-1], "\n")

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		return nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil
	}

	// frontmatter offset: line 1 in the file is the "---", yaml line 1 = file line 2.
	// So yaml Node.Line + offset = file line number.
	// lines[0] is "---" at file line 1. YAML content starts at file line 2.
	// yaml.Node.Line is 1-based relative to the yaml content.
	// File line = yaml.Node.Line + 1 (since yaml starts at file line 2, and yaml line 1 = file line 2).
	offset := 1 // lines[0] is "---"

	var out []linkOccur
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		key := mapping.Content[i]
		val := mapping.Content[i+1]
		if key.Value != "tags" {
			continue
		}
		switch val.Kind {
		case yaml.SequenceNode:
			for _, item := range val.Content {
				if item.Kind == yaml.ScalarNode && item.Value != "" {
					fileLine := item.Line + offset
					tagName := item.Value
					if !strings.HasPrefix(tagName, "#") {
						tagName = "#" + tagName
					}
					// Expand nested tags.
					parts := strings.Split(strings.TrimPrefix(tagName, "#"), "/")
					for j := range parts {
						prefix := "#" + strings.Join(parts[:j+1], "/")
						out = append(out, linkOccur{
							target:     prefix,
							isBasename: false,
							isRelative: false,
							linkType:   "frontmatter",
							rawLink:    prefix,
							subpath:    "",
							lineStart:  fileLine,
							lineEnd:    fileLine,
						})
					}
				}
			}
		case yaml.ScalarNode:
			// tags: single-value (comma-separated or single tag)
			if val.Value != "" {
				fileLine := val.Line + offset
				for _, tag := range strings.Split(val.Value, ",") {
					tag = strings.TrimSpace(tag)
					if tag == "" {
						continue
					}
					if !strings.HasPrefix(tag, "#") {
						tag = "#" + tag
					}
					parts := strings.Split(strings.TrimPrefix(tag, "#"), "/")
					for j := range parts {
						prefix := "#" + strings.Join(parts[:j+1], "/")
						out = append(out, linkOccur{
							target:     prefix,
							isBasename: false,
							isRelative: false,
							linkType:   "frontmatter",
							rawLink:    prefix,
							subpath:    "",
							lineStart:  fileLine,
							lineEnd:    fileLine,
						})
					}
				}
			}
		}
	}
	return out
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
