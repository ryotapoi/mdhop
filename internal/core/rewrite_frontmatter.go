package core

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type frontmatterRewriteOccurrence struct {
	linkOccur
	start     int
	end       int
	scalar    *yaml.Node
	supported bool
}

type frontmatterSourceReplacement struct {
	start int
	end   int
	new   string
}

type frontmatterScalarReplacement struct {
	start int
	end   int
	new   string
}

// rewriteFrontmatterCandidate returns a validated, in-memory candidate for a
// set of quoted-frontmatter rewrites. It deliberately has no caller in this
// change: applying the candidate belongs to the later mutation change.
func rewriteFrontmatterCandidate(content []byte, rewrites []rewriteEntry) ([]byte, error) {
	frontmatterRewrites := make([]rewriteEntry, 0, len(rewrites))
	for _, rewrite := range rewrites {
		if rewrite.linkType == LinkTypeFrontmatterWikilink {
			frontmatterRewrites = append(frontmatterRewrites, rewrite)
		}
	}
	if len(frontmatterRewrites) == 0 {
		return append([]byte(nil), content...), nil
	}

	lines := strings.Split(string(content), "\n")
	fmEnd := frontmatterEnd(lines)
	if fmEnd <= 0 {
		return nil, fmt.Errorf("frontmatter wikilink rewrite has no parseable frontmatter source")
	}

	originalDoc, err := parseFrontmatterDocument(lines[:fmEnd+1])
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter source: %w", err)
	}
	occurrences, err := frontmatterRewriteOccurrences(lines, originalDoc)
	if err != nil {
		return nil, err
	}

	used := make([]bool, len(occurrences))
	replacements := make([]frontmatterSourceReplacement, 0, len(frontmatterRewrites))
	scalarReplacements := make(map[*yaml.Node][]frontmatterScalarReplacement)
	for _, rewrite := range frontmatterRewrites {
		if links := parseWikiLinks(rewrite.newRawLink, rewrite.lineStart); len(links) != 1 || links[0].rawLink != rewrite.newRawLink {
			return nil, fmt.Errorf("frontmatter wikilink %q has invalid planned replacement %q", rewrite.rawLink, rewrite.newRawLink)
		}

		found := -1
		for i, occurrence := range occurrences {
			if !used[i] && occurrence.lineStart == rewrite.lineStart && occurrence.rawLink == rewrite.rawLink {
				found = i
				break
			}
		}
		if found < 0 {
			return nil, fmt.Errorf("frontmatter wikilink %q has no matching extracted source occurrence on line %d", rewrite.rawLink, rewrite.lineStart)
		}

		occurrence := occurrences[found]
		if !occurrence.supported {
			return nil, fmt.Errorf("frontmatter quoted scalar at line %d cannot prove decoded/source correspondence", rewrite.lineStart)
		}
		used[found] = true
		valueOffset := occurrence.start - scalarContentStart(lines, occurrence.scalar)
		scalarReplacements[occurrence.scalar] = append(scalarReplacements[occurrence.scalar], frontmatterScalarReplacement{
			start: valueOffset,
			end:   valueOffset + len(rewrite.rawLink),
			new:   rewrite.newRawLink,
		})
		replacements = append(replacements, frontmatterSourceReplacement{start: occurrence.start, end: occurrence.end, new: rewrite.newRawLink})
	}
	expectedValues := make(map[*yaml.Node]string, len(scalarReplacements))
	for scalar, planned := range scalarReplacements {
		expectedValues[scalar] = applyScalarReplacements(scalar.Value, planned)
	}

	candidate := applyFrontmatterSourceReplacements(string(content), replacements)
	candidateLines := strings.Split(candidate, "\n")
	if candidateEnd := frontmatterEnd(candidateLines); candidateEnd != fmEnd {
		return nil, fmt.Errorf("frontmatter wikilink rewrite changes frontmatter boundaries")
	}
	candidateDoc, err := parseFrontmatterDocument(candidateLines[:fmEnd+1])
	if err != nil {
		return nil, fmt.Errorf("frontmatter wikilink rewrite produces invalid YAML: %w", err)
	}
	if !sameFrontmatterMeaning(originalDoc, candidateDoc, expectedValues) {
		return nil, fmt.Errorf("frontmatter wikilink rewrite changes YAML interpretation")
	}
	if !sameFrontmatterRewriteOccurrences(frontmatterWikilinkOccurrences(candidate), expectedFrontmatterOccurrences(occurrences, frontmatterRewrites)) {
		return nil, fmt.Errorf("frontmatter wikilink rewrite does not match the planned occurrence sequence")
	}
	return []byte(candidate), nil
}

func parseFrontmatterDocument(lines []string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:len(lines)-1], "\n")), &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter is not a YAML mapping")
	}
	return &doc, nil
}

func frontmatterRewriteOccurrences(lines []string, doc *yaml.Node) ([]frontmatterRewriteOccurrence, error) {
	var occurrences []frontmatterRewriteOccurrence
	for i := 0; i < len(doc.Content[0].Content)-1; i += 2 {
		key, value := doc.Content[0].Content[i], doc.Content[0].Content[i+1]
		if key.Value != "tags" {
			var err error
			occurrences, err = appendRewriteOccurrences(lines, value, occurrences)
			if err != nil {
				return nil, err
			}
		}
	}
	return occurrences, nil
}

