package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SetOptions controls a single frontmatter key/value write.
type SetOptions struct {
	File  string // vault-relative path
	Key   string
	Value string
	List  []string // non-nil writes a YAML sequence
}

// SetResult reports the outcome of the set operation.
type SetResult struct {
	File     string
	Key      string
	Value    string
	List     []string
	Created  bool
	Warnings []string
}

var setUpdate = Update
var setChtimes = os.Chtimes

// Set rewrites one frontmatter key in a registered note and refreshes
// the index entry for that note.
func Set(vaultPath string, opts SetOptions) (*SetResult, error) {
	file := NormalizePath(opts.File)
	if filepath.IsAbs(opts.File) || pathEscapesVault(file) {
		return nil, fmt.Errorf("%w: %s", ErrFileNotFound, opts.File)
	}

	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rm, err := buildMapsFromDB(db)
	if err != nil {
		return nil, err
	}
	nodeID, ok := rm.pathToID[file]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrFileNotRegistered, opts.File)
	}

	var dbMtime int64
	if err := db.QueryRow("SELECT mtime FROM nodes WHERE id = ?", nodeID).Scan(&dbMtime); err != nil {
		return nil, err
	}

	diskPaths := newVaultDiskPathResolver(vaultPath)
	fullPath, err := diskPaths.writablePath(file)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrFileNotFound, file)
	}
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	if info.ModTime().Unix() != dbMtime {
		return nil, fmt.Errorf("%w: %s", ErrSourceStale, file)
	}

	original, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	newContent, created, err := rewriteFrontmatterValue(original, opts.Key, opts.Value, opts.List)
	if err != nil {
		return nil, err
	}

	backup := setBackup{
		content: original,
		perm:    info.Mode().Perm(),
		modTime: info.ModTime(),
	}
	if err := writeFilePreservePerm(fullPath, newContent, backup.perm); err != nil {
		return nil, wrapRollbackFailures(err, restoreSetBackup(vaultPath, fullPath, file, backup))
	}

	updateResult, err := setUpdate(vaultPath, UpdateOptions{Files: []string{file}})
	if err != nil {
		return nil, wrapRollbackFailures(err, restoreSetBackup(vaultPath, fullPath, file, backup))
	}

	return &SetResult{
		File:     file,
		Key:      opts.Key,
		Value:    opts.Value,
		List:     opts.List,
		Created:  created,
		Warnings: updateResult.Warnings,
	}, nil
}

type setBackup struct {
	content []byte
	perm    os.FileMode
	modTime time.Time
}

func restoreSetBackup(vaultPath, fullPath, path string, backup setBackup) []rollbackFailure {
	if err := validateVaultWritePath(vaultPath, fullPath); err != nil {
		return []rollbackFailure{{action: "restore", path: path, err: err}}
	}
	var failures []rollbackFailure
	if err := rollbackWriteFile(fullPath, backup.content, backup.perm); err != nil {
		failures = append(failures, rollbackFailure{action: "restore", path: path, err: err})
	}
	if err := setChtimes(fullPath, backup.modTime, backup.modTime); err != nil {
		failures = append(failures, rollbackFailure{action: "restore modification time for", path: path, err: err})
	}
	return failures
}

