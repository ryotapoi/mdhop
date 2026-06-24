package core

import (
	"os"
	"sort"
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
	diskPaths := newVaultDiskPathResolver(vaultPath)

	// Aggregate stats per key
	statsMap := make(map[string]*keyStats)

	for _, rel := range files {
		fullPath, err := diskPaths.existingPath(rel)
		if err != nil {
			return nil, err
		}
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
