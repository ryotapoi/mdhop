package core

import "strings"

func normalizeReferenceLabel(label string) string {
	return strings.ToLower(strings.Join(strings.FieldsFunc(label, func(r rune) bool { return r == ' ' || r == '\t' || r == '\r' || r == '\n' }), " "))
}

// referenceDefinition recognizes only independent, single-line definitions.
// Definition-shaped lines are consumed even when their destination is unsupported.
func referenceDefinition(line string) (label, destination string, definition bool) {
	line = strings.TrimSuffix(line, "\r")
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	if i > 3 || i == len(line) || line[i] != '[' {
		return
	}
	end, valid := referenceBracketEnd(line, i)
	if end < 0 || end+1 >= len(line) || line[end+1] != ':' {
		return
	}
	definition = true
	if !valid {
		return
	}
	label = normalizeReferenceLabel(line[i+1 : end])
	if label == "" {
		return
	}
	rest := strings.TrimLeft(line[end+2:], " \t")
	if rest == "" {
		return
	}
	var tail string
	if rest[0] == '<' {
		close := -1
		for j := 1; j < len(rest); j++ {
			if rest[j] == '\\' && j+1 < len(rest) && asciiPunctuation(rest[j+1]) {
				j++
				continue
			}
			if rest[j] == '<' || rest[j] == '\r' || rest[j] == '\n' {
				return label, "", true
			}
			if rest[j] == '>' {
				close = j
				break
			}
		}
		if close < 0 {
			return label, "", true
		}
		destination, tail = rest[1:close], rest[close+1:]
	} else {
		depth, j := 0, 0
		for ; j < len(rest) && rest[j] != ' ' && rest[j] != '\t'; j++ {
			if rest[j] == '\\' && j+1 < len(rest) && asciiPunctuation(rest[j+1]) {
				j++
				continue
			}
			switch rest[j] {
			case '(':
				depth++
			case ')':
				depth--
				if depth < 0 {
					return label, "", true
				}
			case '<', '>':
				return label, "", true
			}
		}
		if depth != 0 {
			return label, "", true
		}
		destination, tail = rest[:j], rest[j:]
	}
	if strings.ContainsAny(destination, "\r\n") {
		return label, "", true
	}
	if tail != "" {
		if tail[0] != ' ' && tail[0] != '\t' {
			return label, "", true
		}
		tail = strings.TrimSpace(tail)
		if tail != "" {
			if len(tail) < 2 {
				return label, "", true
			}
			closing := tail[0]
			if closing == '(' {
				closing = ')'
			} else if closing != '\'' && closing != '"' {
				return label, "", true
			}
			if tail[len(tail)-1] != closing || strings.ContainsRune(tail[1:len(tail)-1], rune(closing)) {
				return label, "", true
			}
		}
	}
	return
}

// markdownEscapeAt identifies the CommonMark ASCII punctuation escape pair.
func markdownEscapeAt(line string, i int) bool {
	return line[i] == '\\' && i+1 < len(line) && asciiPunctuation(line[i+1])
}

// referenceBracketEnd shares inline bracket boundaries, while valid keeps
// escaped and nested reference labels outside the supported reference grammar.
func referenceBracketEnd(line string, start int) (int, bool) {
	depth, valid := 1, true
	for i := start + 1; i < len(line); i++ {
		switch line[i] {
		case '\\':
			valid = false
			if markdownEscapeAt(line, i) {
				i++
			}
		case '[':
			depth++
			valid = false
		case ']':
			depth--
			if depth == 0 {
				return i, valid
			}
		}
	}
	return -1, false
}

func markdownDestinationOccur(destination, raw string, typ LinkType, line int) (linkOccur, bool) {
	target, subpath, external := markdownDestination(destination)
	if target == "" || external {
		return linkOccur{}, false
	}
	occ := linkOccur{target: normalizeBasename(target), isBasename: false, isRelative: !strings.HasPrefix(target, "/"), linkType: typ, rawLink: raw, subpath: subpath, lineStart: line, lineEnd: line}
	if typ == LinkTypeMarkdownReference {
		occ.referenceTarget = destination
	}
	return occ, true
}

