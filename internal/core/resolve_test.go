package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryotapoi/mdhop/internal/testutil"
)

func copyVaultForResolve(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", name)
	dst := filepath.Join(t.TempDir(), "vault")
	if err := testutil.CopyDir(root, dst); err != nil {
		t.Fatalf("copy vault: %v", err)
	}
	return dst
}

func buildVault(t *testing.T, vaultPath string) {
	t.Helper()
	if _, err := Build(vaultPath); err != nil {
		t.Fatalf("build: %v", err)
	}
}

func TestResolveFieldConstants(t *testing.T) {
	fields := map[string]string{
		"type":    FieldResolveType,
		"name":    FieldResolveName,
		"path":    FieldResolvePath,
		"exists":  FieldResolveExists,
		"subpath": FieldResolveSubpath,
	}
	for want, got := range fields {
		if got != want {
			t.Errorf("field constant = %q, want %q", got, want)
		}
	}
}

func TestResolveWikilinkBasename(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[Design]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Design" {
		t.Errorf("name = %q, want %q", res.Name, "Design")
	}
	if res.Path != "Design.md" {
		t.Errorf("path = %q, want %q", res.Path, "Design.md")
	}
	if !res.Exists {
		t.Errorf("exists = false, want true")
	}
	if res.Subpath != "" {
		t.Errorf("subpath = %q, want empty", res.Subpath)
	}
}

func TestResolveNFCLinkAgainstNFDIndexName(t *testing.T) {
	vault := t.TempDir()
	nfdPath := "Cafe\u0301.md"
	nfcPath := "Caf\u00e9.md"
	if err := os.WriteFile(filepath.Join(vault, nfdPath), []byte("# Cafe\n"), 0o644); err != nil {
		t.Fatalf("write NFD note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "Ref.md"), []byte("[[Caf\u00e9]]\n"), 0o644); err != nil {
		t.Fatalf("write ref: %v", err)
	}
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}

	db := openTestDB(t, dbPath(vault))
	_, err := db.Exec(
		"UPDATE nodes SET node_key = ?, path = ?, name = ? WHERE node_key = ?",
		"note:path:"+nfdPath, nfdPath, "Cafe\u0301", noteKey(nfcPath),
	)
	if err != nil {
		db.Close()
		t.Fatalf("simulate pre-v0.12 NFD row: %v", err)
	}
	db.Close()

	res, err := Resolve(vault, "Ref.md", "[[Caf\u00e9]]")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Type != NodeTypeNote || normalizeTextNFC(res.Path) != nfcPath {
		t.Fatalf("resolved = %+v, want note path %s", res, nfcPath)
	}
}

func TestResolveWikilinkVaultRelative(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[sub/Impl]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Impl" {
		t.Errorf("name = %q, want %q", res.Name, "Impl")
	}
	if res.Path != "sub/Impl.md" {
		t.Errorf("path = %q, want %q", res.Path, "sub/Impl.md")
	}
}

func TestResolveWikilinkSubpath(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Design.md", "[[sub/Impl#Details]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Path != "sub/Impl.md" {
		t.Errorf("path = %q, want %q", res.Path, "sub/Impl.md")
	}
	if res.Subpath != "#Details" {
		t.Errorf("subpath = %q, want %q", res.Subpath, "#Details")
	}
}

func TestResolveWikilinkSelfLink(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[#Index]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Path != "Index.md" {
		t.Errorf("path = %q, want %q", res.Path, "Index.md")
	}
	if res.Subpath != "#Index" {
		t.Errorf("subpath = %q, want %q", res.Subpath, "#Index")
	}
}

func TestResolveMarkdownRelative(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[impl](./sub/Impl.md)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Impl" {
		t.Errorf("name = %q, want %q", res.Name, "Impl")
	}
	if res.Path != "sub/Impl.md" {
		t.Errorf("path = %q, want %q", res.Path, "sub/Impl.md")
	}
}

