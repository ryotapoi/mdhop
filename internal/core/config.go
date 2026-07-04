package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// MetaTypeName represents a supported frontmatter value type.
type MetaTypeName string

const (
	MetaTypeString  MetaTypeName = "string"
	MetaTypeNumber  MetaTypeName = "number"
	MetaTypeDate    MetaTypeName = "date"
	MetaTypeSemver  MetaTypeName = "semver"
	MetaTypeOrdered MetaTypeName = "ordered"
)

// MetaTypeInfo holds the type declaration for a single frontmatter key.
type MetaTypeInfo struct {
	Name          MetaTypeName
	OrderedValues []string // only for MetaTypeOrdered
}

// UnmarshalYAML handles heterogeneous meta type values:
// scalar (e.g. "date") or mapping (e.g. {ordered: [low, high]}).
func (m *MetaTypeInfo) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		m.Name = MetaTypeName(value.Value)
		return nil
	case yaml.MappingNode:
		var raw map[string][]string
		if err := value.Decode(&raw); err != nil {
			return fmt.Errorf("meta type: %w", err)
		}
		if vals, ok := raw["ordered"]; ok {
			if len(raw) != 1 {
				return fmt.Errorf("meta type: unexpected extra keys alongside 'ordered'")
			}
			m.Name = MetaTypeOrdered
			m.OrderedValues = vals
			return nil
		}
		return fmt.Errorf("meta type: unknown mapping keys (expected 'ordered')")
	default:
		return fmt.Errorf("meta type: unsupported YAML node kind %d", value.Kind)
	}
}

// MetaConfig holds frontmatter metadata type declarations.
type MetaConfig struct {
	Types map[string]MetaTypeInfo `yaml:"types"`
	// Profiles lists required frontmatter keys by optional path glob.
	Profiles []MetaRequireProfile `yaml:"profiles"`
	// LinkKeys lists frontmatter keys whose raw path values become graph
	// edges with link type "frontmatter_path". URL values are skipped.
	LinkKeys []string `yaml:"link_keys"`
}

// MetaRequireProfile declares required frontmatter keys for an optional path glob.
type MetaRequireProfile struct {
	Path    string   `yaml:"path"`
	Require []string `yaml:"require"`
}

// LookupType returns the MetaTypeInfo for a given frontmatter key.
// If the key is not configured, returns MetaTypeInfo{Name: MetaTypeString} and false.
func (mc MetaConfig) LookupType(key string) (MetaTypeInfo, bool) {
	info, ok := mc.Types[key]
	if !ok {
		return MetaTypeInfo{Name: MetaTypeString}, false
	}
	return info, true
}

// Config represents the mdhop.yaml configuration file.
type Config struct {
	Build   BuildConfig   `yaml:"build"`
	Exclude ExcludeConfig `yaml:"exclude"`
	Meta    MetaConfig    `yaml:"meta"`
}

// BuildConfig holds build-time settings.
type BuildConfig struct {
	ExcludePaths []string `yaml:"exclude_paths"`
}

// ExcludeConfig holds exclusion patterns from the config file.
type ExcludeConfig struct {
	Paths []string `yaml:"paths"`
	Tags  []string `yaml:"tags"`
}

// LoadConfig reads mdhop.yaml from the vault root.
// Returns zero Config and nil error if the file does not exist.
func LoadConfig(vaultPath string) (Config, error) {
	p := filepath.Join(vaultPath, "mdhop.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("mdhop.yaml: %w", err)
	}
	if err := validateMetaConfig(cfg.Meta); err != nil {
		return Config{}, fmt.Errorf("mdhop.yaml: %w", err)
	}
	return cfg, nil
}

// validMetaTypeNames is the set of recognized meta type names.
var validMetaTypeNames = map[MetaTypeName]bool{
	MetaTypeString:  true,
	MetaTypeNumber:  true,
	MetaTypeDate:    true,
	MetaTypeSemver:  true,
	MetaTypeOrdered: true,
}

// validateMetaConfig checks meta type declarations for errors.
func validateMetaConfig(mc MetaConfig) error {
	for key, info := range mc.Types {
		if !validMetaTypeNames[info.Name] {
			return fmt.Errorf("meta.types.%s: unknown type %q", key, info.Name)
		}
		if info.Name == MetaTypeOrdered {
			if len(info.OrderedValues) == 0 {
				return fmt.Errorf("meta.types.%s: ordered type requires non-empty value list", key)
			}
			seen := make(map[string]bool, len(info.OrderedValues))
			for _, v := range info.OrderedValues {
				if seen[v] {
					return fmt.Errorf("meta.types.%s: duplicate ordered value %q", key, v)
				}
				seen[v] = true
			}
		}
	}
	for _, key := range mc.LinkKeys {
		if key == "tags" {
			return fmt.Errorf("meta.link_keys: %q is not allowed (tags are always parsed as tag links)", key)
		}
		if key == "" {
			return fmt.Errorf("meta.link_keys: empty key is not allowed")
		}
	}
	for i, profile := range mc.Profiles {
		if len(profile.Require) == 0 {
			return fmt.Errorf("meta.profiles[%d]: require must not be empty", i)
		}
		seenKeys := make(map[string]bool, len(profile.Require))
		for _, key := range profile.Require {
			if key == "" {
				return fmt.Errorf("meta.profiles[%d].require: empty key is not allowed", i)
			}
			if seenKeys[key] {
				return fmt.Errorf("meta.profiles[%d].require: duplicate key %q", i, key)
			}
			seenKeys[key] = true
		}
		if profile.Path != "" {
			if err := validateGlobPatterns([]string{profile.Path}); err != nil {
				return fmt.Errorf("meta.profiles[%d].path: %w", i, err)
			}
		}
	}
	return nil
}

// filterBuildExcludes removes files matching any of the given glob patterns.
func filterBuildExcludes(files []string, patterns []string) []string {
	if len(patterns) == 0 {
		return files
	}
	result := make([]string, 0, len(files))
	for _, f := range files {
		excluded := false
		for _, p := range patterns {
			if globMatch(p, f) {
				excluded = true
				break
			}
		}
		if !excluded {
			result = append(result, f)
		}
	}
	return result
}
