package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildStatusRootSymlink(t *testing.T) {
	vault := t.TempDir()
	alias := filepath.Join(t.TempDir(), "vault.md")
	if err := os.Symlink(vault, alias); err != nil {
		t.Fatal(err)
	}
	writeStatusFile(t, vault, "Source.md", "---\nref: Target.md\n---\n[[Target]]\n![[image.png]]\n")
	writeStatusFile(t, vault, "Target.md", "target\n")
	writeStatusFile(t, vault, "image.png", "image")
	writeStatusFile(t, vault, "Cafe\u0301.md", "cafe\n")
	writeStatusFile(t, vault, ".hidden/Hidden.md", "hidden note\n")
	writeStatusFile(t, vault, ".hidden/ignored.png", "hidden asset")
	writeStatusFile(t, vault, ".ignored.png", "hidden asset")
	writeStatusFile(t, vault, ".mdhop/ignored.md", "internal")
	writeStatusFile(t, vault, "excluded/Draft.md", "excluded")
	writeStatusFile(t, vault, "mdhop.toml", "[build]\nexclude_paths = ['excluded/**', 'linked-dir']\n")
	if err := os.Symlink(filepath.Join(vault, "Target.md"), filepath.Join(vault, "Internal.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(vault, ".hidden"), filepath.Join(vault, "linked-dir")); err != nil {
		t.Fatal(err)
	}
	loc := Locations{DBPath: filepath.Join(vault, "index.sqlite")}
	if _, err := Build(vault, loc); err != nil {
		t.Fatal(err)
	}
	notes := queryNodes(t, loc.DBPath, NodeTypeNote)
	assets := queryNodes(t, loc.DBPath, NodeTypeAsset)
	edges := queryEdges(t, loc.DBPath, "Source.md")
	var notePaths, assetPaths []string
	for _, n := range notes {
		notePaths = append(notePaths, n.path)
	}
	for _, n := range assets {
		assetPaths = append(assetPaths, n.path)
	}
	if want := []string{".hidden/Hidden.md", "Café.md", "Internal.md", "Source.md", "Target.md"}; !reflect.DeepEqual(notePaths, want) {
		t.Fatalf("notes = %v, want %v", notePaths, want)
	}
	// Excluded entries and the selected database are absent from the index.
	if want := []string{"image.png", "mdhop.toml"}; !reflect.DeepEqual(assetPaths, want) {
		t.Fatalf("assets = %v, want %v", assetPaths, want)
	}
	if len(edges) != 2 || edges[0].targetKey != "note:path:Target.md" || edges[1].targetKey != "asset:path:image.png" {
		t.Fatalf("unexpected edges: %#v", edges)
	}
	// Directory symlinks are terminal collector entries, never traversed.
	collected, err := collectAssetFiles(alias)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"image.png", "index.sqlite", "linked-dir", "mdhop.toml"}; !reflect.DeepEqual(collected, want) {
		t.Fatalf("collected assets = %v, want %v", collected, want)
	}
	beforeDB := mustReadFile(t, loc.DBPath)
	status, err := Status(alias, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Deleted)+len(status.Modified)+len(status.Untracked) != 0 {
		t.Errorf("alias status: %#v", status)
	}
	if !reflect.DeepEqual(beforeDB, mustReadFile(t, loc.DBPath)) {
		t.Fatal("status changed database")
	}
	if dbPath(vault) != dbPath(alias) {
		t.Fatal("aliases selected different default databases")
	}
	if _, err := Build(alias, loc); err != nil {
		t.Fatal(err)
	}
	if got := queryNodes(t, loc.DBPath, NodeTypeNote); !reflect.DeepEqual(got, notes) {
		t.Errorf("alias notes = %#v, want %#v", got, notes)
	}
	if got := queryNodes(t, loc.DBPath, NodeTypeAsset); !reflect.DeepEqual(got, assets) {
		t.Errorf("alias assets = %#v, want %#v", got, assets)
	}
	if got := queryEdges(t, loc.DBPath, "Source.md"); !reflect.DeepEqual(got, edges) {
		t.Errorf("alias edges = %#v, want %#v", got, edges)
	}
}

func TestNormalizedDiskPathsRootSymlink(t *testing.T) {
	vault := t.TempDir()
	alias := filepath.Join(t.TempDir(), "vault")
	if err := os.Symlink(vault, alias); err != nil {
		t.Fatal(err)
	}
	writeStatusFile(t, vault, "Cafe\u0301/Resume\u0301.md", "NFD content\n")
	paths, err := collectNormalizedDiskPaths(alias)
	if err != nil {
		t.Fatal(err)
	}
	path, ok := paths["Café/Resumé.md"]
	if !ok {
		t.Fatal("root alias did not collect NFD disk spelling")
	}
	if got := readTestFile(t, path); got != "NFD content\n" {
		t.Fatalf("content = %q", got)
	}
	// On Linux exact NFC Stat fails, exercising the resolver's disk-map fallback.
	path, err = newVaultDiskPathResolver(alias).existingPath("Café/Resumé.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, path); got != "NFD content\n" {
		t.Fatalf("resolved content = %q", got)
	}
}

func TestRootSymlinkScanAndMeta(t *testing.T) {
	vault := t.TempDir()
	alias := filepath.Join(t.TempDir(), "vault")
	if err := os.Symlink(vault, alias); err != nil {
		t.Fatal(err)
	}
	writeStatusFile(t, vault, "Cafe\u0301.md", "---\ncreated: 2024-01-15\nref: Target.md\n---\n[[Target]]\n")
	writeStatusFile(t, vault, "Target.md", "target\n")
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	inferred, err := scanMetaTypes(alias, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got := inferred["created"]; got.InferredType != MetaTypeDate || got.TotalValues != 1 {
		t.Errorf("inferred created = %#v", got)
	}
	check, err := MetaCheck(alias, MetaCheckOptions{Keys: []string{"ref"}, Kind: MetaKindPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(check.Issues) != 0 {
		t.Errorf("alias meta-check = %#v", check)
	}
	if _, err := Convert(alias, ConvertOptions{ToFormat: "markdown"}); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(vault, "Cafe\u0301.md")); got != "---\ncreated: 2024-01-15\nref: Target.md\n---\n[Target](Target.md)\n" {
		t.Errorf("converted = %q", got)
	}
	if _, err := Build(alias); err != nil {
		t.Fatal(err)
	}
	status, err := Status(alias)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Deleted)+len(status.Modified)+len(status.Untracked) != 0 {
		t.Errorf("status after rewrite/build: %#v", status)
	}
}
