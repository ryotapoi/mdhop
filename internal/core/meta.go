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
			return t.Format("2006-01-02T15:04:05.000000000"), ""
		case 2:
			return t.UTC().Format("2006-01-02T15:04:05.000000000Z"), ""
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

	// Canonicalize equivalent decimals before padding, retaining all significant digits.
	decPart = strings.TrimRight(decPart, "0")
	paddedInt := padLeft(intPart, numIntPad)
	paddedDec := padRight(decPart, numDecPad)

	if negative {
		// A terminator above all digits reverses prefix ordering for negatives.
		return "0" + ninesComplement(paddedInt) + ninesComplement(paddedDec) + ":", ""
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

	for _, part := range strings.Split(s, ".") {
		if !isDigits(part) {
			return value, fmt.Sprintf("cannot parse %q as semver", value)
		}
	}
	return s, ""
}

func ninesComplement(s string) string {
	buf := make([]byte, len(s))
	for i, c := range s {
		buf[i] = byte('9' - (c - '0'))
	}
	return string(buf)
}

// storedSortValue keeps ordinary values in a binary partition and semver values
// in a numeric partition. Tagging both avoids collisions with arbitrary strings.
func storedSortValue(value, valueType string) string {
	if valueType == string(MetaTypeSemver) && value != "" {
		return "s:" + value
	}
	return "b:" + value
}

// compareSortValues defines one total order: binary values (including invalid
// semver fallbacks) first, then valid semver values. Non-semver ordering is unchanged.
func compareSortValues(a, b string) int {
	if strings.HasPrefix(a, "s:") && strings.HasPrefix(b, "s:") {
		return compareSemver(a[2:], b[2:])
	}
	return strings.Compare(a, b)
}

func compareSemver(a, b string) int {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(left) || i < len(right); i++ {
		var x, y string
		if i < len(left) {
			x = strings.TrimLeft(left[i], "0")
		}
		if i < len(right) {
			y = strings.TrimLeft(right[i], "0")
		}
		if len(x) < len(y) {
			return -1
		}
		if len(x) > len(y) {
			return 1
		}
		if cmp := strings.Compare(x, y); cmp != 0 {
			return cmp
		}
	}
	return 0
}
