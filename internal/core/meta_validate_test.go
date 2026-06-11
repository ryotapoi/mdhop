package core

import (
	"sort"
	"testing"
)

func setupMetaValidateVault(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_meta_validate")
	buildForQuery(t, vault)
	return vault
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
