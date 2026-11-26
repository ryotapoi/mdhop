package core

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// LinkPosition is a 1-based physical line and Unicode code point column.
// Lines include frontmatter; a CRLF counts as one line break.
type LinkPosition struct {
	Line   int
	Column int
}

// LinkLocation is the source range of a raw link. End is exclusive.
type LinkLocation struct {
	Start LinkPosition
	End   LinkPosition
}

type locationEdge struct {
	contentSource
	rawLink   string
	linkType  LinkType
	lineStart int
}

// queryFirstLinkEdges returns the indexed edges on the first source line that
// links to target. The edge order within a line is not column order, so the
// caller picks the leftmost one from the file.
func queryFirstLinkEdges(db dbExecer, sourceID, targetID int64, key string) ([]locationEdge, error) {
	q := `SELECT n.path,n.mtime,e.raw_link,e.link_type,COALESCE(e.line_start,0) FROM edges e
 JOIN nodes n ON n.id=e.source_id WHERE e.source_id=? AND e.target_id=?`
	args := []any{sourceID, targetID}
	if key != "" {
		q += " AND e.frontmatter_key=?"
		args = append(args, key)
	}
	q += " ORDER BY COALESCE(e.line_start,0),e.id"
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var edges []locationEdge
	for rows.Next() {
		var edge locationEdge
		if err := rows.Scan(&edge.path, &edge.mtime, &edge.rawLink, &edge.linkType, &edge.lineStart); err != nil {
			return nil, err
		}
		if len(edges) > 0 && edge.lineStart != edges[0].lineStart {
			break
		}
		edges = append(edges, edge)
	}
	return edges, rows.Err()
}

type locationParagraph struct {
	start      int
	raw, clean string
}

type locationFile struct {
	paragraphs  []locationParagraph
	definitions map[string]string
}

// linkLocator reads each source file at most once per query. A nil cached
// file means its positions cannot be determined.
type linkLocator struct {
	diskPaths *vaultDiskPathResolver
	files     map[string]*locationFile
}

func newLinkLocator(diskPaths *vaultDiskPathResolver) *linkLocator {
	return &linkLocator{diskPaths: diskPaths, files: make(map[string]*locationFile)}
}

// locate returns the raw link and location of the leftmost edge on the first
// indexed line, falling back to the first indexed raw link without a location.
func (l *linkLocator) locate(edges []locationEdge) (string, *LinkLocation, error) {
	if len(edges) == 0 {
		return "", nil, nil
	}
	first := edges[0]
	switch first.linkType {
	case LinkTypeFrontmatter, LinkTypeFrontmatterWikilink, LinkTypeFrontmatterPath:
		return first.rawLink, nil, nil
	}
	if first.lineStart <= 0 {
		return first.rawLink, nil, nil
	}
	file, err := l.load(first.contentSource)
	if err != nil || file == nil {
		return first.rawLink, nil, err
	}
	raw := first.rawLink
	var best *LinkLocation
	bestOffset := -1
	for _, edge := range edges {
		link, loc, offset := file.find(edge)
		if loc != nil && (best == nil || offset < bestOffset) {
			raw, best, bestOffset = link, loc, offset
		}
	}
	return raw, best, nil
}

func (l *linkLocator) load(source contentSource) (*locationFile, error) {
	if file, ok := l.files[source.path]; ok {
		return file, nil
	}
	file, err := readLocationFile(l.diskPaths, source)
	if err != nil {
		return nil, err
	}
	l.files[source.path] = file
	return file, nil
}

// readLocationFile returns nil without error when the file is missing or its
// mtime differs from the index, using the same criterion as checkStale.
func readLocationFile(diskPaths *vaultDiskPathResolver, source contentSource) (*locationFile, error) {
	fullPath, err := diskPaths.existingPath(source.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return nil, nil
		}
		return nil, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return nil, nil
		}
		return nil, err
	}
	if info.ModTime().Unix() != source.mtime {
		return nil, nil
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return nil, nil
		}
		return nil, err
	}
	// Split like parseLinks so line numbers match the index.
	lines := strings.Split(string(content), "\n")
	fmEnd := frontmatterEnd(lines)
	file := &locationFile{definitions: bodyReferenceDefinitions(lines, fmEnd)}
	walkBodyParagraphs(lines, fmEnd, func(lineNum int, raw, clean string) {
		file.paragraphs = append(file.paragraphs, locationParagraph{start: lineNum, raw: raw, clean: clean})
	})
	return file, nil
}

