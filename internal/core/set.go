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
}

// SetResult reports the outcome of the set operation.
type SetResult struct {
	File     string
	Key      string
	Value    string
	Created  bool
	Warnings []string
}

// Set rewrites one scalar frontmatter key in a registered note and refreshes
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
	fullPath, err := diskPaths.existingPath(file)
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
	newContent, created, err := rewriteFrontmatterValue(original, opts.Key, opts.Value)
	if err != nil {
		return nil, err
	}

	backup := setBackup{
		content: original,
		perm:    info.Mode().Perm(),
		modTime: info.ModTime(),
	}
	if err := writeFilePreservePerm(fullPath, newContent, backup.perm); err != nil {
		return nil, err
	}

	updateResult, err := Update(vaultPath, UpdateOptions{Files: []string{file}})
	if err != nil {
		return nil, wrapRollbackFailures(err, restoreSetBackup(fullPath, file, backup))
	}

	return &SetResult{
		File:     file,
		Key:      opts.Key,
		Value:    opts.Value,
		Created:  created,
		Warnings: updateResult.Warnings,
	}, nil
}

type setBackup struct {
	content []byte
	perm    os.FileMode
	modTime time.Time
}

func restoreSetBackup(fullPath, path string, backup setBackup) []rollbackFailure {
	var failures []rollbackFailure
	if err := rollbackWriteFile(fullPath, backup.content, backup.perm); err != nil {
		failures = append(failures, rollbackFailure{action: "restore", path: path, err: err})
	}
	if err := os.Chtimes(fullPath, backup.modTime, backup.modTime); err != nil {
		failures = append(failures, rollbackFailure{action: "restore modification time for", path: path, err: err})
	}
	return failures
}

func rewriteFrontmatterValue(content []byte, key, value string) ([]byte, bool, error) {
	text := string(content)
	lines := strings.Split(text, "\n")
	end := frontmatterEnd(lines)
	if end < 0 {
		newLines := append([]string{"---", key + ": " + formatSetYAMLValue(value), "---"}, lines...)
		return []byte(strings.Join(newLines, "\n")), true, nil
	}

	yamlContent := strings.Join(lines[1:end], "\n")
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		return nil, false, err
	}
	newLine := key + ": " + formatSetYAMLValue(value)
	// Empty frontmatter (e.g. "---\n---\n") unmarshals to a zero-value Node
	// (doc.Kind == 0, not yaml.DocumentNode) rather than an empty mapping.
	// Treat it the same as "key not present": append a new mapping entry.
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind == 0 ||
		(doc.Content[0].Kind == yaml.ScalarNode && doc.Content[0].Tag == "!!null") {
		lines = append(lines[:end], append([]string{newLine}, lines[end:]...)...)
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
		if valNode.Kind == yaml.SequenceNode {
			return nil, false, fmt.Errorf("frontmatter key %q has sequence value; set supports scalar values only", key)
		}
		if valNode.Kind != yaml.ScalarNode || valNode.Style == yaml.LiteralStyle || valNode.Style == yaml.FoldedStyle {
			return nil, false, fmt.Errorf("frontmatter key %q has unsupported value; set supports single-line scalar values only", key)
		}
		if frontmatterValueLineCount(mapping, matchIndex, end, lines) > 1 {
			return nil, false, fmt.Errorf("frontmatter key %q has multi-line value; set supports single-line scalar values only", key)
		}
		// yaml.Node.Line is 1-based against the YAML body. The opening "---"
		// is file line 1, so yaml line 1 = file line 2 and file line = Line + 1.
		fileLine := valNode.Line + 1
		if fileLine < 1 || fileLine > len(lines) {
			return nil, false, fmt.Errorf("frontmatter key %q line is out of range", key)
		}
		lines[fileLine-1] = newLine + yamlCommentSuffix(lines[fileLine-1])
		return []byte(strings.Join(lines, "\n")), false, nil
	}

	lines = append(lines[:end], append([]string{newLine}, lines[end:]...)...)
	return []byte(strings.Join(lines, "\n")), true, nil
}

func frontmatterValueLineCount(mapping *yaml.Node, keyIndex, frontmatterEndLine int, lines []string) int {
	valNode := mapping.Content[keyIndex+1]
	if nextKeyIndex := keyIndex + 2; nextKeyIndex < len(mapping.Content) {
		return mapping.Content[nextKeyIndex].Line - valNode.Line
	}
	return lastFrontmatterValueLineCount(valNode.Line, frontmatterEndLine, lines)
}

func lastFrontmatterValueLineCount(valueYAMLLine, frontmatterEndLine int, lines []string) int {
	if valueYAMLLine < 0 || valueYAMLLine >= len(lines) || !frontmatterLineParsesAsSingleMappingEntry(lines[valueYAMLLine]) {
		return 2
	}
	count := 1
	for lineIndex := valueYAMLLine + 1; lineIndex < frontmatterEndLine && lineIndex < len(lines); lineIndex++ {
		if strings.TrimSpace(stripYAMLComment(lines[lineIndex])) == "" {
			continue
		}
		count++
	}
	return count
}

func frontmatterLineParsesAsSingleMappingEntry(line string) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(line+"\n"), &doc); err != nil {
		return false
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return false
	}
	mapping := doc.Content[0]
	return mapping.Kind == yaml.MappingNode && len(mapping.Content) == 2
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
			if !inSingle {
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