// markdownBoundaries pairs brackets and single-line destination parentheses
// once. Unmatched openings have no span and cannot hide independent links.
type markdownBoundaries struct {
	close []int
	valid []bool
	line  []int
}

func scanMarkdownBoundaries(text string, lineNum int) markdownBoundaries {
	b := markdownBoundaries{make([]int, len(text)), make([]bool, len(text)), make([]int, len(text))}
	for i := range b.close {
		b.close[i] = -1
		b.valid[i] = true
	}
	var brackets, parens []int
	for i := 0; i < len(text); i++ {
		b.line[i] = lineNum
		if text[i] == '\n' {
			lineNum++
			parens = nil
		}
		if text[i] == '\\' {
			if len(brackets) > 0 {
				b.valid[brackets[len(brackets)-1]] = false
			}
			if markdownEscapeAt(text, i) {
				i++
				b.line[i] = lineNum
			}
			continue
		}
		switch text[i] {
		case '[':
			if len(brackets) > 0 {
				b.valid[brackets[len(brackets)-1]] = false
			}
			brackets = append(brackets, i)
		case ']':
			if len(brackets) > 0 {
				start := brackets[len(brackets)-1]
				brackets = brackets[:len(brackets)-1]
				b.close[start] = i
			}
		case '(':
			parens = append(parens, i)
		case ')':
			if len(parens) > 0 {
				start := parens[len(parens)-1]
				parens = parens[:len(parens)-1]
				b.close[start] = i
			}
		}
	}
	return b
}

type markdownSpan struct {
	start, end  int
	destination string
	typ         LinkType
}

// markdownSpans scans disjoint constructs using the precomputed boundaries.
// Unsupported nested/escaped references still own their entire closed span.
func markdownSpans(raw, clean string, definitions map[string]string, b markdownBoundaries) []markdownSpan {
	var spans []markdownSpan
	for i := 0; i < len(clean); {
		if markdownEscapeAt(clean, i) {
			i += 2
			continue
		}
		if clean[i] != '[' || b.close[i] < 0 {
			i++
			continue
		}
		start, end := i, b.close[i]
		i = end + 1
		label := raw[start+1 : end]
		destination, typ, used := "", LinkTypeMarkdownReference, false
		if i < len(clean) && clean[i] == '(' {
			close := b.close[i]
			if close < 0 {
				continue
			}
			destination, typ, used = strings.TrimSpace(clean[i+1:close]), LinkTypeMarkdown, true
			i = close + 1
		} else {
			valid := b.valid[start]
			if i < len(clean) && clean[i] == '[' {
				close := b.close[i]
				if close < 0 {
					continue
				}
				valid = valid && b.valid[i]
				if close > i+1 {
					label = raw[i+1 : close]
				}
				i = close + 1
			}
			if valid {
				destination, used = definitions[normalizeReferenceLabel(label)]
			}
		}
		if used {
			spans = append(spans, markdownSpan{start, i, destination, typ})
		}
	}
	return spans
}

func parseBodyMarkdown(raw, clean string, lineNum int, definitions map[string]string) ([]linkOccur, string) {
	b := scanMarkdownBoundaries(clean, lineNum)
	masked := []byte(clean)
	var out []linkOccur
	for _, span := range markdownSpans(raw, clean, definitions, b) {
		if occ, ok := markdownDestinationOccur(span.destination, raw[span.start:span.end], span.typ, b.line[span.start]); ok {
			occ.lineEnd = b.line[span.end-1]
			out = append(out, occ)
		}
		for i := span.start; i < span.end; i++ {
			if masked[i] != '\n' && masked[i] != '\r' {
				masked[i] = ' '
			}
		}
	}
	return out, string(masked)
}
