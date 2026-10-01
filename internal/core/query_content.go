package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readHead(vaultPath string, source contentSource, n int) ([]string, error) {
	fullPath, err := newVaultDiskPathResolver(vaultPath).existingPath(source.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, source.path)
		}
		return nil, err
	}
	if err := checkStale(fullPath, source.mtime); err != nil {
		return nil, err
	}

	lines, err := readFileLines(fullPath)
	if err != nil {
		return nil, err
	}

	// Skip frontmatter.
	fmEnd := frontmatterEnd(lines)
	start := 0
	if fmEnd > 0 {
		start = fmEnd + 1
	}

	// Skip leading blank lines.
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	end := start + n
	if end > len(lines) {
		end = len(lines)
	}

	return lines[start:end], nil
}

func readSnippets(vaultPath string, sources []snippetSource, contextLines int) ([]SnippetEntry, error) {
	// Cache file lines per source path.
	fileCache := make(map[string][]string)
	var snippets []SnippetEntry
	diskPaths := newVaultDiskPathResolver(vaultPath)

	for _, source := range sources {
		if _, ok := fileCache[source.path]; !ok {
			fullPath, err := diskPaths.existingPath(source.path)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("%w: %s", ErrFileNotFound, source.path)
				}
				return nil, err
			}
			if err := checkStale(fullPath, source.mtime); err != nil {
				return nil, err
			}
			lines, err := readFileLines(fullPath)
			if err != nil {
				return nil, err
			}
			fileCache[source.path] = lines
		}

		lines := fileCache[source.path]
		// line_start and line_end are 1-based.
		start := source.lineStart - contextLines - 1 // 0-based
		if start < 0 {
			start = 0
		}
		end := source.lineEnd + contextLines // 0-based exclusive
		if end > len(lines) {
			end = len(lines)
		}

		snippets = append(snippets, SnippetEntry{
			SourcePath: source.path,
			LineStart:  start + 1, // back to 1-based
			LineEnd:    end,
			Lines:      lines[start:end],
		})
	}

	return snippets, nil
}

func checkStale(fullPath string, dbMtime int64) error {
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrFileNotFound, fullPath)
		}
		return err
	}
	if info.ModTime().Unix() != dbMtime {
		return fmt.Errorf("%w: %s has been modified since last build", ErrSourceStale, filepath.Base(fullPath))
	}
	return nil
}

func readFileLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
