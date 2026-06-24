package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLooksLikeDate(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"2024-01-15", true},
		{"2024-1-5", true},
		{"2024/01/15", true},
		{"2024-01-02T15:04:05", true},
		{"2024-01-02T15:04:05Z", true},
		{"2024-01-02T15:04:05+09:00", true},
		{"not-a-date", false},
		{"42", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := looksLikeDate(tt.input); got != tt.want {
			t.Errorf("looksLikeDate(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLooksLikeNumber(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"42", true},
		{"-3.14", true},
		{"+10", true},
		{"0", true},
		{"1.0", true},
		{"abc", false},
		{"1.2.3", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := looksLikeNumber(tt.input); got != tt.want {
			t.Errorf("looksLikeNumber(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLooksLikeSemver(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1.2.3", true},
		{"v1.2.3", true},
		{"1.0", true},
		{"v2.0", true},
		{"42", false},
		{"abc", false},
		{"1.2.3-beta", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := looksLikeSemver(tt.input); got != tt.want {
			t.Errorf("looksLikeSemver(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestPresetMetaTypes(t *testing.T) {
	presets := presetMetaTypes()

	if len(presets) != 15 {
		t.Fatalf("expected 15 preset entries, got %d", len(presets))
	}

	// Verify first and last entries
	if presets[0].Key != "date" || presets[0].Info.Name != MetaTypeDate {
		t.Errorf("first entry: %+v", presets[0])
	}
	if presets[14].Key != "version" || presets[14].Info.Name != MetaTypeSemver {
		t.Errorf("last entry: %+v", presets[14])
	}

	// Verify no duplicates
	seen := make(map[string]bool)
	for _, p := range presets {
		if seen[p.Key] {
			t.Errorf("duplicate key: %s", p.Key)
		}
		seen[p.Key] = true
	}

	// Count by type
	counts := make(map[MetaTypeName]int)
	for _, p := range presets {
		counts[p.Info.Name]++
	}
	if counts[MetaTypeDate] != 10 {
		t.Errorf("date count: got %d, want 10", counts[MetaTypeDate])
	}
	if counts[MetaTypeNumber] != 4 {
		t.Errorf("number count: got %d, want 4", counts[MetaTypeNumber])
	}
	if counts[MetaTypeSemver] != 1 {
		t.Errorf("semver count: got %d, want 1", counts[MetaTypeSemver])
	}
}

func TestScanMetaTypesResolvesNFCPathToNFDPath(t *testing.T) {
	vault := t.TempDir()
	nfdPath := "Cafe\u0301.md"
	if err := os.WriteFile(filepath.Join(vault, nfdPath), []byte("---\ncreated: 2024-01-15\n---\n# Cafe\n"), 0o644); err != nil {
		t.Fatalf("write NFD note: %v", err)
	}

	inferred, err := scanMetaTypes(vault, Config{})
	if err != nil {
		t.Fatalf("scanMetaTypes: %v", err)
	}
	got, ok := inferred["created"]
	if !ok {
		t.Fatalf("created not inferred; got %+v", inferred)
	}
	if got.InferredType != MetaTypeDate {
		t.Fatalf("created type = %q, want %q", got.InferredType, MetaTypeDate)
	}
}

func TestMergeMetaConfig(t *testing.T) {
	t.Run("all new keys", func(t *testing.T) {
		existing := map[string]MetaTypeInfo{}
		proposed := map[string]MetaTypeInfo{
			"date":     {Name: MetaTypeDate},
			"priority": {Name: MetaTypeNumber},
		}
		merged, added, skipped := mergeMetaConfig(existing, proposed)
		if len(merged) != 2 {
			t.Fatalf("merged len: %d", len(merged))
		}
		sort.Strings(added)
		if len(added) != 2 || added[0] != "date" || added[1] != "priority" {
			t.Errorf("added: %v", added)
		}
		if len(skipped) != 0 {
			t.Errorf("skipped: %v", skipped)
		}
	})

	t.Run("existing keys preserved", func(t *testing.T) {
		existing := map[string]MetaTypeInfo{
			"date": {Name: MetaTypeString}, // user chose string explicitly
		}
		proposed := map[string]MetaTypeInfo{
			"date":     {Name: MetaTypeDate},
			"priority": {Name: MetaTypeNumber},
		}
		merged, added, skipped := mergeMetaConfig(existing, proposed)
		if merged["date"].Name != MetaTypeString {
			t.Errorf("existing date should be preserved, got %q", merged["date"].Name)
		}
		if len(added) != 1 || added[0] != "priority" {
			t.Errorf("added: %v", added)
		}
		if len(skipped) != 1 || skipped[0] != "date" {
			t.Errorf("skipped: %v", skipped)
		}
	})

	t.Run("empty proposed", func(t *testing.T) {
		existing := map[string]MetaTypeInfo{"date": {Name: MetaTypeDate}}
		merged, added, skipped := mergeMetaConfig(existing, map[string]MetaTypeInfo{})
		if len(merged) != 1 {
			t.Fatalf("merged len: %d", len(merged))
		}
		if len(added) != 0 || len(skipped) != 0 {
			t.Errorf("added=%v skipped=%v", added, skipped)
		}
	})
}

func TestInferType(t *testing.T) {
	t.Run("all dates", func(t *testing.T) {
		stats := &keyStats{total: 5, dateMatch: 5, samples: []string{"2024-01-01", "2024-03-20"}}
		got := inferType("created", stats)
		if got.InferredType != MetaTypeDate {
			t.Errorf("got %q, want date", got.InferredType)
		}
		if got.TotalValues != 5 || got.MatchCount != 5 {
			t.Errorf("counts: total=%d match=%d", got.TotalValues, got.MatchCount)
		}
	})

	t.Run("all numbers", func(t *testing.T) {
		stats := &keyStats{total: 10, numberMatch: 10, samples: []string{"1", "42"}}
		got := inferType("priority", stats)
		if got.InferredType != MetaTypeNumber {
			t.Errorf("got %q, want number", got.InferredType)
		}
	})

	t.Run("all semver", func(t *testing.T) {
		stats := &keyStats{total: 3, semverMatch: 3, samples: []string{"1.2.3", "v2.0.0"}}
		got := inferType("version", stats)
		if got.InferredType != MetaTypeSemver {
			t.Errorf("got %q, want semver", got.InferredType)
		}
	})

	t.Run("threshold 80 percent date", func(t *testing.T) {
		// 8/10 = 80% → date
		stats := &keyStats{total: 10, dateMatch: 8, samples: []string{"2024-01-01"}}
		got := inferType("date", stats)
		if got.InferredType != MetaTypeDate {
			t.Errorf("got %q, want date", got.InferredType)
		}
	})

	t.Run("below threshold falls to string", func(t *testing.T) {
		// 7/10 = 70% → string
		stats := &keyStats{total: 10, dateMatch: 7, samples: []string{"2024-01-01"}}
		got := inferType("date", stats)
		if got.InferredType != MetaTypeString {
			t.Errorf("got %q, want string", got.InferredType)
		}
	})

	t.Run("mixed types uses best match", func(t *testing.T) {
		// number: 9/10, date: 2/10 → number wins
		stats := &keyStats{total: 10, dateMatch: 2, numberMatch: 9, samples: []string{"42"}}
		got := inferType("val", stats)
		if got.InferredType != MetaTypeNumber {
			t.Errorf("got %q, want number", got.InferredType)
		}
	})

	t.Run("empty values excluded from denominator", func(t *testing.T) {
		// total=0 (all empty) → string
		stats := &keyStats{total: 0}
		got := inferType("empty", stats)
		if got.InferredType != MetaTypeString {
			t.Errorf("got %q, want string", got.InferredType)
		}
	})

	t.Run("low cardinality noted", func(t *testing.T) {
		unique := map[string]struct{}{
			"draft": {}, "review": {}, "done": {},
		}
		stats := &keyStats{
			total: 10, uniqueSet: unique, uniqueCount: 3,
			samples: []string{"draft", "review", "done"},
		}
		got := inferType("status", stats)
		if got.InferredType != MetaTypeString {
			t.Errorf("got %q, want string", got.InferredType)
		}
		if got.UniqueCount != 3 {
			t.Errorf("unique count: got %d, want 3", got.UniqueCount)
		}
		sort.Strings(got.UniqueValues)
		if len(got.UniqueValues) != 3 || got.UniqueValues[0] != "done" {
			t.Errorf("unique values: %v", got.UniqueValues)
		}
	})

	t.Run("high cardinality unique values nil", func(t *testing.T) {
		stats := &keyStats{
			total: 25, uniqueCount: -1,
			samples: []string{"a"},
		}
		got := inferType("title", stats)
		if got.UniqueValues != nil {
			t.Errorf("expected nil unique values for high cardinality")
		}
		if got.UniqueCount != -1 {
			t.Errorf("unique count: got %d, want -1", got.UniqueCount)
		}
	})

	t.Run("date over semver priority", func(t *testing.T) {
		// "2024-01-01" matches both date and semver is not possible (date has hyphens, semver rejects them)
		// But "1.2.3" could match number: false, semver: true
		// Test: date=9, semver=8, total=10 → date wins (higher count)
		stats := &keyStats{total: 10, dateMatch: 9, semverMatch: 8, samples: []string{"2024-01-01"}}
		got := inferType("d", stats)
		if got.InferredType != MetaTypeDate {
			t.Errorf("got %q, want date", got.InferredType)
		}
	})
}

func TestScanMetaTypes(t *testing.T) {
	vault := copyVault(t, "vault_init_meta")

	cfg, err := LoadConfig(vault)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	inferred, err := scanMetaTypes(vault, cfg)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	// tags and aliases should be skipped
	if _, ok := inferred["tags"]; ok {
		t.Error("tags should be skipped")
	}
	if _, ok := inferred["aliases"]; ok {
		t.Error("aliases should be skipped")
	}

	// created: 3 dates + 1 non-date = 75% → string (below 80%)
	if im, ok := inferred["created"]; !ok {
		t.Error("missing created")
	} else {
		if im.TotalValues != 4 {
			t.Errorf("created total: %d", im.TotalValues)
		}
		if im.InferredType != MetaTypeString {
			t.Errorf("created type: %q (3/4 = 75%% < 80%%)", im.InferredType)
		}
	}

	// priority: all numbers
	if im, ok := inferred["priority"]; !ok {
		t.Error("missing priority")
	} else if im.InferredType != MetaTypeNumber {
		t.Errorf("priority type: %q, want number", im.InferredType)
	}

	// version: 3 files have version → all semver
	if im, ok := inferred["version"]; !ok {
		t.Error("missing version")
	} else if im.InferredType != MetaTypeSemver {
		t.Errorf("version type: %q, want semver", im.InferredType)
	}

	// status: should be string with low cardinality
	if im, ok := inferred["status"]; !ok {
		t.Error("missing status")
	} else {
		if im.InferredType != MetaTypeString {
			t.Errorf("status type: %q, want string", im.InferredType)
		}
		if im.UniqueCount > orderedMaxCardinality {
			t.Errorf("status should have low cardinality, got %d", im.UniqueCount)
		}
		if len(im.UniqueValues) == 0 {
			t.Error("status should have unique values listed")
		}
	}

	// title: should exist
	if _, ok := inferred["title"]; !ok {
		t.Error("missing title")
	}
}

func TestBuildMetaYAMLNode(t *testing.T) {
	t.Run("preset only with comments", func(t *testing.T) {
		types := map[string]MetaTypeInfo{
			"created":  {Name: MetaTypeDate},
			"priority": {Name: MetaTypeNumber},
		}
		node := buildMetaYAMLNode(types, nil, false)
		out, err := marshalYAMLNode(node)
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if !strings.Contains(s, "created: date") {
			t.Errorf("missing created: date in:\n%s", s)
		}
		if !strings.Contains(s, "priority: number") {
			t.Errorf("missing priority: number in:\n%s", s)
		}
		// Should have preset comment
		if !strings.Contains(s, "# preset") {
			t.Errorf("missing preset comment in:\n%s", s)
		}
	})

	t.Run("scan inferred with comments", func(t *testing.T) {
		types := map[string]MetaTypeInfo{
			"created": {Name: MetaTypeDate},
		}
		inferred := map[string]InferredMeta{
			"created": {
				Key: "created", InferredType: MetaTypeDate,
				TotalValues: 15, MatchCount: 15,
				SampleValues: []string{"2024-01-15", "2024-03-20"},
			},
		}
		node := buildMetaYAMLNode(types, inferred, false)
		out, err := marshalYAMLNode(node)
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if !strings.Contains(s, "inferred: date") {
			t.Errorf("missing inferred comment in:\n%s", s)
		}
		if !strings.Contains(s, "15/15") {
			t.Errorf("missing count in:\n%s", s)
		}
	})

	t.Run("ordered candidate comment", func(t *testing.T) {
		types := map[string]MetaTypeInfo{
			"status": {Name: MetaTypeString},
		}
		inferred := map[string]InferredMeta{
			"status": {
				Key: "status", InferredType: MetaTypeString,
				TotalValues: 10, MatchCount: 0,
				SampleValues: []string{"draft", "review", "done"},
				UniqueValues: []string{"done", "draft", "review"},
				UniqueCount:  3,
			},
		}
		node := buildMetaYAMLNode(types, inferred, false)
		out, err := marshalYAMLNode(node)
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if !strings.Contains(s, "consider:") {
			t.Errorf("missing ordered candidate comment in:\n%s", s)
		}
		if !strings.Contains(s, "ordered:") {
			t.Errorf("missing ordered suggestion in:\n%s", s)
		}
	})

	t.Run("no-comment mode", func(t *testing.T) {
		types := map[string]MetaTypeInfo{
			"created": {Name: MetaTypeDate},
		}
		inferred := map[string]InferredMeta{
			"created": {
				Key: "created", InferredType: MetaTypeDate,
				TotalValues: 15, MatchCount: 15,
				SampleValues: []string{"2024-01-15"},
			},
		}
		node := buildMetaYAMLNode(types, inferred, true)
		out, err := marshalYAMLNode(node)
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if strings.Contains(s, "#") {
			t.Errorf("no-comment mode should have no comments:\n%s", s)
		}
	})

	t.Run("round trip through LoadConfig", func(t *testing.T) {
		types := map[string]MetaTypeInfo{
			"created":  {Name: MetaTypeDate},
			"priority": {Name: MetaTypeNumber},
			"version":  {Name: MetaTypeSemver},
		}
		node := buildMetaYAMLNode(types, nil, true)
		out, err := marshalYAMLNode(node)
		if err != nil {
			t.Fatal(err)
		}

		// Write to temp file and load
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), out, 0644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.Meta.Types["created"].Name != MetaTypeDate {
			t.Errorf("created: %v", cfg.Meta.Types["created"])
		}
		if cfg.Meta.Types["priority"].Name != MetaTypeNumber {
			t.Errorf("priority: %v", cfg.Meta.Types["priority"])
		}
		if cfg.Meta.Types["version"].Name != MetaTypeSemver {
			t.Errorf("version: %v", cfg.Meta.Types["version"])
		}
	})
}

func TestMergeIntoExistingYAML(t *testing.T) {
	t.Run("preserves build section", func(t *testing.T) {
		existing := []byte(`build:
  exclude_paths:
    - "templates/*"
meta:
  types:
    date: date
`)
		newTypes := map[string]MetaTypeInfo{
			"priority": {Name: MetaTypeNumber},
		}
		metaNode := buildMetaYAMLNode(newTypes, nil, true)
		out, err := mergeIntoExistingYAML(existing, metaNode)
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if !strings.Contains(s, "exclude_paths") {
			t.Errorf("build section lost:\n%s", s)
		}
		if !strings.Contains(s, "priority: number") {
			t.Errorf("new type not added:\n%s", s)
		}
	})

	t.Run("creates meta section if absent", func(t *testing.T) {
		existing := []byte(`build:
  exclude_paths:
    - "templates/*"
`)
		newTypes := map[string]MetaTypeInfo{
			"created": {Name: MetaTypeDate},
		}
		metaNode := buildMetaYAMLNode(newTypes, nil, true)
		out, err := mergeIntoExistingYAML(existing, metaNode)
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if !strings.Contains(s, "meta:") {
			t.Errorf("meta section missing:\n%s", s)
		}
		if !strings.Contains(s, "created: date") {
			t.Errorf("created not added:\n%s", s)
		}
	})
}

// marshalYAMLNode is a helper to serialize a yaml.Node to bytes.
func marshalYAMLNode(node *yaml.Node) ([]byte, error) {
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{node}}
	return marshalYAML(doc)
}

func TestInitMeta(t *testing.T) {
	t.Run("preset only", func(t *testing.T) {
		dir := t.TempDir()
		result, err := InitMeta(dir, InitMetaOptions{Preset: true})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.YAML, "created: date") {
			t.Errorf("missing preset key in:\n%s", result.YAML)
		}
		if len(result.Added) != 15 {
			t.Errorf("expected 15 added, got %d", len(result.Added))
		}
	})

	t.Run("scan only", func(t *testing.T) {
		vault := copyVault(t, "vault_init_meta")
		result, err := InitMeta(vault, InitMetaOptions{Scan: true})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.YAML, "priority: number") {
			t.Errorf("missing scanned key in:\n%s", result.YAML)
		}
		// created: 3/4 = 75% < 80% → string
		if !strings.Contains(result.YAML, "created: string") {
			t.Errorf("created should be string (below threshold) in:\n%s", result.YAML)
		}
		if len(result.Inferred) == 0 {
			t.Error("expected inferred results")
		}
	})

	t.Run("combined preset and scan", func(t *testing.T) {
		vault := copyVault(t, "vault_init_meta")
		result, err := InitMeta(vault, InitMetaOptions{Preset: true, Scan: true})
		if err != nil {
			t.Fatal(err)
		}
		// Scan result should be present
		if !strings.Contains(result.YAML, "priority: number") {
			t.Errorf("missing scanned key in:\n%s", result.YAML)
		}
		// Preset-only keys (not in vault) should also be present
		if !strings.Contains(result.YAML, "deadline: date") {
			t.Errorf("missing preset-only key in:\n%s", result.YAML)
		}
	})

	t.Run("merge with existing config", func(t *testing.T) {
		vault := copyVault(t, "vault_init_meta")
		// Write existing config
		existing := `build:
  exclude_paths:
    - "templates/*"
meta:
  types:
    priority: string
`
		if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte(existing), 0644); err != nil {
			t.Fatal(err)
		}
		result, err := InitMeta(vault, InitMetaOptions{Preset: true})
		if err != nil {
			t.Fatal(err)
		}
		// priority should be skipped (existing)
		found := false
		for _, k := range result.Skipped {
			if k == "priority" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("priority should be skipped, skipped=%v", result.Skipped)
		}
		// build section preserved
		if !strings.Contains(result.YAML, "exclude_paths") {
			t.Errorf("build section lost:\n%s", result.YAML)
		}
	})

	t.Run("neither preset nor scan", func(t *testing.T) {
		dir := t.TempDir()
		_, err := InitMeta(dir, InitMetaOptions{})
		if err == nil {
			t.Error("expected error when neither --preset nor --scan")
		}
	})

	t.Run("vault not found", func(t *testing.T) {
		_, err := InitMeta("/nonexistent/vault/path", InitMetaOptions{Preset: true})
		if err == nil {
			t.Error("expected error for nonexistent vault")
		}
	})

	t.Run("round trip generated YAML", func(t *testing.T) {
		dir := t.TempDir()
		result, err := InitMeta(dir, InitMetaOptions{Preset: true, NoComment: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "mdhop.yaml"), []byte(result.YAML), 0644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig(dir)
		if err != nil {
			t.Fatalf("LoadConfig round-trip: %v", err)
		}
		if cfg.Meta.Types["created"].Name != MetaTypeDate {
			t.Errorf("created: %v", cfg.Meta.Types["created"])
		}
		if cfg.Meta.Types["version"].Name != MetaTypeSemver {
			t.Errorf("version: %v", cfg.Meta.Types["version"])
		}
	})
}
