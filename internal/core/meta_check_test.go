package core

import (
	"os"
	"path/filepath"
	"testing"
)

func setupMetaCheckVault(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_meta_check")
	buildForQuery(t, vault)
	return vault
}

func TestMetaCheck_PathKind(t *testing.T) {
	vault := setupMetaCheckVault(t)

	result, err := MetaCheck(vault, MetaCheckOptions{
		Keys: []string{"sources"},
		Kind: MetaKindPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ./guide.md resolves, https URL is allowed → only ./missing.md is an issue.
	if len(result.Issues) != 1 {
		t.Fatalf("issues = %+v, want 1", result.Issues)
	}
	is := result.Issues[0]
	if is.Value != "./missing.md" || is.Reason != ReasonNotFound {
		t.Errorf("issue = %+v, want ./missing.md not_found", is)
	}
	if is.SourcePath != "docs/index.md" || is.Key != "sources" {
		t.Errorf("issue source/key = %s/%s, want docs/index.md/sources", is.SourcePath, is.Key)
	}
}

func TestMetaCheckResolvesNFCValueToNFDPath(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "Cafe\u0301.md"), []byte("# Cafe\n"), 0o644); err != nil {
		t.Fatalf("write NFD note: %v", err)
	}
	note := "---\nsources:\n  - Caf\u00e9.md\n---\n"
	if err := os.WriteFile(filepath.Join(vault, "Index.md"), []byte(note), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MetaCheck(vault, MetaCheckOptions{
		Keys: []string{"sources"},
		Kind: MetaKindPath,
	})
	if err != nil {
		t.Fatalf("meta-check: %v", err)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("issues = %+v, want none", result.Issues)
	}
}

func TestMetaCheckPathKindDirectoryReferences(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "docs", "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	note := "---\nsources:\n  - ./assets/\n  - ./missing-dir/\n  - ../../outside/\n  - docs\n---\n"
	if err := os.WriteFile(filepath.Join(vault, "docs", "Index.md"), []byte(note), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	result, err := MetaCheck(vault, MetaCheckOptions{
		Keys: []string{"sources"},
		Kind: MetaKindPath,
	})
	if err != nil {
		t.Fatalf("meta-check: %v", err)
	}

	if len(result.Issues) != 3 {
		t.Fatalf("issues = %+v, want 3", result.Issues)
	}
	got := map[string]MetaIssueReason{}
	for _, issue := range result.Issues {
		got[issue.Value] = issue.Reason
	}
	if got["./missing-dir/"] != ReasonNotFound {
		t.Fatalf("missing dir issue = %v, want not_found; issues=%+v", got["./missing-dir/"], result.Issues)
	}
	if got["../../outside/"] != ReasonVaultEscape {
		t.Fatalf("escaping dir issue = %v, want vault_escape; issues=%+v", got["../../outside/"], result.Issues)
	}
	if got["docs"] != ReasonNotFound {
		t.Fatalf("docs without slash issue = %v, want existing non-directory behavior not_found; issues=%+v", got["docs"], result.Issues)
	}
	if _, ok := got["./assets/"]; ok {
		t.Fatalf("existing directory reported as issue: %+v", result.Issues)
	}
}

func TestMetaCheck_WikilinkKind(t *testing.T) {
	vault := setupMetaCheckVault(t)

	result, err := MetaCheck(vault, MetaCheckOptions{
		Keys: []string{"related"},
		Kind: MetaKindWikilink,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// [[guide]] resolves → only [[Nonexistent]] is an issue.
	if len(result.Issues) != 1 {
		t.Fatalf("issues = %+v, want 1", result.Issues)
	}
	if result.Issues[0].Value != "[[Nonexistent]]" || result.Issues[0].Reason != ReasonNotFound {
		t.Errorf("issue = %+v, want [[Nonexistent]] not_found", result.Issues[0])
	}
}

func TestMetaCheck_RequiresKey(t *testing.T) {
	vault := setupMetaCheckVault(t)
	if _, err := MetaCheck(vault, MetaCheckOptions{Kind: MetaKindPath}); err == nil {
		t.Fatal("expected error when no --key given")
	}
}

func TestMetaCheck_InvalidKind(t *testing.T) {
	vault := setupMetaCheckVault(t)
	if _, err := MetaCheck(vault, MetaCheckOptions{Keys: []string{"sources"}, Kind: "bogus"}); err == nil {
		t.Fatal("expected error for invalid kind")
	}
}
