package core

import (
	"regexp"
	"strings"
)

// Type 7 HTML blocks require a complete tag alone on the line. Other HTML
// starts below need only a prefix, so inline tags and autolinks remain cells.
var tableHTMLTagLine = regexp.MustCompile(`^(?:<[A-Za-z][A-Za-z0-9-]*(?:[ \t\r\n\f]+[A-Za-z_:][A-Za-z0-9_.:-]*(?:[ \t\r\n\f]*=[ \t\r\n\f]*(?:[^ \t\r\n\f"'=<>\x60]+|'[^']*'|"[^"]*"))?)*[ \t\r\n\f]*/?>|</[A-Za-z][A-Za-z0-9-]*[ \t\r\n\f]*>)[ \t\r\n\f]*$`)

// bodyTableLines records table context without changing source spans. Only the
// body lines already admitted by the shared fence/frontmatter scanner qualify.
// The table grammar is based on https://github.github.com/gfm/#tables-extension-;
// this contextual scanner is deliberately not a complete CommonMark block parser.
func bodyTableLines(lines []string, fmEnd int) map[int]bool {
	type bodyLine struct {
		number int
		raw    string
	}
	var body []bodyLine
	walkBodyLines(lines, fmEnd, func(n int, raw, _ string) { body = append(body, bodyLine{n, raw}) })
	tables := make(map[int]bool)
	active := false
	for i, line := range body {
		if i == 0 || line.number != body[i-1].number+1 {
			active = false
		}
		if active {
			if tableBlockBoundary(line.raw) {
				active = false
			} else {
				tables[line.number] = true
				continue
			}
		}
		if i == 0 || line.number != body[i-1].number+1 || tableBlockBoundary(body[i-1].raw) {
			continue
		}
		header, hasPipe := tableCells(body[i-1].raw)
		delimiter, delimiterPipe := tableCells(line.raw)
		if !hasPipe && !delimiterPipe || len(header) != len(delimiter) {
			continue
		}
		valid := true
		for _, cell := range delimiter {
			cell = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(cell), ":"), ":")
			if cell == "" || strings.Trim(cell, "-") != "" {
				valid = false
				break
			}
		}
		if valid {
			tables[body[i-1].number], tables[line.number], active = true, true, true
		}
	}
	return tables
}

func tableCells(raw string) ([]string, bool) {
	line := strings.TrimSpace(raw)
	var cells []string
	start, slashes := 0, 0
	hasPipe := false
	for i := 0; i < len(line); i++ {
		if line[i] == '|' && slashes%2 == 0 {
			cells = append(cells, line[start:i])
			start = i + 1
			hasPipe = true
		}
		if line[i] == '\\' {
			slashes++
		} else {
			slashes = 0
		}
	}
	cells = append(cells, line[start:])
	if hasPipe && cells[0] == "" {
		cells = cells[1:]
	}
	if hasPipe && len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells, hasPipe
}

func tableBlockBoundary(raw string) bool {
	trim := strings.TrimSpace(raw)
	if trim == "" || strings.HasPrefix(raw, "    ") || strings.HasPrefix(raw, "\t") {
		return true
	}
	if _, _, definition := referenceDefinition(raw); definition {
		return true
	}
	heading := strings.TrimLeft(trim, "#")
	if len(heading) < len(trim) && (heading == "" || strings.HasPrefix(heading, " ") || strings.HasPrefix(heading, "\t")) || strings.HasPrefix(trim, ">") || tableHTMLBlockStart(trim) {
		return true
	}
	if strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "+ ") || strings.HasPrefix(trim, "* ") {
		return true
	}
	digits := 0
	for digits < len(trim) && trim[digits] >= '0' && trim[digits] <= '9' {
		digits++
	}
	if digits > 0 && digits+1 < len(trim) && (trim[digits] == '.' || trim[digits] == ')') && trim[digits+1] == ' ' {
		return true
	}
	if !strings.Contains(trim, "|") && (strings.Trim(trim, "- ") == "" || strings.Trim(trim, "* ") == "" || strings.Trim(trim, "_ ") == "") {
		return true
	}
	return false
}

// tableHTMLBlockStart recognizes GFM HTML block starts only. It does not parse
// HTML contents or change the shared body scanner's treatment of HTML lines.
func tableHTMLBlockStart(line string) bool {
	if len(line) < 2 || line[0] != '<' {
		return false
	}
	if strings.HasPrefix(line, "<!--") || strings.HasPrefix(line, "<?") || strings.HasPrefix(line, "<![CDATA[") || len(line) > 2 && line[1] == '!' && line[2] >= 'A' && line[2] <= 'Z' {
		return true
	}
	start := 1
	closing := line[start] == '/'
	if closing {
		start++
	}
	end := start
	for end < len(line) && (line[end] >= 'A' && line[end] <= 'Z' || line[end] >= 'a' && line[end] <= 'z' || line[end] >= '0' && line[end] <= '9' || line[end] == '-') {
		end++
	}
	if end == start {
		return false
	}
	spaceOrEnd := end == len(line) || strings.ContainsRune(" \t\r\n\f", rune(line[end]))
	if !spaceOrEnd && line[end] != '>' && !strings.HasPrefix(line[end:], "/>") {
		return false
	}
	// Only known block tags need case folding, bounded by the longest name.
	if end-start <= len("blockquote") {
		switch strings.ToLower(line[start:end]) {
		case "script", "pre", "style":
			if !closing {
				return spaceOrEnd || line[end] == '>'
			}
		case "address", "article", "aside", "base", "basefont", "blockquote", "body", "caption", "center", "col", "colgroup", "dd", "details", "dialog", "dir", "div", "dl", "dt", "fieldset", "figcaption", "figure", "footer", "form", "frame", "frameset", "h1", "h2", "h3", "h4", "h5", "h6", "head", "header", "hr", "html", "iframe", "legend", "li", "link", "main", "menu", "menuitem", "nav", "noframes", "ol", "optgroup", "option", "p", "param", "section", "source", "summary", "table", "tbody", "td", "tfoot", "th", "thead", "title", "tr", "track", "ul":
			return true
		}
	}
	return tableHTMLTagLine.MatchString(line)
}
