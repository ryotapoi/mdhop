package core

import "strings"

// bodyTableLines records table context without changing source spans. Only the
// body lines already admitted by the shared fence/frontmatter scanner qualify.
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
	if len(heading) < len(trim) && (heading == "" || strings.HasPrefix(heading, " ") || strings.HasPrefix(heading, "\t")) || strings.HasPrefix(trim, ">") || strings.HasPrefix(trim, "<") {
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
