package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestPathLinkTypeClassificationsCoverAllLinkTypes(t *testing.T) {
	expected := map[LinkType]struct {
		isPath    bool
		rewrite   bool
		traversal bool
	}{
		LinkTypeWikilink:            {isPath: true, rewrite: true, traversal: true},
		LinkTypeMarkdown:            {isPath: true, rewrite: true, traversal: true},
		LinkTypeTag:                 {},
		LinkTypeFrontmatter:         {},
		LinkTypeFrontmatterWikilink: {isPath: true, rewrite: true, traversal: true},
		LinkTypeFrontmatterPath:     {isPath: true, traversal: true},
	}

	declared := declaredLinkTypes(t)
	if len(declared) != len(expected) {
		t.Fatalf("declared LinkType count = %d, expected classification count = %d; update this test for every LinkType", len(declared), len(expected))
	}
	declaredSet := make(map[LinkType]bool, len(declared))
	for _, linkType := range declared {
		declaredSet[linkType] = true
		if _, ok := expected[linkType]; !ok {
			t.Errorf("LinkType %q has no path/traversal classification", linkType)
		}
	}

	rewriteTypes := sqlLinkTypeSet(t, pathLinkTypeSQLList)
	traversalTypes := sqlLinkTypeSet(t, traversalLinkTypeSQLList)
	assertSQLLinkTypesDeclared(t, "pathLinkTypeSQLList", rewriteTypes, declaredSet)
	assertSQLLinkTypesDeclared(t, "traversalLinkTypeSQLList", traversalTypes, declaredSet)
	for linkType, want := range expected {
		if got := isPathLinkType(linkType); got != want.isPath {
			t.Errorf("isPathLinkType(%q) = %v, want %v", linkType, got, want.isPath)
		}
		if got := rewriteTypes[linkType]; got != want.rewrite {
			t.Errorf("pathLinkTypeSQLList contains %q = %v, want %v", linkType, got, want.rewrite)
		}
		if got := traversalTypes[linkType]; got != want.traversal {
			t.Errorf("traversalLinkTypeSQLList contains %q = %v, want %v", linkType, got, want.traversal)
		}
	}
}

func declaredLinkTypes(t *testing.T) []LinkType {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(filepath.Dir(testFile), "db.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse db.go: %v", err)
	}

	var linkTypes []LinkType
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok || valueSpec.Type == nil || len(valueSpec.Names) != 1 || len(valueSpec.Values) != 1 {
				continue
			}
			typeName, ok := valueSpec.Type.(*ast.Ident)
			if !ok || typeName.Name != "LinkType" {
				continue
			}
			literal, ok := valueSpec.Values[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				t.Fatalf("LinkType %s must use a string literal", valueSpec.Names[0].Name)
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatalf("unquote LinkType %s: %v", valueSpec.Names[0].Name, err)
			}
			linkTypes = append(linkTypes, LinkType(value))
		}
	}
	return linkTypes
}

func sqlLinkTypeSet(t *testing.T, list string) map[LinkType]bool {
	t.Helper()
	set := make(map[LinkType]bool)
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if len(item) < 2 || item[0] != '\'' || item[len(item)-1] != '\'' {
			t.Fatalf("SQL link type literal %q is not single-quoted", item)
		}
		linkType := LinkType(item[1 : len(item)-1])
		if set[linkType] {
			t.Fatalf("SQL link type literal %q is duplicated", linkType)
		}
		set[linkType] = true
	}
	return set
}

func assertSQLLinkTypesDeclared(t *testing.T, listName string, sqlTypes, declared map[LinkType]bool) {
	t.Helper()
	for linkType := range sqlTypes {
		if !declared[linkType] {
			t.Errorf("%s contains undeclared LinkType %q", listName, linkType)
		}
	}
}

func TestRestoreBackupsPreservesPermission(t *testing.T) {
	dir := t.TempDir()
	filePath := "test.md"
	fullPath := filepath.Join(dir, filePath)

	// Create file with 0o600.
	if err := os.WriteFile(fullPath, []byte("modified\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	backups := []rewriteBackup{
		{path: filePath, content: []byte("original\n"), perm: 0o600},
	}

	restoreBackups(dir, backups)

	// Verify content restored.
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original\n" {
		t.Errorf("content = %q, want %q", string(content), "original\n")
	}

	// Verify permission preserved.
	info, err := os.Stat(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want %o", perm, 0o600)
	}
}

func TestApplyFileRewritesPreservesPermission(t *testing.T) {
	vault := t.TempDir()

	// Create a file with 0o600 containing a wikilink to replace.
	filePath := "source.md"
	fullPath := filepath.Join(vault, filePath)
	original := []byte("[[OldTarget]]\n")
	if err := os.WriteFile(fullPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	// Ensure permission is exactly 0o600 (not masked by umask).
	if err := os.Chmod(fullPath, 0o600); err != nil {
		t.Fatal(err)
	}

	groups := map[string][]rewriteEntry{
		filePath: {
			{
				edgeID:     1,
				rawLink:    "[[OldTarget]]",
				linkType:   LinkTypeWikilink,
				lineStart:  1,
				sourcePath: filePath,
				sourceID:   100,
				newRawLink: "[[NewTarget]]",
			},
		},
	}

	_, backups, err := applyFileRewrites(vault, groups)
	if err != nil {
		t.Fatalf("applyFileRewrites: %v", err)
	}

	// Verify file content was rewritten.
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "[[NewTarget]]\n" {
		t.Errorf("content = %q, want %q", string(content), "[[NewTarget]]\n")
	}

	// Verify file permission preserved.
	info, err := os.Stat(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want %o", perm, 0o600)
	}

	// Verify backup has correct perm field.
	if len(backups) != 1 {
		t.Fatalf("len(backups) = %d, want 1", len(backups))
	}
	if backups[0].perm != 0o600 {
		t.Errorf("backup perm = %o, want %o", backups[0].perm, 0o600)
	}
}
