package core

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func setupMetaValidateVault(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_meta_validate")
	buildForQuery(t, vault)
	return vault
}

func setupMetaValidateVaultWithConfig(t *testing.T, config string) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_meta_validate")
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	buildForQuery(t, vault)
	return vault
}

func missingViolationsByKey(result *MetaValidateResult, key string) []string {
	var paths []string
	for _, v := range result.Violations {
		if v.Reason == ReasonMissing && v.Key == key {
			paths = append(paths, v.SourcePath)
		}
	}
	sort.Strings(paths)
	return paths
}

func TestMetaValidate_Required(t *testing.T) {
	vault := setupMetaValidateVault(t)

	result, err := MetaValidate(vault, MetaValidateOptions{Require: []string{"status"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only missing.md omits status (good/bad_date/bad_enum all declare it).
	// Type/enum checks always run too, so filter to the missing violations.
	var missing []string
	for _, v := range result.Violations {
		if v.Reason != ReasonMissing {
			continue
		}
		if v.Key != "status" {
			t.Errorf("missing key = %s, want status", v.Key)
		}
		if v.Value != "" {
			t.Errorf("missing value = %q, want empty", v.Value)
		}
		missing = append(missing, v.SourcePath)
	}
	sort.Strings(missing)
	want := []string{"missing.md"}
	if len(missing) != len(want) {
		t.Fatalf("missing = %v, want %v", missing, want)
	}
	for i := range want {
		if missing[i] != want[i] {
			t.Errorf("missing[%d] = %s, want %s", i, missing[i], want[i])
		}
	}
}

func TestMetaValidate_TypeAndEnum(t *testing.T) {
	vault := setupMetaValidateVault(t)

	// No --require: type/enum violations come purely from meta.types.
	result, err := MetaValidate(vault, MetaValidateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byReason := map[MetaViolationReason]MetaViolation{}
	for _, v := range result.Violations {
		byReason[v.Reason] = v
	}
	if len(result.Violations) != 2 {
		t.Fatalf("violations = %+v, want 2 (one type, one enum)", result.Violations)
	}

	typeV, ok := byReason[ReasonType]
	if !ok {
		t.Fatalf("no type violation in %+v", result.Violations)
	}
	if typeV.SourcePath != "bad_date.md" || typeV.Key != "updated" || typeV.Value != "someday" {
		t.Errorf("type violation = %+v, want bad_date.md/updated/someday", typeV)
	}

	enumV, ok := byReason[ReasonEnum]
	if !ok {
		t.Fatalf("no enum violation in %+v", result.Violations)
	}
	if enumV.SourcePath != "bad_enum.md" || enumV.Key != "severity" || enumV.Value != "urgent" {
		t.Errorf("enum violation = %+v, want bad_enum.md/severity/urgent", enumV)
	}
}

func TestMetaValidate_NothingToCheck(t *testing.T) {
	// A vault with no meta.types and no --require has nothing to validate.
	vault := copyVaultForQuery(t, "vault_build_basic")
	buildForQuery(t, vault)
	if _, err := MetaValidate(vault, MetaValidateOptions{}); err == nil {
		t.Fatal("expected error when no --require and no meta.types")
	}
}

func TestMetaValidate_PathFilter(t *testing.T) {
	vault := setupMetaValidateVault(t)

	// Restrict to bad_enum.md → only its enum violation remains.
	result, err := MetaValidate(vault, MetaValidateOptions{Path: []string{"bad_enum.md"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 1 {
		t.Fatalf("violations = %+v, want 1", result.Violations)
	}
	if result.Violations[0].Reason != ReasonEnum || result.Violations[0].SourcePath != "bad_enum.md" {
		t.Errorf("violation = %+v, want enum on bad_enum.md", result.Violations[0])
	}
}

func TestMetaValidate_ProfileRequirePath(t *testing.T) {
	vault := setupMetaValidateVaultWithConfig(t, `meta:
  profiles:
    - path: "media/*"
      require: [isbn]
`)

	result, err := MetaValidate(vault, MetaValidateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := missingViolationsByKey(result, "isbn")
	want := []string{"media/book.md"}
	if len(got) != len(want) {
		t.Fatalf("missing isbn = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("missing isbn[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestMetaValidate_ProfileRequirePathCombinesWithCLIPathFilters(t *testing.T) {
	vault := setupMetaValidateVaultWithConfig(t, `meta:
  profiles:
    - path: "media/*"
      require: [isbn]
`)

	tests := []struct {
		name string
		opts MetaValidateOptions
	}{
		{
			name: "cli path excludes profile path",
			opts: MetaValidateOptions{Path: []string{"bad_date.md"}},
		},
		{
			name: "cli exclude removes matching profile note",
			opts: MetaValidateOptions{Exclude: []string{"media/book.md"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MetaValidate(vault, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := missingViolationsByKey(result, "isbn"); len(got) != 0 {
				t.Fatalf("missing isbn = %v, want none", got)
			}
		})
	}
}

func TestMetaValidate_ProfileRequireAllNotes(t *testing.T) {
	vault := setupMetaValidateVaultWithConfig(t, `meta:
  profiles:
    - require: [category]
`)

	result, err := MetaValidate(vault, MetaValidateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := missingViolationsByKey(result, "category")
	want := []string{
		"bad_date.md",
		"bad_enum.md",
		"good.md",
		"media/book.md",
		"media/essay.md",
		"missing.md",
	}
	if len(got) != len(want) {
		t.Fatalf("missing category = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("missing category[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestMetaValidate_ProfileOnlyIsCheckTarget(t *testing.T) {
	vault := setupMetaValidateVaultWithConfig(t, `meta:
  profiles:
    - path: "media/*"
      require: [isbn]
`)

	if _, err := MetaValidate(vault, MetaValidateOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMetaValidate_DeduplicatesCLIAndProfileRequire(t *testing.T) {
	vault := setupMetaValidateVaultWithConfig(t, `meta:
  profiles:
    - require: [status]
`)

	result, err := MetaValidate(vault, MetaValidateOptions{Require: []string{"status"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := missingViolationsByKey(result, "status")
	want := []string{"missing.md"}
	if len(got) != len(want) {
		t.Fatalf("missing status = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("missing status[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}