func rewriteFrontmatterValue(content []byte, key, value string, list []string) ([]byte, bool, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	end := frontmatterEnd(lines)
	newLines := formatSetYAMLLines(key, value, list)
	if end < 0 {
		newLines := append([]string{"---"}, newLines...)
		newLines = append(newLines, "---")
		newLines = append(newLines, lines...)
		return []byte(strings.Join(newLines, "\n")), true, nil
	}

	yamlContent := strings.Join(lines[1:end], "\n")
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		return nil, false, err
	}
	// Empty frontmatter (e.g. "---\n---\n") unmarshals to a zero-value Node
	// (doc.Kind == 0, not yaml.DocumentNode) rather than an empty mapping.
	// Treat it the same as "key not present": append a new mapping entry.
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind == 0 ||
		(doc.Content[0].Kind == yaml.ScalarNode && doc.Content[0].Tag == "!!null") {
		lines = append(lines[:end], append(newLines, lines[end:]...)...)
		return []byte(strings.Join(lines, "\n")), true, nil
	}
	if doc.Content[0].Kind != yaml.MappingNode {
		return nil, false, fmt.Errorf("frontmatter must be a mapping")
	}
	mapping := doc.Content[0]
	matchIndex := -1
	matchCount := 0
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Value != key {
			continue
		}
		matchIndex = i
		matchCount++
	}
	if matchCount > 1 {
		return nil, false, fmt.Errorf("frontmatter has duplicate key %q", key)
	}
	if matchIndex >= 0 {
		valNode := mapping.Content[matchIndex+1]
		if list != nil {
			if valNode.Kind != yaml.ScalarNode && valNode.Kind != yaml.SequenceNode {
				return nil, false, fmt.Errorf("frontmatter key %q has unsupported value; set supports scalar or sequence values for list writes", key)
			}
			if valNode.Kind == yaml.ScalarNode {
				if valNode.Style == yaml.LiteralStyle || valNode.Style == yaml.FoldedStyle {
					return nil, false, fmt.Errorf("frontmatter key %q has unsupported value; set supports single-line scalar values only", key)
				}
				if frontmatterValueLineCount(mapping, matchIndex, end, lines) > 1 {
					return nil, false, fmt.Errorf("frontmatter key %q has multi-line value; set supports single-line scalar values only", key)
				}
				fileLine := valNode.Line + 1
				if fileLine < 1 || fileLine > len(lines) {
					return nil, false, fmt.Errorf("frontmatter key %q line is out of range", key)
				}
			}
			if mapping.Style&yaml.FlowStyle != 0 {
				return rewriteSetFlowMapping(lines, end, &doc, matchIndex, newLines)
			}
			start := mapping.Content[matchIndex].Line
			stop := end
			if nextKeyIndex := matchIndex + 2; nextKeyIndex < len(mapping.Content) {
				stop = mapping.Content[nextKeyIndex].Line
			}
			stop = setListValueEnd(valNode, stop, lines)
			if start < 1 || start > stop || stop > len(lines) {
				return nil, false, fmt.Errorf("frontmatter key %q lines are out of range", key)
			}
			newLines[0] += yamlCommentSuffix(lines[start])
			lines = append(lines[:start], append(newLines, lines[stop:]...)...)
			return []byte(strings.Join(lines, "\n")), false, nil
		}
		if valNode.Kind == yaml.SequenceNode {
			return nil, false, fmt.Errorf("frontmatter key %q has sequence value; set supports scalar values only", key)
		}
		if valNode.Kind != yaml.ScalarNode || valNode.Style == yaml.LiteralStyle || valNode.Style == yaml.FoldedStyle {
			return nil, false, fmt.Errorf("frontmatter key %q has unsupported value; set supports single-line scalar values only", key)
		}
		if frontmatterValueLineCount(mapping, matchIndex, end, lines) > 1 {
			return nil, false, fmt.Errorf("frontmatter key %q has multi-line value; set supports single-line scalar values only", key)
		}
		if mapping.Style&yaml.FlowStyle != 0 {
			return rewriteSetFlowMapping(lines, end, &doc, matchIndex, newLines)
		}
		// yaml.Node.Line is 1-based against the YAML body. The opening "---"
		// is file line 1, so yaml line 1 = file line 2 and file line = Line + 1.
		fileLine := valNode.Line + 1
		if fileLine < 1 || fileLine > len(lines) {
			return nil, false, fmt.Errorf("frontmatter key %q line is out of range", key)
		}
		lines[fileLine-1] = newLines[0] + yamlCommentSuffix(lines[fileLine-1])
		return []byte(strings.Join(lines, "\n")), false, nil
	}

	if mapping.Style&yaml.FlowStyle != 0 {
		return rewriteSetFlowMapping(lines, end, &doc, -1, newLines)
	}
	lines = append(lines[:end], append(newLines, lines[end:]...)...)
	return []byte(strings.Join(lines, "\n")), true, nil
}