// find returns the raw link, location, and paragraph byte offset of the
// leftmost parsed occurrence matching the edge on its indexed line.
func (f *locationFile) find(edge locationEdge) (string, *LinkLocation, int) {
	for _, p := range f.paragraphs {
		last := p.start + strings.Count(p.raw, "\n")
		if edge.lineStart < p.start || edge.lineStart > last {
			continue
		}
		if span, ok := f.findSpan(p, edge); ok {
			raw := edge.rawLink
			if edge.linkType == LinkTypeWikilink {
				if original := p.raw[span.start:span.end]; original != raw {
					// Keep the returned link independent of the source paragraph.
					raw = strings.Clone(original)
				}
			}
			return raw, &LinkLocation{
				Start: paragraphPosition(p, span.start),
				End:   paragraphPosition(p, span.end),
			}, span.start
		}
		break
	}
	return "", nil, 0
}

func (f *locationFile) findSpan(p locationParagraph, edge locationEdge) (bodyLinkSpan, bool) {
	switch edge.linkType {
	case LinkTypeWikilink:
		// Wikilinks are indexed from code-masked physical lines.
		offset, lineNum := 0, p.start
		for line := range strings.SplitSeq(p.clean, "\n") {
			if lineNum == edge.lineStart {
				for _, span := range wikiLinkSpans(line) {
					if span.raw == edge.rawLink {
						return bodyLinkSpan{offset + span.start, offset + span.end}, true
					}
				}
				break
			}
			offset += len(line) + 1
			lineNum++
		}
	case LinkTypeMarkdown, LinkTypeMarkdownReference:
		clean := maskParagraphWikiLinks(p.clean)
		boundaries := scanMarkdownBoundaries(clean, p.start)
		for _, span := range markdownSpans(p.raw, clean, f.definitions, boundaries) {
			if boundaries.line[span.start] != edge.lineStart || span.typ != edge.linkType || p.raw[span.start:span.end] != edge.rawLink {
				continue
			}
			if _, ok := markdownDestinationOccur(span.destination, edge.rawLink, span.typ, edge.lineStart); ok {
				return bodyLinkSpan{span.start, span.end}, true
			}
		}
	case LinkTypeTag:
		for _, span := range f.tagSpans(p, edge.rawLink) {
			if p.start+strings.Count(p.raw[:span.start], "\n") == edge.lineStart {
				return span, true
			}
		}
	}
	return bodyLinkSpan{}, false
}

// tagSpans applies the parser's tag text masking and tag boundaries, so a
// "#tag" inside a wikilink, Markdown link, or longer tag is not selected.
// A nested raw tag such as "#a" may match the leading part of "#a/b".
func (f *locationFile) tagSpans(p locationParagraph, raw string) []bodyLinkSpan {
	_, tagText := parseBodyMarkdown(p.raw, maskParagraphWikiLinks(p.clean), p.start, f.definitions)
	var spans []bodyLinkSpan
	offset := 0
	for _, line := range strings.Split(tagText, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "# ") && trimmed != "#" {
			for i := 0; i+len(raw) <= len(line); i++ {
				if !strings.HasPrefix(line[i:], raw) {
					continue
				}
				if before, _ := utf8.DecodeLastRuneInString(line[:i]); i > 0 && !unicode.IsSpace(before) {
					continue
				}
				if after, _ := utf8.DecodeRuneInString(line[i+len(raw):]); i+len(raw) < len(line) && after != '/' && isTagRune(after) {
					continue
				}
				spans = append(spans, bodyLinkSpan{offset + i, offset + i + len(raw)})
			}
		}
		offset += len(line) + 1
	}
	return spans
}

func paragraphPosition(p locationParagraph, offset int) LinkPosition {
	lineOffset := strings.LastIndexByte(p.raw[:offset], '\n') + 1
	return LinkPosition{
		Line:   p.start + strings.Count(p.raw[:offset], "\n"),
		Column: utf8.RuneCountInString(p.raw[lineOffset:offset]) + 1,
	}
}
