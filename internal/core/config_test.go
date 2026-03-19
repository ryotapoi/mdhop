package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_NotFound(t *testing.T) {
	cfg, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Exclude.Paths) != 0 || len(cfg.Exclude.Tags) != 0 {
		t.Errorf("expected zero config, got %+v", cfg)
	}
}

func TestLoadConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	content := `exclude:
  paths:
    - "daily/*"
    - "templates/*"
  tags:
    - "#daily"
    - "#template"
`
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Exclude.Paths) != 2 {
		t.Errorf("paths = %v, want 2 items", cfg.Exclude.Paths)
	}
	if len(cfg.Exclude.Tags) != 2 {
		t.Errorf("tags = %v, want 2 items", cfg.Exclude.Tags)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(":::invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadConfig_Empty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Exclude.Paths) != 0 || len(cfg.Exclude.Tags) != 0 {
		t.Errorf("expected zero config, got %+v", cfg)
	}
}

func TestNewExcludeFilter_MergeConfigAndCLI(t *testing.T) {
	cfg := ExcludeConfig{
		Paths: []string{"daily/*"},
		Tags:  []string{"#daily"},
	}
	ef, err := NewExcludeFilter(cfg, []string{"templates/*"}, []string{"template"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ef == nil {
		t.Fatal("expected non-nil filter")
	}
	if len(ef.PathGlobs) != 2 {
		t.Errorf("PathGlobs = %v, want 2 items", ef.PathGlobs)
	}
	if len(ef.Tags) != 2 {
		t.Errorf("Tags = %v, want 2 items", ef.Tags)
	}
}

func TestNewExcludeFilter_NilWhenEmpty(t *testing.T) {
	ef, err := NewExcludeFilter(ExcludeConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ef != nil {
		t.Errorf("expected nil, got %+v", ef)
	}
}

func TestNewExcludeFilter_TagNormalization(t *testing.T) {
	ef, err := NewExcludeFilter(ExcludeConfig{}, nil, []string{"Daily", "#TEMPLATE"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ef.Tags[0] != "#daily" {
		t.Errorf("Tags[0] = %q, want %q", ef.Tags[0], "#daily")
	}
	if ef.Tags[1] != "#template" {
		t.Errorf("Tags[1] = %q, want %q", ef.Tags[1], "#template")
	}
}

func TestNewExcludeFilter_BracketPatternError(t *testing.T) {
	_, err := NewExcludeFilter(ExcludeConfig{}, []string{"[abc]/*"}, nil)
	if err == nil {
		t.Fatal("expected error for bracket pattern")
	}
}

func TestPathExcludeSQL(t *testing.T) {
	ef := &ExcludeFilter{PathGlobs: []string{"daily/*", "templates/*"}}
	sql, args := ef.PathExcludeSQL("n.path")
	if sql == "" {
		t.Fatal("expected non-empty SQL")
	}
	if len(args) != 2 {
		t.Errorf("args = %v, want 2 items", args)
	}
}

func TestPathExcludeSQL_Nil(t *testing.T) {
	var ef *ExcludeFilter
	sql, args := ef.PathExcludeSQL("n.path")
	if sql != "" || args != nil {
		t.Errorf("expected empty, got sql=%q args=%v", sql, args)
	}
}

func TestTagExcludeSQL(t *testing.T) {
	ef := &ExcludeFilter{Tags: []string{"#daily", "#template"}}
	sql, args := ef.TagExcludeSQL("n.name")
	if sql == "" {
		t.Fatal("expected non-empty SQL")
	}
	if len(args) != 2 {
		t.Errorf("args = %v, want 2 items", args)
	}
}

func TestTagExcludeSQL_Nil(t *testing.T) {
	var ef *ExcludeFilter
	sql, args := ef.TagExcludeSQL("n.name")
	if sql != "" || args != nil {
		t.Errorf("expected empty, got sql=%q args=%v", sql, args)
	}
}

func TestIsViaExcluded(t *testing.T) {
	ef := &ExcludeFilter{
		PathGlobs: []string{"daily/*"},
		Tags:      []string{"#daily"},
	}

	tests := []struct {
		name string
		info NodeInfo
		want bool
	}{
		{"tag excluded", NodeInfo{Type: "tag", Name: "#daily"}, true},
		{"tag not excluded", NodeInfo{Type: "tag", Name: "#project"}, false},
		{"tag case-insensitive", NodeInfo{Type: "tag", Name: "#Daily"}, true},
		{"note excluded", NodeInfo{Type: "note", Name: "D", Path: "daily/D.md"}, true},
		{"note not excluded", NodeInfo{Type: "note", Name: "A", Path: "A.md"}, false},
		{"phantom never excluded", NodeInfo{Type: "phantom", Name: "Missing"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ef.IsViaExcluded(tt.info)
			if got != tt.want {
				t.Errorf("IsViaExcluded(%+v) = %v, want %v", tt.info, got, tt.want)
			}
		})
	}
}

func TestIsViaExcluded_Nil(t *testing.T) {
	var ef *ExcludeFilter
	if ef.IsViaExcluded(NodeInfo{Type: "note", Path: "daily/D.md"}) {
		t.Error("nil filter should not exclude anything")
	}
}

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		want    bool
	}{
		{"Daily/*", "Daily/2024.md", true},
		{"Daily/*", "Daily/sub/x.md", true},
		{"Daily/*", "Other/x.md", false},
		{"Daily/*", "daily/2024.md", false}, // case-sensitive
		{"*", "anything", true},
		{"*", "", true},
		{"?", "a", true},
		{"?", "", false},
		{"?", "ab", false},
		{"a*b", "ab", true},
		{"a*b", "axyzb", true},
		{"a*b", "axyzc", false},
		{"*.md", "test.md", true},
		{"*.md", "dir/test.md", true},
		{"exact", "exact", true},
		{"exact", "exactx", false},
		{"exact", "xexact", false},
		{"[literal", "[literal", true}, // '[' treated as literal
		{"a?c", "abc", true},
		{"a?c", "ac", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.s, func(t *testing.T) {
			got := globMatch(tt.pattern, tt.s)
			if got != tt.want {
				t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.s, got, tt.want)
			}
		})
	}
}

func TestLoadConfig_WithBuildSection(t *testing.T) {
	dir := t.TempDir()
	content := `build:
  exclude_paths:
    - "daily/*"
    - "templates/*"
exclude:
  paths:
    - "archive/*"
`
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Build.ExcludePaths) != 2 {
		t.Errorf("build.exclude_paths = %v, want 2 items", cfg.Build.ExcludePaths)
	}
	if cfg.Build.ExcludePaths[0] != "daily/*" {
		t.Errorf("build.exclude_paths[0] = %q, want %q", cfg.Build.ExcludePaths[0], "daily/*")
	}
	// Ensure query exclude is also parsed.
	if len(cfg.Exclude.Paths) != 1 {
		t.Errorf("exclude.paths = %v, want 1 item", cfg.Exclude.Paths)
	}
}

func TestValidateGlobPatterns(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		wantErr  bool
	}{
		{"valid patterns", []string{"daily/*", "templates/*"}, false},
		{"empty", []string{}, false},
		{"nil", nil, false},
		{"bracket error", []string{"[abc]/*"}, true},
		{"mixed valid and bracket", []string{"daily/*", "[x]"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGlobPatterns(tt.patterns)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGlobPatterns(%v) error = %v, wantErr %v", tt.patterns, err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig_MetaTypesSimple(t *testing.T) {
	dir := t.TempDir()
	content := "meta:\n  types:\n    date: date\n    priority: number\n"
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Meta.Types["date"].Name != MetaTypeDate {
		t.Errorf("date type = %q, want %q", cfg.Meta.Types["date"].Name, MetaTypeDate)
	}
	if cfg.Meta.Types["priority"].Name != MetaTypeNumber {
		t.Errorf("priority type = %q, want %q", cfg.Meta.Types["priority"].Name, MetaTypeNumber)
	}
}

func TestLoadConfig_MetaTypesEmpty(t *testing.T) {
	dir := t.TempDir()
	// No meta section at all
	content := "exclude:\n  paths:\n    - \"daily/*\"\n"
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Meta.Types != nil {
		t.Errorf("expected nil Types, got %v", cfg.Meta.Types)
	}
}

func TestLoadConfig_MetaTypesOrdered(t *testing.T) {
	dir := t.TempDir()
	content := "meta:\n  types:\n    severity:\n      ordered: [low, middle, high, critical]\n"
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := cfg.Meta.Types["severity"]
	if info.Name != MetaTypeOrdered {
		t.Errorf("Name = %q, want %q", info.Name, MetaTypeOrdered)
	}
	want := []string{"low", "middle", "high", "critical"}
	if len(info.OrderedValues) != len(want) {
		t.Fatalf("OrderedValues len = %d, want %d", len(info.OrderedValues), len(want))
	}
	for i, v := range want {
		if info.OrderedValues[i] != v {
			t.Errorf("OrderedValues[%d] = %q, want %q", i, info.OrderedValues[i], v)
		}
	}
}

func TestLoadConfig_MetaTypesUnknown(t *testing.T) {
	dir := t.TempDir()
	content := "meta:\n  types:\n    foo: unknown_type\n"
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestLoadConfig_MetaTypesOrderedEmpty(t *testing.T) {
	dir := t.TempDir()
	content := "meta:\n  types:\n    severity:\n      ordered: []\n"
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for empty ordered list")
	}
}

func TestLoadConfig_MetaTypesOrderedDuplicate(t *testing.T) {
	dir := t.TempDir()
	content := "meta:\n  types:\n    severity:\n      ordered: [low, high, low]\n"
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for duplicate ordered values")
	}
}

func TestLoadConfig_MetaTypesOrderedExtraKeys(t *testing.T) {
	dir := t.TempDir()
	content := "meta:\n  types:\n    severity:\n      ordered: [low, high]\n      extra: [a]\n"
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for extra keys alongside ordered")
	}
}

func TestLoadConfig_MetaTypesMixed(t *testing.T) {
	dir := t.TempDir()
	content := `meta:
  types:
    created: date
    priority: number
    version: semver
    title: string
    severity:
      ordered: [low, middle, high, critical]
`
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tests := []struct {
		key  string
		want MetaTypeName
	}{
		{"created", MetaTypeDate},
		{"priority", MetaTypeNumber},
		{"version", MetaTypeSemver},
		{"title", MetaTypeString},
		{"severity", MetaTypeOrdered},
	}
	for _, tt := range tests {
		info, ok := cfg.Meta.Types[tt.key]
		if !ok {
			t.Errorf("key %q not found", tt.key)
			continue
		}
		if info.Name != tt.want {
			t.Errorf("key %q: Name = %q, want %q", tt.key, info.Name, tt.want)
		}
	}
	if len(cfg.Meta.Types["severity"].OrderedValues) != 4 {
		t.Errorf("severity OrderedValues len = %d, want 4", len(cfg.Meta.Types["severity"].OrderedValues))
	}
}

func TestLookupMetaType(t *testing.T) {
	mc := MetaConfig{
		Types: map[string]MetaTypeInfo{
			"date":     {Name: MetaTypeDate},
			"priority": {Name: MetaTypeNumber},
		},
	}
	// Existing key
	info, ok := mc.LookupType("date")
	if !ok {
		t.Error("expected ok=true for existing key")
	}
	if info.Name != MetaTypeDate {
		t.Errorf("Name = %q, want %q", info.Name, MetaTypeDate)
	}
	// Missing key
	info, ok = mc.LookupType("missing")
	if ok {
		t.Error("expected ok=false for missing key")
	}
	if info.Name != MetaTypeString {
		t.Errorf("Name = %q, want %q for missing key", info.Name, MetaTypeString)
	}
	// nil Types map
	var empty MetaConfig
	info, ok = empty.LookupType("anything")
	if ok {
		t.Error("expected ok=false for nil Types")
	}
	if info.Name != MetaTypeString {
		t.Errorf("Name = %q, want %q for nil Types", info.Name, MetaTypeString)
	}
}

func TestFilterBuildExcludes(t *testing.T) {
	files := []string{"A.md", "B.md", "daily/D.md", "daily/E.md", "templates/T.md"}
	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{"no patterns", nil, []string{"A.md", "B.md", "daily/D.md", "daily/E.md", "templates/T.md"}},
		{"empty patterns", []string{}, []string{"A.md", "B.md", "daily/D.md", "daily/E.md", "templates/T.md"}},
		{"single pattern", []string{"daily/*"}, []string{"A.md", "B.md", "templates/T.md"}},
		{"multiple patterns", []string{"daily/*", "templates/*"}, []string{"A.md", "B.md"}},
		{"case sensitive", []string{"Daily/*"}, []string{"A.md", "B.md", "daily/D.md", "daily/E.md", "templates/T.md"}},
		{"no match", []string{"archive/*"}, []string{"A.md", "B.md", "daily/D.md", "daily/E.md", "templates/T.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterBuildExcludes(files, tt.patterns)
			if len(got) != len(tt.want) {
				t.Fatalf("filterBuildExcludes len = %d, want %d (got %v)", len(got), len(tt.want), got)
			}
			for i, f := range got {
				if f != tt.want[i] {
					t.Errorf("filterBuildExcludes[%d] = %q, want %q", i, f, tt.want[i])
				}
			}
		})
	}
}
