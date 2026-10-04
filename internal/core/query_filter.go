package core

import (
	"fmt"
	"strings"
)

// QueryFilterOptions holds caller conditions, independently added to config.
// Via accepts at most one typed identifier; it does not interpret GLOB syntax.
type QueryFilterOptions struct {
	Hide         ExcludeConfig
	ViaInclude   ExcludeConfig
	ViaExclude   ExcludeConfig
	NoConfigHide bool
	NoConfigVia  bool
	Via          []string
}

// QueryFilter keeps hiding independent from via eligibility.
type QueryFilter struct {
	hide    *ExcludeFilter
	include *ExcludeFilter
	exclude *ExcludeFilter
	via     *TypedVia
}

// NewQueryFilter resolves query config, including the legacy exclude fallback.
// Semantic validation applies only to enabled conditions.
func NewQueryFilter(cfg Config, opts QueryFilterOptions) (*QueryFilter, error) {
	var hide, include, exclude ExcludeConfig
	if !opts.NoConfigHide {
		hide = cfg.Query.Hide
	}
	if !opts.NoConfigVia {
		include = cfg.Query.Via.Include
		exclude = cfg.Exclude
		if cfg.Query.Via.Exclude != nil {
			exclude = *cfg.Query.Via.Exclude
		}
	}
	h, err := newQueryConditions(hide, opts.Hide)
	if err != nil {
		return nil, fmt.Errorf("query hide: %w", err)
	}
	i, err := newQueryConditions(include, opts.ViaInclude)
	if err != nil {
		return nil, fmt.Errorf("query via include: %w", err)
	}
	e, err := newQueryConditions(exclude, opts.ViaExclude)
	if err != nil {
		return nil, fmt.Errorf("query via exclude: %w", err)
	}
	via, err := ParseTypedVia(opts.Via)
	if err != nil {
		return nil, err
	}
	return &QueryFilter{hide: h, include: i, exclude: e, via: via}, nil
}

func newQueryConditions(cfg, caller ExcludeConfig) (*ExcludeFilter, error) {
	f, err := NewExcludeFilter(cfg, caller.Paths, caller.Tags)
	if err != nil || f == nil {
		return f, err
	}
	for i, p := range f.PathGlobs {
		f.PathGlobs[i] = NormalizePath(p)
	}
	for i, t := range f.Tags {
		f.Tags[i] = normalizeTextNFC(t)
	}
	return f, nil
}

// IsHidden hides only note paths and tag names; assets and phantoms remain visible.
func (f *QueryFilter) IsHidden(info NodeInfo) bool {
	if f == nil || (info.Type != NodeTypeNote && info.Type != NodeTypeTag) {
		return false
	}
	return queryConditionsMatch(f.hide, info)
}

// AllowsVia intersects typed selection and inclusion, with exclusion taking precedence.
func (f *QueryFilter) AllowsVia(info NodeInfo) bool {
	if f == nil {
		return true
	}
	if f.via != nil && !f.via.Matches(info) {
		return false
	}
	return (f.include == nil || queryConditionsMatch(f.include, info)) && !queryConditionsMatch(f.exclude, info)
}

func queryConditionsMatch(f *ExcludeFilter, info NodeInfo) bool {
	if f == nil {
		return false
	}
	switch info.Type {
	case NodeTypeNote, NodeTypeAsset:
		info.Path = NormalizePath(info.Path)
	case NodeTypeTag:
		info.Name = normalizeQueryTag(info.Name)
	}
	return f.IsViaExcluded(info)
}

func normalizeQueryTag(name string) string {
	if !strings.HasPrefix(name, "#") {
		name = "#" + name
	}
	return normalizeTextNFC(name)
}

// TypedVia identifies exactly one node by its type and normalized key.
// Parsing and matching do not check whether the node is registered in the index.
type TypedVia struct {
	Type NodeType
	key  string
}

// ParseTypedVia parses zero or one type:identifier values, preserving punctuation.
func ParseTypedVia(values []string) (*TypedVia, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("via: only one typed identifier is allowed")
	}
	typ, identifier, ok := strings.Cut(values[0], ":")
	if !ok || identifier == "" {
		return nil, fmt.Errorf("via: expected type:identifier with a non-empty identifier")
	}
	nodeType := NodeType(typ)
	key, ok := typedViaKey(nodeType, identifier)
	if !ok {
		return nil, fmt.Errorf("via: unsupported type %q", typ)
	}
	return &TypedVia{Type: nodeType, key: key}, nil
}

func typedViaKey(typ NodeType, identifier string) (string, bool) {
	switch typ {
	case NodeTypeNote:
		return noteKey(identifier), true
	case NodeTypeAsset:
		return assetKey(identifier), true
	case NodeTypePhantom:
		return phantomKey(identifier), true
	case NodeTypeTag:
		return tagKey(normalizeQueryTag(identifier)), true
	default:
		return "", false
	}
}

// Matches requires the same type and full identifier, without basename fallback.
func (v *TypedVia) Matches(info NodeInfo) bool {
	if v == nil {
		return true
	}
	if v.Type != info.Type {
		return false
	}
	identifier := info.Name
	if info.Type == NodeTypeNote || info.Type == NodeTypeAsset {
		identifier = info.Path
	}
	key, ok := typedViaKey(info.Type, identifier)
	return ok && key == v.key
}
