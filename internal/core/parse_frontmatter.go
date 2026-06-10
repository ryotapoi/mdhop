package core

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// FrontmatterEntry is a single key/value occurrence in YAML frontmatter.
type FrontmatterEntry struct {
	Key   string
	Value string
	Line  int
}

// frontmatterResult holds links and metadata extracted from YAML frontmatter.
type frontmatterResult struct {
	links []linkOccur
	meta  []FrontmatterEntry
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
func parseFrontmatter(lines []string) frontmatterResult {
	if len(lines) < 3 {
		return frontmatterResult{}
	}
	// Extract YAML content between --- markers.
	yamlContent := strings.Join(lines[1:len(lines)-1], "\n")

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		return frontmatterResult{}
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return frontmatterResult{}
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return frontmatterResult{}
	}

	// frontmatter offset: line 1 in the file is the "---", yaml line 1 = file line 2.
	// So yaml Node.Line + offset = file line number.
	// lines[0] is "---" at file line 1. YAML content starts at file line 2.
	// yaml.Node.Line is 1-based relative to the yaml content.
	// File line = yaml.Node.Line + 1 (since yaml starts at file line 2, and yaml line 1 = file line 2).
	offset := 1 // lines[0] is "---"

	var fr frontmatterResult
	blockScalarLines := collectBlockScalarLines(mapping.Content, lines)
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		key := mapping.Content[i]
		val := mapping.Content[i+1]
		if key.Value == "tags" {
			fr.links, fr.meta = parseFrontmatterTags(val, offset, fr.links, fr.meta)
			continue
		}
		fr.meta = collectMeta(key.Value, val, offset, fr.meta)
		// Extract wikilinks from the file-line range covering this key/val pair.
		// We rely on raw text scanning rather than yaml.Node interpretation to
		// handle bare [[note]] (parsed as nested flow seq), quoted "[[note]]"
		// (parsed as scalar), and array forms uniformly.
		startIdx, endIdx := frontmatterEntryRange(mapping.Content, i, len(lines))
		fr.links = append(fr.links, parseFrontmatterWikilinks(lines, startIdx, endIdx, blockScalarLines)...)
	}
	return fr
}

// collectBlockScalarLines returns the set of file line numbers (1-based) that
// fall inside a YAML block scalar value (`key: |` / `key: >`). Inside a block
// scalar, '#' is part of the value, not a comment, so callers must skip
// stripYAMLComment for these lines (ADR 0013).
//
// yaml.Node.Line is 1-based against the yaml body which starts at lines[1],
// so file line = node.Line + 1.
//
// A block scalar body line must be more deeply indented than the key
// (YAML 1.2 §8.1). Lines that fall back to the key's indent (or shallower)
// terminate the body, even if they precede the next mapping key (e.g. a
// top-level "# comment" line between two mapping entries).
func collectBlockScalarLines(content []*yaml.Node, lines []string) map[int]bool {
	out := map[int]bool{}
	totalLines := len(lines)
	for i := 0; i < len(content)-1; i += 2 {
		key := content[i]
		val := content[i+1]
		if val.Kind != yaml.ScalarNode {
			continue
		}
		if val.Style != yaml.LiteralStyle && val.Style != yaml.FoldedStyle {
			continue
		}
		// val.Line points at the line carrying the `|` or `>` indicator
		// (same line as the key). Body starts on the next file line.
		bodyStartFileLine := val.Line + 2
		// Hard upper bound: next key file line - 1, or last frontmatter
		// content line (exclude closing "---").
		bodyEndFileLine := totalLines - 1
		if i+2 < len(content) {
			bodyEndFileLine = content[i+2].Line
		}
		// Block scalar body indent must be deeper than the key column.
		keyIndent := key.Column - 1 // Column is 1-based
		for ln := bodyStartFileLine; ln <= bodyEndFileLine; ln++ {
			if ln < 1 || ln > totalLines {
				continue
			}
			line := lines[ln-1]
			if strings.TrimSpace(line) == "" {
				// Blank lines belong to the body (per YAML spec) but
				// carry no '#', so marking them is harmless either way.
				out[ln] = true
				continue
			}
			if leadingSpaceCount(line) <= keyIndent {
				// Dedented to key indent or shallower: body terminates here.
				break
			}
			out[ln] = true
		}
	}
	return out
}

// leadingSpaceCount returns the number of leading space characters in line.
// YAML block scalar indentation is measured in spaces only (tabs are not
// allowed for indentation), so a simple space count is sufficient.
func leadingSpaceCount(line string) int {
	for i := 0; i < len(line); i++ {
		if line[i] != ' ' {
			return i
		}
	}
	return len(line)
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
func parseFrontmatterWikilinks(rawLines []string, startIdx, endIdx int, blockScalarLines map[int]bool) []linkOccur {
	var out []linkOccur
	for j := startIdx; j < endIdx; j++ {
		lineNum := j + 1 // 1-based file line
		line := rawLines[j]
		if !blockScalarLines[lineNum] {
			line = stripYAMLComment(line)
		}
		for _, l := range parseWikiLinks(line, lineNum) {
			l.linkType = LinkTypeFrontmatterWikilink
			out = append(out, l)
		}
	}
	return out
}

// stripYAMLComment returns line with the YAML comment portion removed.
// A '#' starts a comment only when it is at the beginning of the line or
// preceded by whitespace, and only when it is not inside a single- or
// double-quoted scalar on the same line.
func stripYAMLComment(line string) string {
	inSingle := false
	inDouble := false
	prev := byte(' ') // treat line start as if preceded by whitespace
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch ch {
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '#':
			if !inSingle && !inDouble && (prev == ' ' || prev == '\t') {
				return line[:i]
			}
		}
		prev = ch
	}
	return line
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
			linkType:  LinkTypeFrontmatter,
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
