package core

import (
	"os"
	"path/filepath"
	"testing"
)

func loadQueryTestConfig(t *testing.T, text string) (Config, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return LoadConfig(dir)
}

func TestQueryConfigFallback(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		allowed     bool
	}{
		{"missing", "", false},
		{"include only", "query:\n  via:\n    include:\n      paths: ['archive/*']\n", false},
		{"empty mapping", "query:\n  via:\n    exclude: {}\n", true},
		{"empty lists", "query:\n  via:\n    exclude: {paths: [], tags: []}\n", true},
		{"replacement", "query:\n  via:\n    exclude: {paths: ['other/*']}\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadQueryTestConfig(t, "exclude: {paths: ['archive/*']}\n"+tc.query)
			if err != nil {
				t.Fatal(err)
			}
			f, err := NewQueryFilter(cfg, QueryFilterOptions{})
			if err != nil {
				t.Fatal(err)
			}
			note := NodeInfo{Type: NodeTypeNote, Path: "archive/a.md"}
			if got := f.AllowsVia(note); got != tc.allowed {
				t.Fatalf("AllowsVia=%v want %v", got, tc.allowed)
			}
			if f.IsHidden(note) {
				t.Fatal("legacy exclusion must not hide")
			}
			legacy, err := NewExcludeFilter(cfg.Exclude, nil, nil)
			if err != nil || !legacy.IsViaExcluded(note) {
				t.Fatalf("legacy search exclusions changed: %v", err)
			}
		})
	}
	for _, text := range []string{"exclude: null", "exclude: &empty null", "<<: {exclude: null}", "exclude: bad", "exclude: []", "include: {paths: bad}"} {
		t.Run(text, func(t *testing.T) {
			if _, err := loadQueryTestConfig(t, "query:\n  via:\n    "+text+"\n"); err == nil {
				t.Fatal("expected structural error")
			}
		})
	}
}

