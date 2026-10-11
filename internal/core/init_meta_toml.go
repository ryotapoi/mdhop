package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// presetEntry is a key-type pair with stable ordering for TOML output.
type presetEntry struct {
	Key  string
	Info MetaTypeInfo
}

// presetMetaTypes returns the recommended type definitions in a stable order.
func presetMetaTypes() []presetEntry {
	return []presetEntry{
		// dates
		{"date", MetaTypeInfo{Name: MetaTypeDate}},
		{"created", MetaTypeInfo{Name: MetaTypeDate}},
		{"modified", MetaTypeInfo{Name: MetaTypeDate}},
		{"updated", MetaTypeInfo{Name: MetaTypeDate}},
		{"lastmod", MetaTypeInfo{Name: MetaTypeDate}},
		{"due", MetaTypeInfo{Name: MetaTypeDate}},
		{"deadline", MetaTypeInfo{Name: MetaTypeDate}},
		{"scheduled", MetaTypeInfo{Name: MetaTypeDate}},
		{"start", MetaTypeInfo{Name: MetaTypeDate}},
		{"done", MetaTypeInfo{Name: MetaTypeDate}},
		// numbers
		{"priority", MetaTypeInfo{Name: MetaTypeNumber}},
		{"weight", MetaTypeInfo{Name: MetaTypeNumber}},
		{"order", MetaTypeInfo{Name: MetaTypeNumber}},
		{"rating", MetaTypeInfo{Name: MetaTypeNumber}},
		// semver
		{"version", MetaTypeInfo{Name: MetaTypeSemver}},
	}
}

// orderedKeys returns keys from types in a stable order:
// preset keys in definition order, then remaining keys alphabetically.
func orderedKeys(types map[string]MetaTypeInfo) []string {
	presets := presetMetaTypes()
	presetSet := make(map[string]bool, len(presets))
	var result []string

	// Preset keys in definition order (only those present in types)
	for _, p := range presets {
		if _, ok := types[p.Key]; ok {
			result = append(result, p.Key)
			presetSet[p.Key] = true
		}
	}

	// Non-preset keys alphabetically
	var extra []string
	for k := range types {
		if !presetSet[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	result = append(result, extra...)
	return result
}

// formatSamples returns a quoted, truncated sample preview like: e.g. "val1", "val2"
func formatSamples(samples []string) string {
	if len(samples) == 0 {
		return ""
	}
	if len(samples) > 2 {
		samples = samples[:2]
	}
	quoted := make([]string, len(samples))
	for i, s := range samples {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return ", e.g. " + strings.Join(quoted, ", ")
}

// buildTypeComment generates the comment for a type entry.
func buildTypeComment(key string, info MetaTypeInfo, inferred map[string]InferredMeta) string {
	im, hasInferred := inferred[key]
	if !hasInferred {
		return "preset"
	}

	if info.Name != MetaTypeString {
		return fmt.Sprintf("inferred: %s (%d/%d values%s)", info.Name, im.MatchCount, im.TotalValues, formatSamples(im.SampleValues))
	}

	// String type — check for ordered candidate
	var lines []string
	if im.UniqueValues != nil && im.UniqueCount > 0 && im.UniqueCount <= orderedMaxCardinality {
		lines = append(lines, fmt.Sprintf("string (%d unique values: %s)", im.UniqueCount, strings.Join(im.UniqueValues, ", ")))
		lines = append(lines, fmt.Sprintf("consider: %s = { ordered = %s }", key, formatOrderedSuggestion(im.UniqueValues)))
	} else if im.TotalValues > 0 {
		lines = append(lines, fmt.Sprintf("string (%d values%s)", im.TotalValues, formatSamples(im.SampleValues)))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// generateMetaTOML preserves the existing config's values while adding types.
// Formatting and user comments are regenerated; unknown settings are retained.
func generateMetaTOML(existingData []byte, types map[string]MetaTypeInfo, inferred map[string]InferredMeta, noComment bool) (string, error) {
	root := make(map[string]any)
	if err := toml.Unmarshal(existingData, &root); err != nil {
		return "", fmt.Errorf("parse existing TOML: %w", err)
	}
	meta, _ := root["meta"].(map[string]any)
	if meta == nil {
		meta = make(map[string]any)
		root["meta"] = meta
	}
	existing, _ := meta["types"].(map[string]any)
	// Emit types separately for a stable, readable order and explanatory comments.
	delete(meta, "types")
	out, err := toml.Marshal(root)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	buf.Write(out)
	buf.WriteString("\n[meta.types]\n")
	for _, key := range orderedKeys(types) {
		info := types[key]
		if !noComment {
			for _, line := range strings.Split(buildTypeComment(key, info, inferred), "\n") {
				if line != "" {
					buf.WriteString("# " + line + "\n")
				}
			}
		}
		var value any = string(info.Name)
		if info.Name == MetaTypeOrdered {
			value = map[string]any{"ordered": info.OrderedValues}
		}
		if old, ok := existing[key]; ok {
			value = old
		}
		err := toml.NewEncoder(&buf).SetTablesInline(true).Encode(map[string]any{key: value})
		if err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func formatOrderedSuggestion(values []string) string {
	var buf strings.Builder
	_ = toml.NewEncoder(&buf).Encode(map[string]any{"values": values})
	_, value, _ := strings.Cut(strings.TrimSpace(buf.String()), " = ")
	return value
}
