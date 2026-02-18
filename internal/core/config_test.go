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