func TestResolveMarkdownAbsolute(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "sub/Impl.md", "[index](/Index.md)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Index" {
		t.Errorf("name = %q, want %q", res.Name, "Index")
	}
	if res.Path != "Index.md" {
		t.Errorf("path = %q, want %q", res.Path, "Index.md")
	}
}

func TestResolveMarkdownPath(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_edges")
	buildVault(t, vault)

	res, err := Resolve(vault, "A.md", "[C](sub/C.md)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "C" {
		t.Errorf("name = %q, want %q", res.Name, "C")
	}
	if res.Path != "sub/C.md" {
		t.Errorf("path = %q, want %q", res.Path, "sub/C.md")
	}
}

func TestResolveMarkdownRelativeParent(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_relative")
	buildVault(t, vault)

	res, err := Resolve(vault, "dir/Source.md", "[root](../Root.md)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Root" {
		t.Errorf("name = %q, want %q", res.Name, "Root")
	}
	if res.Path != "Root.md" {
		t.Errorf("path = %q, want %q", res.Path, "Root.md")
	}
}

func TestResolvePhantom(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[Missing]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypePhantom {
		t.Errorf("type = %q, want %q", res.Type, "phantom")
	}
	if res.Name != "Missing" {
		t.Errorf("name = %q, want %q", res.Name, "Missing")
	}
	if res.Exists {
		t.Errorf("exists = true, want false")
	}
}

func TestResolveTag(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "#overview")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeTag {
		t.Errorf("type = %q, want %q", res.Type, "tag")
	}
	if res.Name != "#overview" {
		t.Errorf("name = %q, want %q", res.Name, "#overview")
	}
}

func TestResolveFrontmatterTag(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "#project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeTag {
		t.Errorf("type = %q, want %q", res.Type, "tag")
	}
	if res.Name != "#project" {
		t.Errorf("name = %q, want %q", res.Name, "#project")
	}
}

func TestResolveMarkdownBasename(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[design](Design.md)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Design" {
		t.Errorf("name = %q, want %q", res.Name, "Design")
	}
	if res.Path != "Design.md" {
		t.Errorf("path = %q, want %q", res.Path, "Design.md")
	}
}

func TestResolveWikilinkRelative(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[./sub/Impl]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Impl" {
		t.Errorf("name = %q, want %q", res.Name, "Impl")
	}
	if res.Path != "sub/Impl.md" {
		t.Errorf("path = %q, want %q", res.Path, "sub/Impl.md")
	}
}

func TestResolveCaseInsensitive(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	res, err := Resolve(vault, "Index.md", "[[design]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeNote {
		t.Errorf("type = %q, want %q", res.Type, "note")
	}
	if res.Name != "Design" {
		t.Errorf("name = %q, want %q", res.Name, "Design")
	}
	if res.Path != "Design.md" {
		t.Errorf("path = %q, want %q", res.Path, "Design.md")
	}
}

func TestResolveErrorSourceNotInDB(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	_, err := Resolve(vault, "NonExist.md", "[[Design]]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "source not in index") {
		t.Errorf("error = %q, want containing %q", err.Error(), "source not in index")
	}
}

func TestResolveErrorLinkNotInSource(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_full")
	buildVault(t, vault)

	// [[NotHere]] is not a link in Index.md
	_, err := Resolve(vault, "Index.md", "[[NotHere]]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "link not found") {
		t.Errorf("error = %q, want containing %q", err.Error(), "link not found")
	}
}

