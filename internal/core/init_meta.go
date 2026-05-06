package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

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
