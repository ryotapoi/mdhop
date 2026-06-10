package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	// LinkKeys lists frontmatter keys whose raw path values become graph
	// edges with link type "frontmatter_path". URL values are skipped.
	LinkKeys []string `yaml:"link_keys"`
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

// ExcludeFilter holds compiled exclusion conditions for query filtering.
// nil means no exclusion.
type ExcludeFilter struct {
	PathGlobs []string // SQLite GLOB patterns (case-sensitive)
	Tags      []string // lowercase, # prefixed
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

// NewExcludeFilter merges config and CLI exclusions into an ExcludeFilter.
// Returns nil if there are no exclusions.
func NewExcludeFilter(cfg ExcludeConfig, cliPaths, cliTags []string) (*ExcludeFilter, error) {
	paths := make([]string, 0, len(cfg.Paths)+len(cliPaths))
	paths = append(paths, cfg.Paths...)
	paths = append(paths, cliPaths...)
	tags := make([]string, 0, len(cfg.Tags)+len(cliTags))
	tags = append(tags, cfg.Tags...)
	tags = append(tags, cliTags...)

	if err := validateGlobPatterns(paths); err != nil {
		return nil, err
	}

	if len(paths) == 0 && len(tags) == 0 {
		return nil, nil
	}

	normalizedTags := make([]string, len(tags))
	for i, t := range tags {
		if !strings.HasPrefix(t, "#") {
			t = "#" + t
		}
		normalizedTags[i] = strings.ToLower(t)
	}

	return &ExcludeFilter{
		PathGlobs: paths,
		Tags:      normalizedTags,
	}, nil
}

// TagExcludeSQL returns a SQL fragment and args for excluding tags by name.
// alias is the column expression for the tag name (e.g. "n.name").
func (ef *ExcludeFilter) TagExcludeSQL(alias string) (string, []any) {
	if ef == nil || len(ef.Tags) == 0 {
		return "", nil
	}
	placeholders := make([]string, len(ef.Tags))
	args := make([]any, len(ef.Tags))
	for i, t := range ef.Tags {
		placeholders[i] = "?"
		args[i] = t
	}
	return fmt.Sprintf(" AND LOWER(%s) NOT IN (%s)", alias, strings.Join(placeholders, ",")), args
}

// IsViaExcluded checks if a via node should be excluded from twohop results.
func (ef *ExcludeFilter) IsViaExcluded(info NodeInfo) bool {
	if ef == nil {
		return false
	}
	switch info.Type {
	case NodeTypeTag:
		lower := strings.ToLower(info.Name)
		for _, t := range ef.Tags {
			if t == lower {
				return true
			}
		}
	case NodeTypeNote, NodeTypeAsset:
		for _, g := range ef.PathGlobs {
			if globMatch(g, info.Path) {
				return true
			}
		}
	}
	return false
}