func TestResolveBasenameRootPriority(t *testing.T) {
	// Design.md is at root + insert other/Design.md → root priority resolves to root.
	vault := filepath.Join(t.TempDir(), "vault")
	if err := testutil.CopyDir(filepath.Join("..", "..", "testdata", "vault_build_full"), vault); err != nil {
		t.Fatalf("copy vault: %v", err)
	}
	buildVault(t, vault)

	// Manually insert a second note with the same basename as "Design".
	db := openTestDB(t, dbPath(vault))
	_, err := db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag, mtime) VALUES (?, 'note', 'Design', 'other/Design.md', 1, 0)`,
		noteKey("other/Design.md"),
	)
	if err != nil {
		db.Close()
		t.Fatalf("insert duplicate: %v", err)
	}
	db.Close()

	// Root priority: Design.md is at root → resolves to it.
	res, err := Resolve(vault, "Index.md", "[[Design]]")
	if err != nil {
		t.Fatalf("expected success (root priority), got: %v", err)
	}
	if res.Path != "Design.md" {
		t.Errorf("path = %q, want %q", res.Path, "Design.md")
	}
}

func TestResolveBasenameAmbiguousNoRoot(t *testing.T) {
	// Two notes in subdirs (no root) → ambiguous.
	vault := filepath.Join(t.TempDir(), "vault")
	if err := testutil.CopyDir(filepath.Join("..", "..", "testdata", "vault_build_full"), vault); err != nil {
		t.Fatalf("copy vault: %v", err)
	}
	buildVault(t, vault)

	// Rename Design.md in DB to sub1/Design.md (no root).
	db := openTestDB(t, dbPath(vault))
	_, err := db.Exec(
		`UPDATE nodes SET path = 'sub1/Design.md', node_key = ? WHERE path = 'Design.md'`,
		noteKey("sub1/Design.md"),
	)
	if err != nil {
		db.Close()
		t.Fatalf("update path: %v", err)
	}
	// Insert second note at sub2/Design.md.
	_, err = db.Exec(
		`INSERT INTO nodes (node_key, type, name, path, exists_flag, mtime) VALUES (?, 'note', 'Design', 'sub2/Design.md', 1, 0)`,
		noteKey("sub2/Design.md"),
	)
	if err != nil {
		db.Close()
		t.Fatalf("insert duplicate: %v", err)
	}
	db.Close()

	_, err = Resolve(vault, "Index.md", "[[Design]]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error = %q, want containing %q", err.Error(), "ambiguous")
	}
}

func TestResolveBasenameBackendAmbiguityPolicy(t *testing.T) {
	db := openTestDB(t, filepath.Join(t.TempDir(), "index.sqlite"))
	defer db.Close()
	if err := initSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	link := linkOccur{
		rawLink:    "[[A]]",
		target:     "A",
		linkType:   LinkTypeWikilink,
		isBasename: true,
	}

	rm := newResolveMaps([]string{"sub1/A.md", "sub2/A.md"}, nil)
	id, _, err := resolveLink(db, "Source.md", link, rm)
	if err != nil {
		t.Fatalf("map resolver should fall through to phantom, got: %v", err)
	}
	var nodeType NodeType
	var name string
	if err := db.QueryRow(`SELECT type, name FROM nodes WHERE id = ?`, id).Scan(&nodeType, &name); err != nil {
		t.Fatalf("query resolved node: %v", err)
	}
	if nodeType != NodeTypePhantom || name != "A" {
		t.Fatalf("map resolver target = (%s, %q), want phantom A", nodeType, name)
	}

	if _, err := upsertNote(db, "sub1/A.md", "A", 0, 1); err != nil {
		t.Fatalf("insert first note: %v", err)
	}
	if _, err := upsertNote(db, "sub2/A.md", "A", 0, 1); err != nil {
		t.Fatalf("insert second note: %v", err)
	}
	_, _, err = resolveLinkFromDB(db, "Source.md", link)
	if !errors.Is(err, ErrAmbiguousLink) {
		t.Fatalf("DB resolver error = %v, want ErrAmbiguousLink", err)
	}
}

func TestResolveBasenameBackendPriorityContract(t *testing.T) {
	tests := []struct {
		name          string
		notes         []string
		assets        []string
		target        string
		wantType      NodeType
		wantPath      string
		wantAmbiguous bool
	}{
		{
			name:     "note unique wins over asset",
			notes:    []string{"Note.md"},
			assets:   []string{"Note"},
			target:   "note",
			wantType: NodeTypeNote,
			wantPath: "Note.md",
		},
		{
			name:     "note root priority wins over asset",
			notes:    []string{"Note.md", "sub/Note.md"},
			assets:   []string{"Note"},
			target:   "Note",
			wantType: NodeTypeNote,
			wantPath: "Note.md",
		},
		{
			name:     "asset unique",
			assets:   []string{"sub/photo.png"},
			target:   "photo.png",
			wantType: NodeTypeAsset,
			wantPath: "sub/photo.png",
		},
		{
			name:     "asset root priority",
			assets:   []string{"logo.png", "sub/logo.png"},
			target:   "logo.png",
			wantType: NodeTypeAsset,
			wantPath: "logo.png",
		},
		{
			name:     "phantom fallback",
			target:   "Missing",
			wantType: NodeTypePhantom,
		},
		{
			name:          "ambiguous notes do not fall through in DB",
			notes:         []string{"sub1/Note.md", "sub2/Note.md"},
			target:        "Note",
			wantAmbiguous: true,
		},
		{
			name:          "ambiguous assets do not fall through in DB",
			assets:        []string{"sub1/logo.png", "sub2/logo.png"},
			target:        "logo.png",
			wantAmbiguous: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t, filepath.Join(t.TempDir(), "index.sqlite"))
			defer db.Close()
			if err := initSchema(db); err != nil {
				t.Fatalf("init schema: %v", err)
			}

			rm := newResolveMaps(tt.notes, tt.assets)
			for _, path := range tt.notes {
				id, err := upsertNote(db, path, basename(path), 0, 1)
				if err != nil {
					t.Fatalf("insert note %s: %v", path, err)
				}
				rm.registerNote(path, id)
			}
			for _, path := range tt.assets {
				id, err := upsertAsset(db, path, filepath.Base(path), 0)
				if err != nil {
					t.Fatalf("insert asset %s: %v", path, err)
				}
				rm.registerAsset(path, id)
			}

			link := linkOccur{
				rawLink:    "[[" + tt.target + "#Heading]]",
				target:     tt.target,
				subpath:    "#Heading",
				linkType:   LinkTypeWikilink,
				isBasename: true,
			}
			mapID, mapSubpath, mapErr := resolveLink(db, "Source.md", link, rm)
			dbID, dbSubpath, dbErr := resolveLinkFromDB(db, "Source.md", link)

			if tt.wantAmbiguous {
				// Build/add/update/move reject these links before map resolution;
				// retain the internal map fallback while asserting resolve's
				// strict ErrAmbiguousLink contract.
				if mapErr != nil {
					t.Fatalf("map resolver error = %v, want guarded phantom fallback", mapErr)
				}
				var mapType NodeType
				if err := db.QueryRow(`SELECT type FROM nodes WHERE id = ?`, mapID).Scan(&mapType); err != nil {
					t.Fatalf("query map target: %v", err)
				}
				if mapType != NodeTypePhantom {
					t.Fatalf("map resolver type = %q, want phantom", mapType)
				}
				if mapSubpath != link.subpath {
					t.Errorf("map subpath = %q, want %q", mapSubpath, link.subpath)
				}
				if !errors.Is(dbErr, ErrAmbiguousLink) {
					t.Fatalf("DB resolver error = %v, want ErrAmbiguousLink", dbErr)
				}
				if dbID != 0 || dbSubpath != "" {
					t.Errorf("DB resolver result = (%d, %q), want zero result on error", dbID, dbSubpath)
				}
				return
			}

			if mapErr != nil || dbErr != nil {
				t.Fatalf("resolver errors: map=%v db=%v", mapErr, dbErr)
			}
			if mapID != dbID {
				t.Fatalf("backend target IDs differ: map=%d db=%d", mapID, dbID)
			}
			if mapSubpath != link.subpath || dbSubpath != link.subpath {
				t.Fatalf("subpaths = map:%q db:%q, want %q", mapSubpath, dbSubpath, link.subpath)
			}

			var gotType NodeType
			var gotPath string
			if err := db.QueryRow(`SELECT type, COALESCE(path, '') FROM nodes WHERE id = ?`, mapID).Scan(&gotType, &gotPath); err != nil {
				t.Fatalf("query resolved target: %v", err)
			}
			if gotType != tt.wantType || gotPath != tt.wantPath {
				t.Fatalf("resolved target = (%q, %q), want (%q, %q)", gotType, gotPath, tt.wantType, tt.wantPath)
			}
		})
	}
}

func TestResolveAssetPathBased(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_assets")
	// Add a markdown file that links to sub/photo.jpg via path-based markdown link.
	if err := os.WriteFile(filepath.Join(vault, "PathLinker.md"), []byte("[photo](sub/photo.jpg)\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	buildVault(t, vault)

	res, err := Resolve(vault, "PathLinker.md", "[photo](sub/photo.jpg)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypeAsset {
		t.Errorf("type = %q, want %q", res.Type, "asset")
	}
	if res.Path != "sub/photo.jpg" {
		t.Errorf("path = %q, want %q", res.Path, "sub/photo.jpg")
	}
	if res.Name != "photo.jpg" {
		t.Errorf("name = %q, want %q", res.Name, "photo.jpg")
	}
	if !res.Exists {
		t.Errorf("exists = false, want true")
	}
	if res.Subpath != "" {
		t.Errorf("subpath = %q, want empty", res.Subpath)
	}
}

func TestResolvePhantomPathBased(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_assets")
	// Add two markdown files with path-based links to non-existent targets.
	if err := os.WriteFile(filepath.Join(vault, "Trigger.md"), []byte("[gone](deep/Gone)\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "MdStrip.md"), []byte("[gone](deep/Gone.md)\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	buildVault(t, vault)

	t.Run("without_md_suffix", func(t *testing.T) {
		res, err := Resolve(vault, "Trigger.md", "[gone](deep/Gone)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Type != NodeTypePhantom {
			t.Errorf("type = %q, want %q", res.Type, "phantom")
		}
		if res.Name != "Gone" {
			t.Errorf("name = %q, want %q", res.Name, "Gone")
		}
		if res.Path != "" {
			t.Errorf("path = %q, want empty", res.Path)
		}
		if res.Exists {
			t.Errorf("exists = true, want false")
		}
	})

	t.Run("with_md_suffix_stripped", func(t *testing.T) {
		res, err := Resolve(vault, "MdStrip.md", "[gone](deep/Gone.md)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Type != NodeTypePhantom {
			t.Errorf("type = %q, want %q", res.Type, "phantom")
		}
		if res.Name != "Gone" {
			t.Errorf("name = %q, want %q", res.Name, "Gone")
		}
		if res.Path != "" {
			t.Errorf("path = %q, want empty", res.Path)
		}
		if res.Exists {
			t.Errorf("exists = true, want false")
		}
	})
}

func TestResolveErrorDBNotFound(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_empty")
	// Do NOT build — DB should not exist.

	_, err := Resolve(vault, "A.md", "[[X]]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "index not found") {
		t.Errorf("error = %q, want containing %q", err.Error(), "index not found")
	}
}

func TestResolvePhantomAssetExtension(t *testing.T) {
	vault := copyVaultForResolve(t, "vault_build_assets")
	// Add a markdown file that links to a non-existent asset (png).
	if err := os.WriteFile(filepath.Join(vault, "AssetMissLinker.md"), []byte("[missing](nonexistent/file.png)\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	buildVault(t, vault)

	res, err := Resolve(vault, "AssetMissLinker.md", "[missing](nonexistent/file.png)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Type != NodeTypePhantom {
		t.Errorf("type = %q, want %q", res.Type, "phantom")
	}
	if res.Name != "file.png" {
		t.Errorf("name = %q, want %q", res.Name, "file.png")
	}
	if res.Path != "" {
		t.Errorf("path = %q, want empty", res.Path)
	}
	if res.Exists {
		t.Errorf("exists = true, want false")
	}
	if res.Subpath != "" {
		t.Errorf("subpath = %q, want empty", res.Subpath)
	}
}