// Flow mappings may put several keys on one physical line, so replacing that
// line would remove unrelated entries. Re-encode only this frontmatter form.
func rewriteSetFlowMapping(lines []string, end int, doc *yaml.Node, matchIndex int, newLines []string) ([]byte, bool, error) {
	var replacement yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(newLines, "\n")), &replacement); err != nil {
		return nil, false, err
	}
	entry := replacement.Content[0].Content
	mapping := doc.Content[0]
	if matchIndex < 0 {
		mapping.Content = append(mapping.Content, entry...)
	} else {
		oldValue := mapping.Content[matchIndex+1]
		entry[1].HeadComment = oldValue.HeadComment
		entry[1].LineComment = oldValue.LineComment
		entry[1].FootComment = oldValue.FootComment
		mapping.Content[matchIndex+1] = entry[1]
	}
	yamlContent, err := yaml.Marshal(doc)
	if err != nil {
		return nil, false, err
	}
	rewritten := append([]string{lines[0]}, strings.Split(strings.TrimSuffix(string(yamlContent), "\n"), "\n")...)
	rewritten = append(rewritten, lines[end:]...)
	return []byte(strings.Join(rewritten, "\n")), matchIndex < 0, nil
}

func formatSetYAMLLines(key, value string, list []string) []string {
	key = formatSetYAMLValue(key)
	if list == nil {
		return []string{key + ": " + formatSetYAMLValue(value)}
	}
	if len(list) == 0 {
		return []string{key + ": []"}
	}
	lines := []string{key + ":"}
	for _, item := range list {
		lines = append(lines, "  - "+strconv.Quote(item))
	}
	return lines
}

// setListValueEnd ends the replacement immediately after the parsed value,
// preserving comments and formatting before the next key or frontmatter end.
func setListValueEnd(value *yaml.Node, stop int, lines []string) int {
	lastLine := value.Line
	for _, child := range value.Content {
		lastLine = max(lastLine, setListValueEnd(child, 0, lines))
	}
	lastLine = max(lastLine, setScalarValueEnd(value, lines))
	if value.Kind == yaml.SequenceNode && value.Style == yaml.FlowStyle {
		lastLine = max(lastLine, setFlowSequenceEnd(value.Line, lines))
	}
	if stop == 0 {
		return lastLine
	}
	return min(stop, lastLine+1)
}

// setScalarValueEnd finds the last physical line of block and quoted scalar
// sequence items. yaml.Node records their starting line, but not their end.
func setScalarValueEnd(value *yaml.Node, lines []string) int {
	if value.Kind != yaml.ScalarNode || value.Line < 0 || value.Line >= len(lines) {
		return value.Line
	}
	switch value.Style {
	case yaml.LiteralStyle, yaml.FoldedStyle:
		return setBlockScalarEnd(value.Line, lines)
	case yaml.DoubleQuotedStyle:
		return setQuotedScalarEnd(value.Line, value.Column, lines, '"')
	case yaml.SingleQuotedStyle:
		return setQuotedScalarEnd(value.Line, value.Column, lines, '\'')
	default:
		if isBlockSequenceItem(lines[value.Line]) {
			return setPlainSequenceItemEnd(value.Line, lines)
		}
		return value.Line
	}
}

func isBlockSequenceItem(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return trimmed == "-" || strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "-\t")
}

func setPlainSequenceItemEnd(start int, lines []string) int {
	indent := leadingIndent(lines[start])
	end := start
	for line := start + 1; line < len(lines); line++ {
		if strings.TrimSpace(lines[line]) == "" {
			continue
		}
		if leadingIndent(lines[line]) <= indent {
			break
		}
		end = line
	}
	return end
}

func setFlowSequenceEnd(start int, lines []string) int {
	depth := 0
	inSingle := false
	inDouble := false
	for line := start; line < len(lines); line++ {
	scanLine:
		for column := 0; column < len(lines[line]); column++ {
			ch := lines[line][column]
			if inDouble {
				if ch == '"' && !yamlQuoteEscaped(lines[line], column) {
					inDouble = false
				}
				continue
			}
			if inSingle {
				if ch == '\'' {
					if column+1 < len(lines[line]) && lines[line][column+1] == '\'' {
						column++
						continue
					}
					inSingle = false
				}
				continue
			}
			switch ch {
			case '#':
				if column == 0 || lines[line][column-1] == ' ' || lines[line][column-1] == '\t' {
					break scanLine
				}
			case '"':
				inDouble = true
			case '\'':
				inSingle = true
			case '[':
				depth++
			case ']':
				if depth > 0 {
					depth--
					if depth == 0 {
						return line
					}
				}
			}
		}
	}
	return start
}

