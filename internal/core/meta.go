package core

import (
	"fmt"
	"strings"
	"time"
)

// NormalizeSortValue converts a frontmatter value into a sort-friendly string
// based on the declared type. Returns (sortValue, warning). warning == "" means
// no warning. On normalization failure, the original value is returned as-is
// (string fallback) with a non-empty warning.
func NormalizeSortValue(value string, typeInfo MetaTypeInfo) (string, string) {
	if value == "" {
		return "", ""
	}

	switch typeInfo.Name {
	case MetaTypeString:
		return value, ""
	case MetaTypeDate:
		return normalizeDate(value)
	case MetaTypeNumber:
		return normalizeNumber(value)
	case MetaTypeSemver:
		return normalizeSemver(value)
	case MetaTypeOrdered:
		return normalizeOrdered(value, typeInfo.OrderedValues)
	default:
		return value, ""
	}
}

// dateLayout pairs a parse layout with its output category.
type dateLayout struct {
	parse    string
	category int // 0=date-only, 1=datetime-no-tz, 2=datetime-with-tz
}

var dateLayouts = []dateLayout{
	{"2006-01-02T15:04:05Z07:00", 2},
	{"2006-01-02T15:04:05", 1},
	{"2006-01-02", 0},
	{"2006-1-2", 0},
	{"2006/01/02", 0},
	{"2006/1/2", 0},
}

func normalizeDate(value string) (string, string) {
	for _, dl := range dateLayouts {
		t, err := time.Parse(dl.parse, value)
		if err != nil {
			continue
		}
		switch dl.category {
		case 0:
			return t.Format("2006-01-02"), ""
		case 1:
			return t.Format("2006-01-02T15:04:05"), ""
		case 2:
			return t.UTC().Format("2006-01-02T15:04:05Z"), ""
		}
	}
	return value, fmt.Sprintf("cannot parse %q as date", value)
}

const (
	numIntPad = 20
	numDecPad = 8
)

func normalizeNumber(value string) (string, string) {
	// Reject scientific notation
	if strings.ContainsAny(value, "eE") {
		return value, fmt.Sprintf("scientific notation not supported: %q", value)
	}

	s := strings.TrimPrefix(value, "+")
	negative := false
	if strings.HasPrefix(s, "-") {
		negative = true
		s = s[1:]
	}

	// Split integer and decimal parts
	var intPart, decPart string
	dotIdx := strings.Index(s, ".")
	if dotIdx < 0 {
		intPart = s
		decPart = "0"
	} else {
		if s == "." {
			return value, fmt.Sprintf("cannot parse %q as number", value)
		}
		if strings.Count(s, ".") > 1 {
			return value, fmt.Sprintf("cannot parse %q as number", value)
		}
		intPart = s[:dotIdx]
		decPart = s[dotIdx+1:]
		if intPart == "" {
			intPart = "0"
		}
		if decPart == "" {
			decPart = "0"
		}
	}

	// Validate digits only
	if !isDigits(intPart) || !isDigits(decPart) {
		return value, fmt.Sprintf("cannot parse %q as number", value)
	}

	// Check integer part length
	if len(intPart) > numIntPad {
		return value, fmt.Sprintf("number too large (>%d digits): %q", numIntPad, value)
	}

	// Normalize -0 / -0.0 to positive zero
	if negative && isAllZeros(intPart) && isAllZeros(decPart) {
		negative = false
	}

	// Pad
	paddedInt := padLeft(intPart, numIntPad)
	paddedDec := padRight(decPart, numDecPad)

	if negative {
		return "0" + ninesComplement(paddedInt) + ninesComplement(paddedDec), ""
	}
	return "1" + paddedInt + "." + paddedDec, ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isAllZeros(s string) bool {
	for _, c := range s {
		if c != '0' {
			return false
		}
	}
	return true
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat("0", n-len(s)) + s
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat("0", n-len(s))
}

func normalizeOrdered(value string, orderedValues []string) (string, string) {
	for i, v := range orderedValues {
		if v == value {
			return fmt.Sprintf("%05d", i), ""
		}
	}
	return value, fmt.Sprintf("value %q not in ordered list", value)
}

func normalizeSemver(value string) (string, string) {
	s := value
	if strings.HasPrefix(s, "v") || strings.HasPrefix(s, "V") {
		s = s[1:]
	}

	// Reject prerelease/build metadata
	if strings.ContainsAny(s, "-+") {
		return value, fmt.Sprintf("prerelease/build metadata not supported: %q", value)
	}

	parts := strings.Split(s, ".")
	if len(parts) <= 1 {
		return value, fmt.Sprintf("cannot parse %q as semver", value)
	}

	// Pad to at least 3 segments
	for len(parts) < 3 {
		parts = append(parts, "0")
	}

	padded := make([]string, len(parts))
	for i, p := range parts {
		// Validate numeric
		if !isDigits(p) {
			return value, fmt.Sprintf("cannot parse %q as semver", value)
		}
		padded[i] = padLeft(p, 5)
	}
	return strings.Join(padded, "."), ""
}

func ninesComplement(s string) string {
	buf := make([]byte, len(s))
	for i, c := range s {
		buf[i] = byte('9' - (c - '0'))
	}
	return string(buf)
}
