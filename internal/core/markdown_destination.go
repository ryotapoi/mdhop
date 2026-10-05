package core

import (
	"html"
	"strconv"
	"strings"
	"unicode/utf8"
)

func asciiPunctuation(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}

// decodeMarkdownLexical walks the original span once. Generated escapes and
// entities are deliberately not parsed again.
func decodeMarkdownLexical(raw string) string {
	var out strings.Builder
	for i := 0; i < len(raw); {
		if raw[i] == '\\' && i+1 < len(raw) && asciiPunctuation(raw[i+1]) {
			out.WriteByte(raw[i+1])
			i += 2
			continue
		}
		if raw[i] == '&' {
			if decoded, n := markdownEntity(raw[i:]); n > 0 {
				out.WriteString(decoded)
				i += n
				continue
			}
		}
		out.WriteByte(raw[i])
		i++
	}
	return out.String()
}

func markdownEntity(raw string) (string, int) {
	end := strings.IndexByte(raw, ';')
	if end < 2 || end > 33 {
		return "", 0
	}
	name := raw[1:end]
	if name[0] == '#' {
		digits, base, max := name[1:], 10, 7
		if len(digits) > 0 && (digits[0] == 'x' || digits[0] == 'X') {
			digits, base, max = digits[1:], 16, 6
		}
		if len(digits) == 0 || len(digits) > max {
			return "", 0
		}
		for _, c := range digits {
			if !(c >= '0' && c <= '9' || base == 16 && (c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F')) {
				return "", 0
			}
		}
		value, err := strconv.ParseUint(digits, base, 32)
		if err != nil {
			return "", 0
		}
		r := rune(value)
		if r == 0 || !utf8.ValidRune(r) {
			r = utf8.RuneError
		}
		return string(r), end + 1
	}
	if len(name) < 2 || len(name) > 31 {
		return "", 0
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return "", 0
		}
	}
	candidate := raw[:end+1]
	decoded := html.UnescapeString(candidate)
	// The HTML decoder accepts legacy prefix matches; Markdown requires a full
	// semicolon-terminated name. A partial match leaves its trailing semicolon.
	if decoded == candidate || strings.HasSuffix(decoded, ";") && decoded != ";" {
		return "", 0
	}
	return decoded, end + 1
}

func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func decodePercent(raw string) string {
	var out strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '%' && i+2 < len(raw) {
			hi, h := hexValue(raw[i+1])
			lo, l := hexValue(raw[i+2])
			if h && l {
				out.WriteByte(hi*16 + lo)
				i += 2
				continue
			}
		}
		out.WriteByte(raw[i])
	}
	return out.String()
}

// markdownDestination separates syntax before percent decoding. Encoded # is
// part of a filename; it must never become a fragment delimiter on a second pass.
func markdownDestination(raw string) (target, subpath string, external bool) {
	lexical := decodeMarkdownLexical(strings.TrimSpace(raw))
	if isURL(lexical) {
		return "", "", true
	}
	target, subpath = extractSubpath(lexical)
	target = decodePercent(target)
	if subpath != "" {
		subpath = "#" + decodePercent(subpath[1:])
	}
	return
}

// encodeMarkdownComponent protects delimiters and lexical decoding while
// retaining readable Unicode, slashes and ordinary path punctuation.
func encodeMarkdownComponent(value string) string {
	const hex = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c <= ' ' || c == 127 || strings.ContainsRune("%#&\\()<>|:`", rune(c)) {
			out.WriteByte('%')
			out.WriteByte(hex[c>>4])
			out.WriteByte(hex[c&15])
		} else {
			out.WriteByte(c)
		}
	}
	return out.String()
}

func encodeMarkdownDestination(target, subpath string) string {
	result := encodeMarkdownComponent(target)
	if subpath != "" {
		result += "#" + encodeMarkdownComponent(strings.TrimPrefix(subpath, "#"))
	}
	return result
}

func wikilinkRepresentable(target, subpath, alias string, linkType LinkType) bool {
	// A trailing bracket touching the closing wrapper would close the link
	// early. A subpath or alias separator can keep that bracket in the value.
	if alias == "" && strings.HasSuffix(target+subpath, "]") {
		return false
	}
	// Quoted frontmatter values bypass the body code span scanner. YAML source
	// correspondence and scalar meaning are checked before writing the candidate.
	if linkType != LinkTypeFrontmatterWikilink && strings.Contains(target+subpath, "`") {
		return false
	}
	return !strings.ContainsAny(target, "#|\r\n") && !strings.Contains(target, "]]") && !strings.ContainsAny(subpath, "|\r\n") && !strings.Contains(subpath, "]]")
}

// An odd trailing backslash before the table alias escape would turn its pipe
// into a cell delimiter. The wikilink syntax cannot represent that combination.
func tableWikiAliasSafe(name string) bool {
	slashes := len(name) - len(strings.TrimRight(name, `\`))
	return slashes%2 == 0
}