func setBlockScalarEnd(start int, lines []string) int {
	indent := leadingIndent(lines[start])
	end := start
	for line := start + 1; line < len(lines); line++ {
		if strings.TrimSpace(lines[line]) == "" {
			continue
		}
		if leadingIndent(lines[line]) <= indent {
			break
		}
		end = line
	}
	return end
}

func setQuotedScalarEnd(start, column int, lines []string, quote byte) int {
	opened := false
	for line := start; line < len(lines); line++ {
		startColumn := 0
		if line == start && column > 1 {
			startColumn = len(string([]rune(lines[line])[:column-1]))
		}
		for column := startColumn; column < len(lines[line]); column++ {
			if lines[line][column] != quote {
				continue
			}
			if quote == '"' && yamlQuoteEscaped(lines[line], column) {
				continue
			}
			if quote == '\'' && opened && column+1 < len(lines[line]) && lines[line][column+1] == quote {
				column++
				continue
			}
			if opened {
				return line
			}
			opened = true
		}
	}
	return start
}

func yamlQuoteEscaped(line string, quoteIndex int) bool {
	backslashes := 0
	for index := quoteIndex - 1; index >= 0 && line[index] == '\\'; index-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func leadingIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func frontmatterValueLineCount(mapping *yaml.Node, keyIndex, frontmatterEndLine int, lines []string) int {
	valNode := mapping.Content[keyIndex+1]
	if setScalarValueEnd(valNode, lines) > valNode.Line || mapping.Content[keyIndex].Line != valNode.Line {
		return 2
	}
	stop := frontmatterEndLine
	stopColumn := 0
	if nextKeyIndex := keyIndex + 2; nextKeyIndex < len(mapping.Content) {
		stop = mapping.Content[nextKeyIndex].Line
		if mapping.Style&yaml.FlowStyle != 0 && stop > valNode.Line {
			stopColumn = mapping.Content[nextKeyIndex].Column
			stop++
		}
	}
	count := 1
	for lineIndex := valNode.Line + 1; lineIndex < stop && lineIndex < len(lines); lineIndex++ {
		line := lines[lineIndex]
		if stopColumn > 0 && lineIndex == stop-1 {
			line = string([]rune(line)[:stopColumn-1])
		}
		line = strings.TrimSpace(stripYAMLComment(line))
		if mapping.Style&yaml.FlowStyle != 0 {
			line = strings.TrimSpace(strings.Trim(line, ",}"))
		}
		if line == "" {
			continue
		}
		count++
	}
	return count
}

func formatSetYAMLValue(value string) string {
	if needsSetYAMLQuotes(value) {
		return strconv.Quote(value)
	}
	return value
}

// yamlPlainScalarIndicators are characters that, per the YAML plain-scalar
// grammar, cannot start a plain (unquoted) scalar without changing its
// meaning (block sequence entry, anchor, tag, flow collection, etc.).
// Without this check, a value like "- leading dash" is written verbatim and
// reparses as a YAML block sequence entry, corrupting the frontmatter block
// for every key that follows it.
const yamlPlainScalarIndicators = "-?:,[]{}#&*!|>'\"%@`"

func needsSetYAMLQuotes(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return true
	}
	lower := strings.ToLower(value)
	switch lower {
	case "true", "false", "null", "~", ".nan", ".inf", "-.inf", "+.inf":
		return true
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return true
	}
	if strings.ContainsAny(value, "\n\r\t") {
		return true
	}
	if strings.Contains(value, ": ") || strings.HasSuffix(value, ":") {
		return true
	}
	if strings.Contains(value, " #") {
		return true
	}
	if strings.ContainsRune(yamlPlainScalarIndicators, rune(value[0])) {
		return true
	}
	return false
}

func yamlCommentSuffix(line string) string {
	inSingle := false
	inDouble := false
	prev := byte(' ')
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch ch {
		case '"':
			if !inSingle && !yamlQuoteEscaped(line, i) {
				inDouble = !inDouble
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '#':
			if !inSingle && !inDouble && (prev == ' ' || prev == '\t') {
				if i == 0 {
					return line[i:]
				}
				return line[i-1:]
			}
		}
		prev = ch
	}
	return ""
}
