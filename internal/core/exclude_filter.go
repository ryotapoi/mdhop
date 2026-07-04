package core

import (
	"fmt"
	"strings"
)

// ExcludeFilter holds compiled exclusion conditions for query filtering.
// nil means no exclusion.
type ExcludeFilter struct {
	PathGlobs []string // SQLite GLOB patterns (case-sensitive)
	Tags      []string // lowercase, # prefixed
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
	globs, args := globOrSQL(alias, ef.PathGlobs)
	// path IS NULL protects phantom/tag nodes (NOT (NULL GLOB ?) -> NULL -> false in WHERE).
	return fmt.Sprintf(" AND (%s IS NULL OR NOT (%s))", alias, globs), args
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
