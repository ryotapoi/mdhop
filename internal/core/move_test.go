package core

import (
	"os"
	"path/filepath"
	"testing"
)

func newMoveVault(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	if err := os.MkdirAll(vault, 0o755); err != nil {
		t.Fatalf("mkdir vault: %v", err)
	}
	for rel, content := range files {
		writeVaultFile(t, vault, rel, content)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	return vault
}

func writeVaultFile(t *testing.T, vault, rel, content string) {
	t.Helper()
	full := filepath.Join(vault, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func readVaultFile(t *testing.T, vault, rel string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(vault, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(content)
}

func assertEdgeRawLinks(t *testing.T, vault, sourcePath string, want []string) {
	t.Helper()
	edges := queryEdges(t, dbPath(vault), sourcePath)
	got := make(map[string]int, len(edges))
	for _, e := range edges {
		got[e.rawLink]++
	}
	for _, raw := range want {
		if got[raw] == 0 {
			t.Fatalf("missing edge rawLink %q for %s; edges: %+v", raw, sourcePath, edges)
		}
		got[raw]--
	}
}
