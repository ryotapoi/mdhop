package core

import (
	"slices"
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
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		key := mapping.Content[i]
		val := mapping.Content[i+1]
		if key.Value == "tags" {
			fr.links, fr.meta = parseFrontmatterTags(val, offset, fr.links, fr.meta)
			continue
		}
		fr.meta = collectMeta(key.Value, val, offset, fr.meta)
		fr.links = append(fr.links, collectFrontmatterWikilinks(val, offset)...)
	}
	return fr
}

// collectFrontmatterWikilinks extracts wikilinks from quoted YAML scalar and
// list-item values only, matching Obsidian property link semantics. Bare
// `key: [[Note]]` and bare list items `- [[Note]]` are YAML nested sequences,
// not quoted scalars, and are ignored.
func collectFrontmatterWikilinks(val *yaml.Node, offset int) []linkOccur {
	if val == nil || val.Tag == "!!null" {
		return nil
	}
	switch val.Kind {
	case yaml.ScalarNode:
		return wikilinksFromQuotedScalar(val, offset)
	case yaml.SequenceNode:
		var out []linkOccur
		for _, item := range val.Content {
			out = append(out, collectFrontmatterWikilinks(item, offset)...)
		}
		return out
	default:
		return nil
	}
}

func wikilinksFromQuotedScalar(val *yaml.Node, offset int) []linkOccur {
	if val.Kind != yaml.ScalarNode || val.Value == "" {
		return nil
	}
	if val.Style != yaml.DoubleQuotedStyle && val.Style != yaml.SingleQuotedStyle {
		return nil
	}
	fileLine := val.Line + offset
	var out []linkOccur
	for _, l := range parseWikiLinks(val.Value, fileLine) {
		l.linkType = LinkTypeFrontmatterWikilink
		out = append(out, l)
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

// frontmatterPathLinks converts raw path values of the configured link keys
// into linkOccur entries with link type "frontmatter_path". Path classification
// follows markdown link semantics: "./" / "../" prefixes are note-relative,
// values containing "/" are vault-relative paths, and bare names resolve by
// basename. Skipped values: empty, URLs (containing "://"), and wikilinks
// (already parsed as frontmatter_wikilink). The whole value is treated as the
// path; "#" fragments are not split off.
func frontmatterPathLinks(meta []FrontmatterEntry, linkKeys []string) []linkOccur {
	if len(linkKeys) == 0 {
		return nil
	}
	var out []linkOccur
	for _, m := range meta {
		if !slices.Contains(linkKeys, m.Key) {
			continue
		}
		if occ, ok := frontmatterPathOccur(m.Value, m.Line); ok {
			out = append(out, occ)
		}
	}
	return out
}

// frontmatterPathOccur classifies a single link-key value as a
// frontmatter_path linkOccur. ok is false for skipped values: empty, URLs
// (containing "://"), and values containing "[[" (well-formed wikilinks are
// already parsed as frontmatter_wikilink; broken ones are not paths either).
func frontmatterPathOccur(value string, line int) (linkOccur, bool) {
	v := strings.TrimSpace(value)
	if v == "" || strings.Contains(v, "://") || strings.Contains(v, "[[") {
		return linkOccur{}, false
	}
	// normalizeBasename only strips a ".md" suffix, never a leading "./" or
	// "../", so isRelative classification (computed from v) stays valid for
	// the normalized target.
	return linkOccur{
		target:     normalizeBasename(v),
		isBasename: isBasenameLink(v),
		isRelative: isRelativePath(v),
		linkType:   LinkTypeFrontmatterPath,
		rawLink:    value,
		lineStart:  line,
		lineEnd:    line,
	}, true
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
