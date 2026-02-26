package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConvertOptions controls the convert operation.
type ConvertOptions struct {
	ToFormat string   // "wikilink" or "markdown"
	DryRun   bool
	Files    []string // limit to these source files
}

// ConvertResult reports the outcome of the convert operation.
type ConvertResult struct {
	Rewritten []RewrittenLink
}

// Convert converts links between wikilink and markdown link formats.
// It works by scanning files directly (no DB required).
func Convert(vaultPath string, opts ConvertOptions) (*ConvertResult, error) {
	if opts.ToFormat != "wikilink" && opts.ToFormat != "markdown" {
		return nil, fmt.Errorf("invalid ToFormat: %q (must be wikilink or markdown)", opts.ToFormat)
	}

	files, err := collectMarkdownFiles(vaultPath)
	if err != nil {
		return nil, err
	}

	cfg, err := LoadConfig(vaultPath)
	if err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(cfg.Build.ExcludePaths); err != nil {
		return nil, err
	}
	files = filterBuildExcludes(files, cfg.Build.ExcludePaths)

	sort.Strings(files)

	// Build fileSet for existence checks.
	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}

	// Build file scope if --file is specified.
	var fileScope map[string]bool
	if len(opts.Files) > 0 {
		fileScope = make(map[string]bool, len(opts.Files))
		for _, f := range opts.Files {
			np := NormalizePath(f)
			if !fileSet[np] {
				return nil, fmt.Errorf("file not found or excluded: %s", f)
			}
			fileScope[np] = true
		}
	}

	// For wikilink → markdown, build note name set and isAssetTarget closure.
	var isAssetTarget func(string) bool
	if opts.ToFormat == "markdown" {
		noteNameSet := make(map[string]bool, len(files))
		for _, f := range files {
			base := filepath.Base(f)
			name := strings.TrimSuffix(base, ".md")
			noteNameSet[strings.ToLower(name)] = true
		}
		isAssetTarget = func(target string) bool {
			return !isNoteTarget(target, noteNameSet)
		}
	}

	result := &ConvertResult{}
	var rewrites []rewriteEntry

	for _, sourcePath := range files {
		if fileScope != nil && !fileScope[sourcePath] {
			continue
		}

		fullPath := filepath.Join(vaultPath, sourcePath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}

		var links []linkOccur
		if opts.ToFormat == "wikilink" {
			links = parseLinksForConvert(string(content))
		} else {
			links = parseLinks(string(content))
		}

		for _, lo := range links {
			var newRawLink string

			switch opts.ToFormat {
			case "wikilink":
				if lo.linkType != "markdown" {
					continue
				}
				newRawLink = convertMarkdownToWikilink(lo.rawLink)
			case "markdown":
				if lo.linkType != "wikilink" {
					continue
				}
				newRawLink = convertWikilinkToMarkdown(lo.rawLink, isAssetTarget)
			}

			if newRawLink == lo.rawLink || newRawLink == "" {
				continue
			}

			rewrites = append(rewrites, rewriteEntry{
				rawLink:    lo.rawLink,
				linkType:   lo.linkType,
				lineStart:  lo.lineStart,
				sourcePath: sourcePath,
				newRawLink: newRawLink,
			})
		}
	}

	// Build rewritten result entries.
	for _, re := range rewrites {
		result.Rewritten = append(result.Rewritten, RewrittenLink{
			File:    re.sourcePath,
			OldLink: re.rawLink,
			NewLink: re.newRawLink,
		})
	}

	if opts.DryRun || len(rewrites) == 0 {
		return result, nil
	}

	// Apply disk rewrites.
	groups := make(map[string][]rewriteEntry)
	for _, re := range rewrites {
		groups[re.sourcePath] = append(groups[re.sourcePath], re)
	}
	_, _, applyErr := applyFileRewrites(vaultPath, groups)
	if applyErr != nil {
		return nil, applyErr
	}

	return result, nil
}

// convertMarkdownToWikilink converts a markdown link rawLink to wikilink format.
// Returns the original rawLink if conversion is not possible.
func convertMarkdownToWikilink(rawLink string) string {
	text, url := extractMarkdownParts(rawLink)
	if text == "" && url == "" {
		return rawLink
	}

	// Skip URLs.
	if isURL(url) {
		return rawLink
	}

	target, subpath := extractSubpath(url)

	// Self-link: [text](#heading)
	if target == "" && subpath != "" {
		wikiTarget := subpath // e.g. "#Section"
		if text == subpath {
			return "[[" + wikiTarget + "]]"
		}
		return "[[" + wikiTarget + "|" + text + "]]"
	}

	// Build wikilink target: strip .md for notes.
	wikiTarget := buildRewritePath(target)

	// Determine if alias is needed.
	// Wikilink [[path/to/Name]] displays as "Name" (basename).
	// Wikilink [[Name#H]] displays as "Name#H" (or "Name > H" in Obsidian).
	// No alias needed if text matches either:
	//   - basename alone (e.g. text="Name" for [[Name#H]])
	//   - basename + subpath (e.g. text="Name#H" for [[Name#H]])
	baseName := filepath.Base(wikiTarget)
	needAlias := text != baseName
	if subpath != "" && text == baseName+subpath {
		needAlias = false
	}
	if needAlias {
		return "[[" + wikiTarget + subpath + "|" + text + "]]"
	}
	return "[[" + wikiTarget + subpath + "]]"
}

