package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	inferThresholdPercent = 80
	orderedMaxCardinality = 10
	uniqueSetCutoff       = 20
	maxSampleValues       = 5
)

// keyStats tracks per-key aggregated statistics for type inference.
type keyStats struct {
	total       int
	dateMatch   int
	numberMatch int
	semverMatch int
	samples     []string
	uniqueSet   map[string]struct{}
	uniqueCount int // -1 means cutoff exceeded
}

// InferredMeta holds the inference result for a single frontmatter key.
type InferredMeta struct {
	Key          string
	InferredType MetaTypeName
	TotalValues  int
	MatchCount   int
	SampleValues []string
	UniqueValues []string // non-nil only when cardinality <= orderedMaxCardinality
	UniqueCount  int      // -1 when cutoff exceeded
}

// looksLikeDate returns true if the value can be parsed as a date.
func looksLikeDate(v string) bool {
	_, warning := normalizeDate(v)
	return warning == ""
}

// looksLikeNumber returns true if the value can be parsed as a number.
func looksLikeNumber(v string) bool {
	_, warning := normalizeNumber(v)
	return warning == ""
}

// looksLikeSemver returns true if the value can be parsed as semver.
func looksLikeSemver(v string) bool {
	_, warning := normalizeSemver(v)
	return warning == ""
}

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

// inferType determines the best type for a key based on aggregated stats.
func inferType(key string, stats *keyStats) InferredMeta {
	result := InferredMeta{
		Key:          key,
		InferredType: MetaTypeString,
		TotalValues:  stats.total,
		SampleValues: stats.samples,
		UniqueCount:  stats.uniqueCount,
	}

	// Populate unique values if within cardinality threshold
	if stats.uniqueSet != nil && stats.uniqueCount >= 0 && stats.uniqueCount <= orderedMaxCardinality {
		vals := make([]string, 0, len(stats.uniqueSet))
		for v := range stats.uniqueSet {
			vals = append(vals, v)
		}
		sort.Strings(vals)
		result.UniqueValues = vals
	}

	if stats.total == 0 {
		return result
	}

	// Find the best matching type above threshold
	type candidate struct {
		name  MetaTypeName
		count int
	}
	candidates := []candidate{
		{MetaTypeDate, stats.dateMatch},
		{MetaTypeNumber, stats.numberMatch},
		{MetaTypeSemver, stats.semverMatch},
	}

	bestName := MetaTypeString
	bestCount := 0
	for _, c := range candidates {
		if c.count*100/stats.total >= inferThresholdPercent && c.count > bestCount {
			bestName = c.name
			bestCount = c.count
		}
	}

	result.InferredType = bestName
	result.MatchCount = bestCount
	return result
}

// mergeMetaConfig merges proposed types into existing config.
// Existing keys are preserved (user's explicit choice). Returns merged map,
// list of added keys, and list of skipped (already existing) keys.
func mergeMetaConfig(existing, proposed map[string]MetaTypeInfo) (map[string]MetaTypeInfo, []string, []string) {
	merged := make(map[string]MetaTypeInfo, len(existing)+len(proposed))
	for k, v := range existing {
		merged[k] = v
	}

	var added, skipped []string
	for k, v := range proposed {
		if _, ok := existing[k]; ok {
			skipped = append(skipped, k)
			continue
		}
		merged[k] = v
		added = append(added, k)
	}
	sort.Strings(added)
	sort.Strings(skipped)
	return merged, added, skipped
}

// skipKeys are well-known frontmatter keys that should not be type-inferred.
var skipKeys = map[string]bool{
	"tags":    true,
	"aliases": true,
}

