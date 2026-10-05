package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ConvertOptions controls the convert operation.
type ConvertOptions struct {
	ToFormat string // "wikilink" or "markdown"
	DryRun   bool
	Files    []string // limit to these source files
}

// ConvertResult reports the outcome of the convert operation.
type ConvertResult struct {
	Rewritten []RewrittenLink
}

// Convert converts wikilinks and inline Markdown links by scanning files
// directly (no DB required). Reference links and definitions are unchanged.
func Convert(vaultPath string, opts ConvertOptions) (*ConvertResult, error) {
	if opts.ToFormat != "wikilink" && opts.ToFormat != "markdown" {
		return nil, fmt.Errorf("invalid ToFormat: %q (must be wikilink or markdown)", opts.ToFormat)
	}

	result := &ConvertResult{}
	rewrites, err := scanAndRewrite(vaultPath, scanRewriteOptions{
		DryRun: opts.DryRun,
		Prepare: func(files, _ []string) (scanRewritePlan, error) {
			fileSet := make(map[string]bool, len(files))
			for _, f := range files {
				fileSet[f] = true
			}
			fileScope := make(map[string]bool, len(opts.Files))
			for _, f := range opts.Files {
				np := NormalizePath(f)
				if !fileSet[np] {
					return scanRewritePlan{}, fmt.Errorf("file not found or excluded: %s", f)
				}
				fileScope[np] = true
			}

			var noteNameSet, notePathSet map[string]bool
			if opts.ToFormat == "markdown" {
				noteNameSet = make(map[string]bool, len(files))
				notePathSet = make(map[string]bool, len(files))
				for _, f := range files {
					name := strings.TrimSuffix(filepath.Base(f), ".md")
					noteNameSet[strings.ToLower(name)] = true
					notePathSet[strings.ToLower(NormalizePath(f))] = true
				}
			}

			scanFiles := files
			if len(fileScope) > 0 {
				scanFiles = nil
				for _, f := range files {
					if fileScope[f] {
						scanFiles = append(scanFiles, f)
					}
				}
			}
			return scanRewritePlan{Files: scanFiles, Rewrite: func(sourcePath string, content []byte) ([]rewriteEntry, error) {
				isAssetTarget := func(target string) bool {
					return !isNoteTarget(target, sourcePath, noteNameSet, notePathSet)
				}
				var links []linkOccur
				if opts.ToFormat == "wikilink" {
					links = parseLinksForConvert(string(content)).Links
				} else {
					links = parseLinks(string(content)).Links
				}
				var entries []rewriteEntry
				for _, lo := range links {
					var newRawLink string

					switch opts.ToFormat {
					case "wikilink":
						if lo.linkType != LinkTypeMarkdown {
							continue
						}
						newRawLink = convertMarkdownToWikilink(lo.rawLink, lo.inTable)
					case "markdown":
						if lo.linkType != LinkTypeWikilink {
							continue
						}
						newRawLink = convertWikilinkToMarkdown(lo.rawLink, isAssetTarget, lo.inTable)
					}

					if newRawLink == lo.rawLink || newRawLink == "" {
						continue
					}

					entries = append(entries, rewriteEntry{
						rawLink:    lo.rawLink,
						linkType:   lo.linkType,
						lineStart:  lo.lineStart,
						sourcePath: sourcePath,
						newRawLink: newRawLink,
					})
				}
				return entries, nil
			}}, nil
		},
	})
	if err != nil {
		return nil, err
	}

	// Build rewritten result entries.
	for _, re := range rewrites {
		result.Rewritten = append(result.Rewritten, RewrittenLink{
			File:    re.sourcePath,
			OldLink: re.rawLink,
			NewLink: re.newRawLink,
		})
	}

	return result, nil
}

