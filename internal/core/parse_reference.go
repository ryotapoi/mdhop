package core

import "strings"

func normalizeReferenceLabel(label string) string {
	return strings.ToLower(strings.Join(strings.FieldsFunc(label, func(r rune) bool { return r == ' ' || r == '\t' }), " "))
}

// referenceDefinition recognizes only independent, single-line definitions.
// Definition-shaped lines are consumed even when their destination is unsupported.
func referenceDefinition(line string) (label, destination string, definition bool) {
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

// referenceBracketEnd consumes nested brackets but rejects them as supported syntax.
func referenceBracketEnd(line string, start int) (int, bool) {
	depth, valid := 1, true
	for i := start + 1; i < len(line); i++ {
		switch line[i] {
		case '\\':
			valid = false
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
	occ := linkOccur{target: normalizeBasename(target), isBasename: isBasenameLink(target), isRelative: isRelativePath(target), linkType: typ, rawLink: raw, subpath: subpath, lineStart: line, lineEnd: line}
	if typ == LinkTypeMarkdownReference {
		occ.referenceTarget = destination
	}
	return occ, true
}

// parseBodyMarkdown scans disjoint bracket spans, so undefined full references
// cannot fall back to a shortcut and inline links cannot consume earlier references.
func parseBodyMarkdown(line string, lineNum int, definitions map[string]string) ([]linkOccur, string, int) {
	pending := 0
	masked := []byte(line)
	var out []linkOccur
	for i := 0; i < len(line); {
		if line[i] != '[' {
			i++
			continue
		}
		start := i
		end, valid := referenceBracketEnd(line, start)
		if end < 0 {
			pending = unmatchedBracketDepth(line[start:])
			break
		}
		i = end + 1
		if start > 0 && line[start-1] == '\\' {
			valid = false
		}
		if !valid && (i >= len(line) || line[i] != '(') {
			// Consume the following reference label as part of this unsupported span.
			if i < len(line) && line[i] == '[' {
				if close, _ := referenceBracketEnd(line, i); close >= 0 {
					i = close + 1
				} else {
					pending = unmatchedBracketDepth(line[i:])
					break
				}
			} else if i < len(line) && line[i] == '(' {
				if close := markdownDestinationEnd(line, i+1); close >= 0 {
					i = close + 1
				} else {
					break
				}
			}
			continue
		}
		label := line[start+1 : end]
		destination, typ, used := "", LinkTypeMarkdownReference, false
		if i < len(line) && line[i] == '(' {
			close := markdownDestinationEnd(line, i+1)
			if close < 0 {
				break
			}
			destination, typ, used = strings.TrimSpace(line[i+1:close]), LinkTypeMarkdown, true
			i = close + 1
		} else {
			if i < len(line) && line[i] == '[' {
				close, ok := referenceBracketEnd(line, i)
				if close < 0 {
					pending = unmatchedBracketDepth(line[i:])
					break
				}
				if close > i+1 {
					label = line[i+1 : close]
				}
				i = close + 1
				if !ok {
					continue
				}
			}
			destination, used = definitions[normalizeReferenceLabel(label)]
		}
		if used {
			if occ, ok := markdownDestinationOccur(destination, line[start:i], typ, lineNum); ok {
				out = append(out, occ)
			}
			for j := start; j < i; j++ {
				masked[j] = ' '
			}
		}
	}
	return out, string(masked), pending
}

func unmatchedBracketDepth(line string) int {
	depth := 0
	for i := 0; i < len(line); i++ {
		if line[i] == '[' {
			depth++
		}
		if line[i] == ']' {
			depth--
		}
	}
	return depth
}

// maskReferenceContinuation rejects multiline labels as a whole instead of
// indexing the trailing label of an unsupported full reference as a shortcut.
func maskReferenceContinuation(line string, pending *int) string {
	if *pending == 0 {
		return line
	}
	masked := []byte(line)
	i := 0
	for i < len(line) && *pending > 0 {
		if line[i] == '[' {
			*pending++
		}
		if line[i] == ']' {
			*pending--
		}
		masked[i] = ' '
		i++
	}
	if *pending == 0 && i < len(line) && line[i] == '[' {
		close, _ := referenceBracketEnd(line, i)
		if close < 0 {
			*pending = unmatchedBracketDepth(line[i:])
			close = len(line) - 1
		}
		for ; i <= close; i++ {
			masked[i] = ' '
		}
	}
	return string(masked)
}

// referenceContinuationCloses looks ahead within the same body paragraph.
// This distinguishes a closed unsupported multiline construct from an unmatched
// prose bracket without mistaking its balanced inner spans for standalone links.
func referenceContinuationCloses(lines []string, nextLine, depth int) bool {
	for _, raw := range lines[nextLine:] {
		if strings.TrimSpace(raw) == "" {
			return false
		}
		if _, fence := parseFenceOpening(strings.TrimSpace(raw)); fence {
			return false
		}
		if _, _, definition := referenceDefinition(raw); definition {
			return false
		}
		clean := maskWikiLinks(stripInlineCode(raw))
		for i := 0; i < len(clean); i++ {
			if clean[i] == '[' {
				depth++
			}
			if clean[i] == ']' {
				depth--
			}
			if depth == 0 {
				return true
			}
		}
	}
	return false
}