// scanMetaTypes scans vault files and infers frontmatter types.
func scanMetaTypes(vaultPath string, cfg Config) (map[string]InferredMeta, error) {
	files, err := collectMarkdownFiles(vaultPath)
	if err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(cfg.Build.ExcludePaths); err != nil {
		return nil, err
	}
	files = filterBuildExcludes(files, cfg.Build.ExcludePaths)

	// Aggregate stats per key
	statsMap := make(map[string]*keyStats)

	for _, rel := range files {
		fullPath := filepath.Join(vaultPath, rel)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		pr := parseLinks(string(content))
		for _, entry := range pr.Meta {
			if skipKeys[entry.Key] {
				continue
			}
			v := entry.Value
			ks, ok := statsMap[entry.Key]
			if !ok {
				ks = &keyStats{uniqueSet: make(map[string]struct{})}
				statsMap[entry.Key] = ks
			}

			// Skip empty values from stats (excluded from denominator)
			if v == "" {
				continue
			}

			ks.total++

			if looksLikeDate(v) {
				ks.dateMatch++
			}
			if looksLikeNumber(v) {
				ks.numberMatch++
			}
			if looksLikeSemver(v) {
				ks.semverMatch++
			}

			// Samples
			if len(ks.samples) < maxSampleValues {
				ks.samples = append(ks.samples, v)
			}

			// Unique set with cutoff
			if ks.uniqueCount >= 0 {
				ks.uniqueSet[v] = struct{}{}
				ks.uniqueCount = len(ks.uniqueSet)
				if ks.uniqueCount > uniqueSetCutoff {
					ks.uniqueSet = nil
					ks.uniqueCount = -1
				}
			}
		}
	}

	// Infer types
	result := make(map[string]InferredMeta, len(statsMap))
	for key, ks := range statsMap {
		result[key] = inferType(key, ks)
	}
	return result, nil
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

// InitMetaOptions controls the behavior of InitMeta.
type InitMetaOptions struct {
	Preset    bool
	Scan      bool
	NoComment bool
}

// InitMetaResult holds the output of InitMeta.
type InitMetaResult struct {
	YAML     string
	Added    []string
	Skipped  []string
	Inferred []InferredMeta
}

// InitMeta generates meta type definitions for a vault.
func InitMeta(vaultPath string, opts InitMetaOptions) (*InitMetaResult, error) {
	if !opts.Preset && !opts.Scan {
		return nil, fmt.Errorf("at least one of --preset or --scan is required")
	}

	// Verify vault exists
	info, err := os.Stat(vaultPath)
	if err != nil {
		return nil, fmt.Errorf("vault path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("vault path is not a directory: %s", vaultPath)
	}

	cfg, err := LoadConfig(vaultPath)
	if err != nil {
		return nil, err
	}

	// Build proposed types
	proposed := make(map[string]MetaTypeInfo)
	var inferredMap map[string]InferredMeta

	if opts.Preset {
		for _, p := range presetMetaTypes() {
			proposed[p.Key] = p.Info
		}
	}

	if opts.Scan {
		scanned, err := scanMetaTypes(vaultPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		inferredMap = scanned
		// Scan results override preset (data-driven > curated)
		for key, im := range scanned {
			proposed[key] = MetaTypeInfo{Name: im.InferredType}
		}
	}

	// Merge with existing config
	existing := cfg.Meta.Types
	if existing == nil {
		existing = make(map[string]MetaTypeInfo)
	}
	merged, added, skipped := mergeMetaConfig(existing, proposed)

	// Read existing config file for YAML merge (reuse raw bytes, not parsed config)
	var existingData []byte
	configPath := filepath.Join(vaultPath, "mdhop.yaml")
	data, readErr := os.ReadFile(configPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return nil, readErr
	}
	if readErr == nil {
		existingData = data
	}

	// Generate YAML
	yamlStr, err := generateMetaYAML(existingData, merged, inferredMap, opts.NoComment)
	if err != nil {
		return nil, err
	}

	var inferredList []InferredMeta
	if inferredMap != nil {
		for _, im := range inferredMap {
			inferredList = append(inferredList, im)
		}
		sort.Slice(inferredList, func(i, j int) bool {
			return inferredList[i].Key < inferredList[j].Key
		})
	}

	return &InitMetaResult{
		YAML:     yamlStr,
		Added:    added,
		Skipped:  skipped,
		Inferred: inferredList,
	}, nil
}