func TestQueryFilterComposition(t *testing.T) {
	cfg := Config{Exclude: ExcludeConfig{Paths: []string{"legacy/*"}}, Query: QueryConfig{
		Hide: ExcludeConfig{Paths: []string{"hidden/*"}, Tags: []string{"Private"}},
		Via:  QueryViaConfig{Include: ExcludeConfig{Paths: []string{"topics/*"}}, Exclude: &ExcludeConfig{Tags: []string{"blocked"}}},
	}}
	opts := QueryFilterOptions{Hide: ExcludeConfig{Tags: []string{"caller"}}, ViaInclude: ExcludeConfig{Tags: []string{"Private", "blocked"}}, ViaExclude: ExcludeConfig{Paths: []string{"topics/secret.md"}}}
	f, err := NewQueryFilter(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		node            NodeInfo
		hidden, allowed bool
	}{
		{NodeInfo{Type: NodeTypeNote, Path: "topics/open.md"}, false, true},
		{NodeInfo{Type: NodeTypeAsset, Path: "topics/image.png"}, false, true},
		{NodeInfo{Type: NodeTypeNote, Path: "topics/secret.md"}, false, false},
		{NodeInfo{Type: NodeTypeTag, Name: "#PRIVATE"}, true, true},
		{NodeInfo{Type: NodeTypeTag, Name: "#blocked"}, false, false},
		{NodeInfo{Type: NodeTypeTag, Name: "#caller"}, true, false},
		{NodeInfo{Type: NodeTypeNote, Path: "hidden/a.md"}, true, false},
		{NodeInfo{Type: NodeTypeAsset, Path: "hidden/a.md"}, false, false},
		{NodeInfo{Type: NodeTypePhantom, Name: "topics/a.md"}, false, false},
		{NodeInfo{Type: NodeTypeNote, Path: "other.md", Name: "#PRIVATE"}, false, false},
	} {
		if f.IsHidden(tc.node) != tc.hidden || f.AllowsVia(tc.node) != tc.allowed {
			t.Errorf("%+v: hidden=%v allowed=%v", tc.node, f.IsHidden(tc.node), f.AllowsVia(tc.node))
		}
	}
	opts.NoConfigHide = true
	f, err = NewQueryFilter(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if f.IsHidden(NodeInfo{Type: NodeTypeTag, Name: "#private"}) || !f.IsHidden(NodeInfo{Type: NodeTypeTag, Name: "#caller"}) || f.AllowsVia(NodeInfo{Type: NodeTypeNote, Path: "other.md"}) {
		t.Fatal("no-config-hide must affect only configured hide")
	}
	opts.NoConfigHide = false
	opts.NoConfigVia = true
	f, err = NewQueryFilter(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !f.IsHidden(NodeInfo{Type: NodeTypeTag, Name: "#private"}) || f.AllowsVia(NodeInfo{Type: NodeTypeNote, Path: "topics/open.md"}) || !f.AllowsVia(NodeInfo{Type: NodeTypeTag, Name: "#blocked"}) {
		t.Fatal("no-config-via must preserve caller include and hide")
	}
	cfg.Query.Via = QueryViaConfig{}
	f, err = NewQueryFilter(cfg, QueryFilterOptions{NoConfigVia: true, ViaExclude: ExcludeConfig{Tags: []string{"caller"}}})
	if err != nil || !f.AllowsVia(NodeInfo{Type: NodeTypeNote, Path: "legacy/a.md"}) || f.AllowsVia(NodeInfo{Type: NodeTypeTag, Name: "#caller"}) {
		t.Fatalf("no-config-via fallback/caller: %v", err)
	}
}

func TestQueryFilterGlob(t *testing.T) {
	for _, tc := range []struct {
		pattern, path string
		want          bool
	}{
		{"archive/*", "archive/deep/a.md", true},
		{"archive/", "archive/deep/a.md", false},
		{"a?.md", "a界.md", true},
		{"a?.md", "abc.md", false},
		{"Archive/*", "archive/a.md", false},
		{"./cafe\u0301/*", "café/a.md", true},
		{"a,b.md", "a,b.md", true},
		{"a,b.md", "a.md", false},
	} {
		f, err := NewQueryFilter(Config{}, QueryFilterOptions{ViaInclude: ExcludeConfig{Paths: []string{tc.pattern}}})
		if err != nil {
			t.Fatal(err)
		}
		if got := f.AllowsVia(NodeInfo{Type: NodeTypeNote, Path: tc.path}); got != tc.want {
			t.Errorf("%q %q got %v", tc.pattern, tc.path, got)
		}
	}
	for _, opts := range []QueryFilterOptions{
		{Hide: ExcludeConfig{Paths: []string{"[a]"}}},
		{ViaInclude: ExcludeConfig{Paths: []string{"[a]"}}},
		{ViaExclude: ExcludeConfig{Paths: []string{"[a]"}}},
	} {
		if _, err := NewQueryFilter(Config{}, opts); err == nil {
			t.Fatal("character class must fail")
		}
	}
	cfg, err := loadQueryTestConfig(t, "query:\n  hide: {paths: ['[a]']}\n  via:\n    include: {paths: ['[b]']}\n    exclude: {paths: ['[c]']}\n")
	if err != nil {
		t.Fatalf("semantic validation leaked into config/search: %v", err)
	}
	if _, err := NewQueryFilter(cfg, QueryFilterOptions{}); err == nil {
		t.Fatal("enabled invalid config must fail")
	}
	if _, err := NewQueryFilter(cfg, QueryFilterOptions{NoConfigHide: true, NoConfigVia: true}); err != nil {
		t.Fatalf("disabled config must not validate: %v", err)
	}
}

func TestTypedVia(t *testing.T) {
	for _, tc := range []struct {
		value       string
		match, miss NodeInfo
	}{
		{"note:./cafe\u0301/a*?[],b:c.md", NodeInfo{Type: NodeTypeNote, Path: "café/a*?[],b:c.md"}, NodeInfo{Type: NodeTypeNote, Path: "café/axz,b:c.md"}},
		{"asset:images/A.png", NodeInfo{Type: NodeTypeAsset, Path: "images/A.png"}, NodeInfo{Type: NodeTypeNote, Path: "images/A.png"}},
		{"phantom:Cafe\u0301:*,?[]", NodeInfo{Type: NodeTypePhantom, Name: "CAFÉ:*,?[]"}, NodeInfo{Type: NodeTypePhantom, Name: "Café:abc"}},
		{"tag:Cafe\u0301,plan:x", NodeInfo{Type: NodeTypeTag, Name: "#CAFÉ,PLAN:X"}, NodeInfo{Type: NodeTypeTag, Name: "#café"}},
	} {
		v, err := ParseTypedVia([]string{tc.value})
		if err != nil {
			t.Fatal(err)
		}
		if !v.Matches(tc.match) || v.Matches(tc.miss) {
			t.Errorf("%q strict match failed", tc.value)
		}
	}
	v, _ := ParseTypedVia([]string{"note:dir/A.md"})
	for _, path := range []string{"A.md", "dir/a.md"} {
		if v.Matches(NodeInfo{Type: NodeTypeNote, Path: path}) {
			t.Errorf("unexpected fallback: %s", path)
		}
	}
	for _, values := range [][]string{{""}, {"note:"}, {"Note:a"}, {"other:a"}, {"note"}, {"note:a", "note:b"}} {
		if _, err := ParseTypedVia(values); err == nil {
			t.Errorf("expected error: %q", values)
		}
	}
	if v, err := ParseTypedVia(nil); err != nil || v != nil {
		t.Fatal("absent via must allow all")
	}
	f, err := NewQueryFilter(Config{}, QueryFilterOptions{Via: []string{"note:topics/a.md"}, ViaInclude: ExcludeConfig{Paths: []string{"other/*"}}})
	if err != nil || f.AllowsVia(NodeInfo{Type: NodeTypeNote, Path: "topics/a.md"}) {
		t.Fatal("typed via must intersect include")
	}
	f, err = NewQueryFilter(Config{}, QueryFilterOptions{Via: []string{"note:topics/a.md"}, ViaInclude: ExcludeConfig{Paths: []string{"topics/*"}}, ViaExclude: ExcludeConfig{Paths: []string{"topics/a.md"}}})
	if err != nil || f.AllowsVia(NodeInfo{Type: NodeTypeNote, Path: "topics/a.md"}) {
		t.Fatal("exclude must override typed via")
	}
}