// convertMarkdownToWikilink converts a markdown link rawLink to wikilink format.
// Returns the original rawLink if conversion is not possible.
func convertMarkdownToWikilink(rawLink string, table ...bool) string {
	text, url := extractMarkdownParts(rawLink)
	if text == "" && url == "" {
		return rawLink
	}

	// Skip URLs.
	target, subpath, external := markdownDestination(url)
	if external {
		return rawLink
	}

	if !wikilinkRepresentable(target, subpath, LinkTypeWikilink) || strings.Contains(text, "]]") {
		return rawLink
	}
	separator := "|"
	if len(table) > 0 && table[0] {
		separator = `\|`
	}

	// Self-link: [text](#heading)
	if target == "" && subpath != "" {
		wikiTarget := subpath // e.g. "#Section"
		if text == subpath {
			return "[[" + wikiTarget + "]]"
		}
		if len(table) > 0 && table[0] && !tableWikiAliasSafe(subpath) {
			return rawLink
		}
		return "[[" + wikiTarget + separator + text + "]]"
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
		if len(table) > 0 && table[0] && !tableWikiAliasSafe(wikiTarget+subpath) {
			return rawLink
		}
		return "[[" + wikiTarget + subpath + separator + text + "]]"
	}
	return "[[" + wikiTarget + subpath + "]]"
}

// convertWikilinkToMarkdown converts a wikilink rawLink to markdown link format.
// isAssetTarget determines if a target should be treated as an asset (no .md added).
// Returns the original rawLink if conversion is not possible.
func convertWikilinkToMarkdown(rawLink string, isAssetTarget func(string) bool, table ...bool) string {
	parts := splitWikilinkParts(rawLink, table...)
	if parts.target == "" && parts.subpath == "" && parts.alias == "" {
		return rawLink
	}

	target, subpath := parts.target, parts.subpath
	alias := strings.TrimPrefix(strings.TrimPrefix(parts.alias, `\`), "|")

	// Self-link: [[#Heading]] or [[#Heading|alias]]
	if target == "" && subpath != "" {
		text := subpath // e.g. "#Section"
		if alias != "" {
			text = alias
		}
		return "[" + text + "](" + encodeMarkdownDestination("", subpath) + ")"
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

	return "[" + text + "](" + encodeMarkdownDestination(mdTarget, subpath) + ")"
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
// 3. Extension exists but matches a note basename (basename link) or path → note
// 4. Otherwise → asset
func isNoteTarget(target, sourcePath string, noteNameSet, notePathSet map[string]bool) bool {
	ext := filepath.Ext(target)
	if ext == "" {
		return true // no extension → note
	}
	if strings.EqualFold(ext, ".md") {
		return true // .md → note
	}
	if isBasenameLink(target) {
		return noteNameSet[strings.ToLower(target)]
	}
	// Explicit paths must match a note at that path, not a basename elsewhere.
	notePath := resolveToVaultRelative(sourcePath, linkOccur{target: target, isRelative: isRelativePath(target)})
	return notePathSet[strings.ToLower(notePath+".md")]
}

// parseLinksForConvert extends parseLinks with markdown self-link support.
// Markdown self-links [text](#heading) are not captured by parseMarkdownLinks
// (which requires target != ""), so we add an extra pass.
func parseLinksForConvert(content string) parseResult {
	pr := parseLinks(content)

	// Additional pass: collect markdown self-links.
	lines := strings.Split(content, "\n")
	fmEnd := frontmatterEnd(lines)
	tableLines := bodyTableLines(lines, fmEnd)
	walkBodyLines(lines, fmEnd, func(lineNum int, raw, clean string) {
		if _, _, definition := referenceDefinition(raw); definition {
			return
		}
		links := parseMarkdownSelfLinks(maskWikiLinks(clean), lineNum)
		for i := range links {
			links[i].inTable = tableLines[lineNum]
		}
		pr.Links = append(pr.Links, links...)
	})
	return pr
}

// parseMarkdownSelfLinks extracts inline Markdown self-links [text](#fragment),
// including balanced parentheses in the fragment.
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
		mid, valid := referenceBracketEnd(remaining, open)
		if mid < 0 {
			break
		}
		if !valid || mid+1 >= len(remaining) || remaining[mid+1] != '(' {
			remaining = remaining[mid+1:]
			continue
		}
		close := markdownDestinationEnd(remaining, mid+2)
		if close == -1 {
			break
		}
		rawTarget := strings.TrimSpace(remaining[mid+2 : close])
		rawLink := remaining[open : close+1]

		// Only self-links: target starts with # and has no path component.
		target, subpath, external := markdownDestination(rawTarget)
		if target == "" && subpath != "" && !external {
			out = append(out, linkOccur{
				target:     "",
				isBasename: false,
				isRelative: false,
				linkType:   LinkTypeMarkdown,
				rawLink:    rawLink,
				subpath:    subpath,
				lineStart:  lineNum,
				lineEnd:    lineNum,
			})
		}
		remaining = remaining[close+1:]
	}
	return out
}
