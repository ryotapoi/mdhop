package core

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
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

// MetaConfig holds frontmatter metadata type declarations.
type MetaConfig struct {
	// Type declarations are decoded separately because their values have mixed shapes.
	Types map[string]MetaTypeInfo `toml:"-"`
	// Profiles lists required frontmatter keys by optional path glob.
	Profiles []MetaRequireProfile `toml:"profiles"`
	// LinkKeys lists frontmatter keys whose raw path values become graph
	// edges with link type "frontmatter_path". URL values are skipped.
	LinkKeys []string `toml:"link_keys"`
}

// MetaRequireProfile declares required frontmatter keys for an optional path glob.
type MetaRequireProfile struct {
	Path    string   `toml:"path"`
	Require []string `toml:"require"`
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

// Config represents the mdhop.toml configuration file.
type Config struct {
	Build   BuildConfig   `toml:"build"`
	Exclude ExcludeConfig `toml:"exclude"`
	Query   QueryConfig   `toml:"query"`
	Meta    MetaConfig    `toml:"meta"`
}

// BuildConfig holds build-time settings.
type BuildConfig struct {
	ExcludePaths []string `toml:"exclude_paths"`
}

// ExcludeConfig holds exclusion patterns from the config file.
type ExcludeConfig struct {
	Paths []string `toml:"paths"`
	Tags  []string `toml:"tags"`
}

// QueryConfig separates display hiding from via selection.
type QueryConfig struct {
	Hide ExcludeConfig  `toml:"hide"`
	Via  QueryViaConfig `toml:"via"`
}

// QueryViaConfig uses nil Exclude only when the key is absent, allowing the
// top-level exclude config to supply the query via exclusion fallback.
type QueryViaConfig struct {
	Include ExcludeConfig  `toml:"include"`
	Exclude *ExcludeConfig `toml:"exclude"`
}

// LoadConfig reads the selected file, or mdhop.toml at the vault root.
// Only a missing default file is treated as an empty configuration.
func LoadConfig(vaultPath string, locations ...Locations) (Config, error) {
	location := selectedLocations(locations)
	p := location.ConfigFile(vaultPath)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) && location.ConfigPath == "" {
			return Config{}, nil
		}
		return Config{}, err
	}
	return decodeConfig(data, p)
}

func decodeConfig(data []byte, p string) (Config, error) {
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("%s: %w", p, err)
	}
	var raw struct {
		Meta struct {
			Types map[string]any `toml:"types"`
		} `toml:"meta"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("%s: %w", p, err)
	}
	if raw.Meta.Types != nil {
		cfg.Meta.Types = make(map[string]MetaTypeInfo, len(raw.Meta.Types))
		for key, value := range raw.Meta.Types {
			info, err := decodeMetaType(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s: meta.types.%s: %w", p, key, err)
			}
			cfg.Meta.Types[key] = info
		}
	}
	if err := validateMetaConfig(cfg.Meta); err != nil {
		return Config{}, fmt.Errorf("%s: %w", p, err)
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

// decodeMetaType accepts a type name or an ordered-value table.
func decodeMetaType(value any) (MetaTypeInfo, error) {
	if name, ok := value.(string); ok {
		return MetaTypeInfo{Name: MetaTypeName(name)}, nil
	}
	if table, ok := value.(map[string]any); ok {
		values, present := table["ordered"]
		if !present {
			return MetaTypeInfo{}, fmt.Errorf("unknown mapping keys (expected 'ordered')")
		}
		if len(table) != 1 {
			return MetaTypeInfo{}, fmt.Errorf("unexpected extra keys alongside 'ordered'")
		}
		array, ok := values.([]any)
		if !ok {
			return MetaTypeInfo{}, fmt.Errorf("ordered must be an array of strings")
		}
		info := MetaTypeInfo{Name: MetaTypeOrdered}
		for _, value := range array {
			text, ok := value.(string)
			if !ok {
				return MetaTypeInfo{}, fmt.Errorf("ordered must be an array of strings")
			}
			info.OrderedValues = append(info.OrderedValues, text)
		}
		return info, nil
	}
	return MetaTypeInfo{}, fmt.Errorf("expected type name or ordered table")
}
