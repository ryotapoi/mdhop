package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the mdhop.yaml configuration file.
type Config struct {
	Build   BuildConfig   `yaml:"build"`
	Exclude ExcludeConfig `yaml:"exclude"`
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
	return cfg, nil
}

// validateGlobPatterns checks that none of the patterns use unsupported character classes.
func validateGlobPatterns(patterns []string) error {
	for _, p := range patterns {
		if strings.Contains(p, "[") {
			return fmt.Errorf("unsupported glob pattern (character class): %s", p)
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

// PathExcludeSQL returns a SQL fragment and args for excluding paths.
// alias is the column expression for path (e.g. "n.path").
func (ef *ExcludeFilter) PathExcludeSQL(alias string) (string, []any) {
	if ef == nil || len(ef.PathGlobs) == 0 {
		return "", nil
	}
	var parts []string
	var args []any
	for _, g := range ef.PathGlobs {
		parts = append(parts, alias+" GLOB ?")
		args = append(args, g)
	}
	// path IS NULL protects phantom/tag nodes (NOT (NULL GLOB ?) → NULL → false in WHERE).
	return fmt.Sprintf(" AND (%s IS NULL OR NOT (%s))", alias, strings.Join(parts, " OR ")), args
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
	case "tag":
		lower := strings.ToLower(info.Name)
		for _, t := range ef.Tags {
			if t == lower {
				return true
			}
		}
	case "note":
		for _, g := range ef.PathGlobs {
			if globMatch(g, info.Path) {
				return true
			}
		}
	}
	return false
}

// globMatch implements SQLite GLOB semantics in Go.
// '*' matches any sequence of characters (including '/').
// '?' matches exactly one character.
// '[' is treated as a literal character (character classes not supported).
func globMatch(pattern, s string) bool {
	return globMatchImpl([]rune(pattern), []rune(s))
}

func globMatchImpl(pattern, s []rune) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// Skip consecutive '*'.
			for len(pattern) > 0 && pattern[0] == '*' {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true
			}
			// Try matching the rest of the pattern at every position.
			for i := 0; i <= len(s); i++ {
				if globMatchImpl(pattern, s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(s) == 0 {
				return false
			}
			pattern = pattern[1:]
			s = s[1:]
		default:
			if len(s) == 0 || pattern[0] != s[0] {
				return false
			}
			pattern = pattern[1:]
			s = s[1:]
		}
	}
	return len(s) == 0
}