// convertWikilinkToMarkdown converts a wikilink rawLink to markdown link format.
// isAssetTarget determines if a target should be treated as an asset (no .md added).
// Returns the original rawLink if conversion is not possible.
func convertWikilinkToMarkdown(rawLink string, isAssetTarget func(string) bool) string {
	inner := strings.TrimPrefix(rawLink, "[[")
	inner = strings.TrimSuffix(inner, "]]")
	if inner == "" {
		return rawLink
	}

	// Extract alias.
	var alias string
	if idx := strings.Index(inner, "|"); idx >= 0 {
		alias = inner[idx+1:]
		inner = inner[:idx]
	}

	target, subpath := extractSubpath(inner)

	// Self-link: [[#Heading]] or [[#Heading|alias]]
	if target == "" && subpath != "" {
		text := subpath // e.g. "#Section"
		if alias != "" {
			text = alias
		}
		return "[" + text + "](" + subpath + ")"
	}

	// Determine if we need to add .md extension.
	mdTarget := target
	if isAssetTarget == nil || !isAssetTarget(target) {
		// It's a note — add .md if not already present.
		if !strings.HasSuffix(strings.ToLower(target), ".md") {
			mdTarget = target + ".md"
		}
	}

	// Determine display text.
	baseName := filepath.Base(target)
	text := baseName
	if subpath != "" {
		text = baseName + subpath
	}
	if alias != "" {
		text = alias
	}

	return "[" + text + "](" + mdTarget + subpath + ")"
}

// extractMarkdownParts extracts text and url from a markdown link [text](url).
func extractMarkdownParts(rawLink string) (text, url string) {
	if !strings.HasPrefix(rawLink, "[") {
		return "", ""
	}
	mid := strings.Index(rawLink, "](")
	if mid < 0 {
		return "", ""
	}
	text = rawLink[1:mid]
	url = rawLink[mid+2:]
	url = strings.TrimSuffix(url, ")")
	return text, url
}

// isNoteTarget determines if a wikilink target refers to a note (vs asset).
// Rules:
// 1. No extension → note
// 2. .md extension → note
// 3. Extension exists but matches a note basename → note
// 4. Otherwise → asset
func isNoteTarget(target string, noteNameSet map[string]bool) bool {
	ext := filepath.Ext(target)
	if ext == "" {
		return true // no extension → note
	}
	if strings.EqualFold(ext, ".md") {
		return true // .md → note
	}
	// Check if basename (without path) matches a known note name.
	base := filepath.Base(target)
	return noteNameSet[strings.ToLower(base)]
}

// parseLinksForConvert extends parseLinks with markdown self-link support.
// Markdown self-links [text](#heading) are not captured by parseMarkdownLinks
// (which requires target != ""), so we add an extra pass.
func parseLinksForConvert(content string) []linkOccur {
	out := parseLinks(content)

	// Additional pass: collect markdown self-links.
	lines := strings.Split(content, "\n")
	fmEnd := frontmatterEnd(lines)
	inFence := false
	startLine := 0
	if fmEnd > 0 {
		startLine = fmEnd + 1
	}
	for i := startLine; i < len(lines); i++ {
		lineNum := i + 1
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		clean := stripInlineCode(lines[i])
		out = append(out, parseMarkdownSelfLinks(clean, lineNum)...)
	}
	return out
}

// parseMarkdownSelfLinks extracts markdown self-links [text](#fragment) from a line.
func parseMarkdownSelfLinks(line string, lineNum int) []linkOccur {
	var out []linkOccur
	remaining := line
	for {
		open := strings.Index(remaining, "[")
		if open == -1 {
			break
		}
		// Skip wikilinks.
		if open+1 < len(remaining) && remaining[open+1] == '[' {
			remaining = remaining[open+2:]
			continue
		}
		mid := strings.Index(remaining[open:], "](")
		if mid == -1 {
			break
		}
		mid = open + mid
		close := strings.Index(remaining[mid+2:], ")")
		if close == -1 {
			break
		}
		close = mid + 2 + close
		rawTarget := strings.TrimSpace(remaining[mid+2 : close])
		rawLink := remaining[open : close+1]

		// Only self-links: target starts with # and has no path component.
		if strings.HasPrefix(rawTarget, "#") {
			out = append(out, linkOccur{
				target:     "",
				isBasename: false,
				isRelative: false,
				linkType:   "markdown",
				rawLink:    rawLink,
				subpath:    rawTarget,
				lineStart:  lineNum,
				lineEnd:    lineNum,
			})
		}
		remaining = remaining[close+1:]
	}
	return out
}
