package core

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// presetEntry is a key-type pair with stable ordering for YAML output.
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

// buildMetaYAMLNode constructs a yaml.Node for the meta section with comments.
func buildMetaYAMLNode(types map[string]MetaTypeInfo, inferred map[string]InferredMeta, noComment bool) *yaml.Node {
	typesMapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	keys := orderedKeys(types)
	for _, key := range keys {
		info := types[key]

		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}

		// Add comment
		if !noComment {
			comment := buildTypeComment(key, info, inferred)
			if comment != "" {
				keyNode.HeadComment = comment
			}
		}

		var valNode *yaml.Node
		if info.Name == MetaTypeOrdered {
			valNode = buildOrderedValueNode(info.OrderedValues)
		} else {
			valNode = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: string(info.Name)}
		}

		typesMapping.Content = append(typesMapping.Content, keyNode, valNode)
	}

	// Build meta mapping: { types: typesMapping }
	metaMapping := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "meta"},
			{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "types"},
					typesMapping,
				},
			},
		},
	}
	return metaMapping
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
		lines = append(lines, fmt.Sprintf("consider: %s:", key))
		lines = append(lines, fmt.Sprintf("  ordered: [%s]", strings.Join(im.UniqueValues, ", ")))
	} else if im.TotalValues > 0 {
		lines = append(lines, fmt.Sprintf("string (%d values%s)", im.TotalValues, formatSamples(im.SampleValues)))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// buildOrderedValueNode creates a mapping node: { ordered: [values...] }
func buildOrderedValueNode(values []string) *yaml.Node {
	seqNode := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
	for _, v := range values {
		seqNode.Content = append(seqNode.Content, &yaml.Node{
			Kind: yaml.ScalarNode, Tag: "!!str", Value: v,
		})
	}
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "ordered"},
			seqNode,
		},
	}
}

// mergeIntoExistingYAML merges a new meta.types node into existing YAML content.
// Preserves all existing sections (build, exclude, etc.).
func mergeIntoExistingYAML(existingData []byte, metaNode *yaml.Node) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(existingData, &doc); err != nil {
		return nil, fmt.Errorf("parse existing YAML: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("unexpected YAML structure")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping at YAML root")
	}

	// Extract the types node from metaNode
	// metaNode is: { meta: { types: { ... } } }
	newTypesNode := metaNode.Content[1].Content[1] // meta -> value -> types -> value

	// Find existing meta key
	metaIdx := -1
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "meta" {
			metaIdx = i
			break
		}
	}

	if metaIdx >= 0 {
		metaVal := root.Content[metaIdx+1]
		if metaVal.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("meta is not a mapping")
		}
		// Find existing types key within meta
		typesIdx := -1
		for i := 0; i < len(metaVal.Content)-1; i += 2 {
			if metaVal.Content[i].Value == "types" {
				typesIdx = i
				break
			}
		}
		if typesIdx >= 0 {
			// Merge into existing types: add new keys only
			existingTypes := metaVal.Content[typesIdx+1]
			existingKeySet := make(map[string]bool)
			for i := 0; i < len(existingTypes.Content)-1; i += 2 {
				existingKeySet[existingTypes.Content[i].Value] = true
			}
			for i := 0; i < len(newTypesNode.Content)-1; i += 2 {
				key := newTypesNode.Content[i].Value
				if !existingKeySet[key] {
					existingTypes.Content = append(existingTypes.Content, newTypesNode.Content[i], newTypesNode.Content[i+1])
				}
			}
		} else {
			// Add types key to meta
			metaVal.Content = append(metaVal.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "types"},
				newTypesNode,
			)
		}
	} else {
		// Add entire meta section
		root.Content = append(root.Content, metaNode.Content...)
	}

	return marshalYAML(&doc)
}

// marshalYAML encodes a yaml.Node with 2-space indent to match mdhop.yaml convention.
func marshalYAML(node any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// generateMetaYAML produces the final YAML string, merging with existing config if present.
// existingData is the raw mdhop.yaml content (nil if file doesn't exist).
func generateMetaYAML(existingData []byte, types map[string]MetaTypeInfo, inferred map[string]InferredMeta, noComment bool) (string, error) {
	metaNode := buildMetaYAMLNode(types, inferred, noComment)

	if existingData == nil {
		doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{metaNode}}
		out, err := marshalYAML(doc)
		if err != nil {
			return "", err
		}
		return string(out), nil
	}

	out, err := mergeIntoExistingYAML(existingData, metaNode)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