func appendRewriteOccurrences(lines []string, value *yaml.Node, out []frontmatterRewriteOccurrence) ([]frontmatterRewriteOccurrence, error) {
	if value == nil || value.Tag == "!!null" {
		return out, nil
	}
	if value.Kind == yaml.SequenceNode {
		for _, item := range value.Content {
			var err error
			out, err = appendRewriteOccurrences(lines, item, out)
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	if value.Kind != yaml.ScalarNode || (value.Style != yaml.DoubleQuotedStyle && value.Style != yaml.SingleQuotedStyle) {
		return out, nil
	}
	spans := wikiLinkSpans(value.Value)
	if len(spans) == 0 {
		return out, nil
	}

	contentStart, contentEnd, err := scalarSourceRange(lines, value)
	supported := err == nil && sourceAt(lines, contentStart, contentEnd) == value.Value
	if !supported {
		contentStart = -1
	}
	for _, span := range spans {
		link := parseWikiLinks(span.raw, value.Line+1)[0]
		link.linkType = LinkTypeFrontmatterWikilink
		out = append(out, frontmatterRewriteOccurrence{
			linkOccur: link,
			start:     contentStart + span.start,
			end:       contentStart + span.end,
			scalar:    value,
			supported: supported,
		})
	}
	return out, nil
}

func scalarSourceRange(lines []string, value *yaml.Node) (int, int, error) {
	if value.Line < 1 || value.Line >= len(lines) {
		return 0, 0, fmt.Errorf("frontmatter quoted scalar has invalid source line %d", value.Line+1)
	}
	line := lines[value.Line] // YAML line 1 is file line 2; Node.Column is UTF-8 character based.
	startInLine, ok := utf8ColumnOffset(line, value.Column)
	if !ok || startInLine >= len(line) {
		return 0, 0, fmt.Errorf("frontmatter quoted scalar at line %d has invalid source column", value.Line+1)
	}
	quote := line[startInLine]
	if (value.Style == yaml.DoubleQuotedStyle && quote != '"') || (value.Style == yaml.SingleQuotedStyle && quote != '\'') {
		return 0, 0, fmt.Errorf("frontmatter quoted scalar at line %d does not start at its YAML source coordinate", value.Line+1)
	}
	endInLine := strings.IndexByte(line[startInLine+1:], quote)
	if endInLine < 0 {
		return 0, 0, fmt.Errorf("frontmatter quoted scalar at line %d is not a single-line literal", value.Line+1)
	}
	start := fileOffset(lines, value.Line) + startInLine + 1
	return start, start + endInLine, nil
}

func scalarContentStart(lines []string, value *yaml.Node) int {
	start, _, _ := scalarSourceRange(lines, value)
	return start
}

func utf8ColumnOffset(line string, column int) (int, bool) {
	if column < 1 {
		return 0, false
	}
	for offset, current := 0, 1; ; current++ {
		if current == column {
			return offset, true
		}
		if offset == len(line) {
			return 0, false
		}
		_, size := utf8.DecodeRuneInString(line[offset:])
		offset += size
	}
}

func fileOffset(lines []string, line int) int {
	offset := 0
	for i := 0; i < line; i++ {
		offset += len(lines[i]) + 1
	}
	return offset
}

func sourceAt(lines []string, start, end int) string {
	return strings.Join(lines, "\n")[start:end]
}

func applyFrontmatterSourceReplacements(content string, replacements []frontmatterSourceReplacement) string {
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for i := 1; i < len(replacements); i++ {
		if replacements[i-1].start < replacements[i].end {
			panic("overlapping frontmatter source replacements")
		}
	}
	for _, replacement := range replacements {
		content = content[:replacement.start] + replacement.new + content[replacement.end:]
	}
	return content
}

func applyScalarReplacements(value string, replacements []frontmatterScalarReplacement) string {
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for i := 1; i < len(replacements); i++ {
		if replacements[i-1].start < replacements[i].end {
			panic("overlapping frontmatter scalar replacements")
		}
	}
	for _, replacement := range replacements {
		value = value[:replacement.start] + replacement.new + value[replacement.end:]
	}
	return value
}

func frontmatterWikilinkOccurrences(content string) []linkOccur {
	var out []linkOccur
	for _, occurrence := range parseLinks(content).Links {
		if occurrence.linkType == LinkTypeFrontmatterWikilink {
			out = append(out, occurrence)
		}
	}
	return out
}

func expectedFrontmatterOccurrences(original []frontmatterRewriteOccurrence, rewrites []rewriteEntry) []linkOccur {
	result := make([]linkOccur, len(original))
	for i, occurrence := range original {
		result[i] = occurrence.linkOccur
	}
	used := make([]bool, len(original))
	for _, rewrite := range rewrites {
		for i, occurrence := range original {
			if !used[i] && occurrence.lineStart == rewrite.lineStart && occurrence.rawLink == rewrite.rawLink {
				updated := parseWikiLinks(rewrite.newRawLink, rewrite.lineStart)[0]
				updated.linkType = LinkTypeFrontmatterWikilink
				result[i] = updated
				used[i] = true
				break
			}
		}
	}
	return result
}

func sameFrontmatterRewriteOccurrences(got, want []linkOccur) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func sameFrontmatterMeaning(original, candidate *yaml.Node, expected map[*yaml.Node]string) bool {
	var same func(*yaml.Node, *yaml.Node) bool
	same = func(left, right *yaml.Node) bool {
		if left.Kind != right.Kind || left.Tag != right.Tag || left.Style != right.Style || len(left.Content) != len(right.Content) {
			return false
		}
		want := left.Value
		if replacement, ok := expected[left]; ok {
			want = replacement
		}
		if right.Value != want {
			return false
		}
		for i := range left.Content {
			if !same(left.Content[i], right.Content[i]) {
				return false
			}
		}
		return true
	}
	return same(original, candidate)
}
