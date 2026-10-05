package core

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPathLinkTypeClassificationsCoverAllLinkTypes(t *testing.T) {
	expected := map[LinkType]struct {
		isPath    bool
		rewrite   bool
		traversal bool
	}{
		LinkTypeWikilink:            {isPath: true, rewrite: true, traversal: true},
		LinkTypeMarkdown:            {isPath: true, rewrite: true, traversal: true},
		LinkTypeMarkdownReference:   {isPath: true, traversal: true},
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

	rewriteTypes := linkTypeSet(t, rewriteLinkTypes)
	traversalTypes := linkTypeSet(t, traversalLinkTypes)
	assertSQLLinkTypesDeclared(t, "rewriteLinkTypes", rewriteTypes, declaredSet)
	assertSQLLinkTypesDeclared(t, "traversalLinkTypes", traversalTypes, declaredSet)
	for linkType, want := range expected {
		if got := isPathLinkType(linkType); got != want.isPath {
			t.Errorf("isPathLinkType(%q) = %v, want %v", linkType, got, want.isPath)
		}
		if got := rewriteTypes[linkType]; got != want.rewrite {
			t.Errorf("rewriteLinkTypes contains %q = %v, want %v", linkType, got, want.rewrite)
		}
		if got := traversalTypes[linkType]; got != want.traversal {
			t.Errorf("traversalLinkTypes contains %q = %v, want %v", linkType, got, want.traversal)
		}
	}
}

func assertRollbackFailureReported(t *testing.T, err error, primary, rollback string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected operation to fail")
	}
	for _, want := range []string{primary, "rollback failed", "could not restore", rollback, "mdhop build"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q:\n%s", want, err)
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

func linkTypeSet(t *testing.T, linkTypes []LinkType) map[LinkType]bool {
	t.Helper()
	set := make(map[LinkType]bool)
	for _, linkType := range linkTypes {
		if set[linkType] {
			t.Fatalf("LinkType %q is duplicated", linkType)
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

	if err := os.WriteFile(fullPath, []byte("modified\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	backups := []rewriteBackup{
		{path: filePath, content: []byte("original\n"), perm: 0o600, mtime: setRollbackTestMtime(t, fullPath)},
	}

	if failures := restoreBackupFiles(dir, backups); len(failures) != 0 {
		t.Fatalf("restoreBackupFiles failures: %#v", failures)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original\n" {
		t.Errorf("content = %q, want %q", string(content), "original\n")
	}

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

	rewrites := []rewriteEntry{{
		edgeID:     1,
		rawLink:    "[[OldTarget]]",
		linkType:   LinkTypeWikilink,
		lineStart:  1,
		sourcePath: filePath,
		sourceID:   100,
		newRawLink: "[[NewTarget]]",
	}}

	_, backups, rollbackFailures, err := applyFileRewritesWithRollbackFailures(vault, rewrites)
	if err != nil {
		t.Fatalf("applyFileRewritesWithRollbackFailures: %v", err)
	}
	if len(rollbackFailures) != 0 {
		t.Fatalf("unexpected rollback failures: %#v", rollbackFailures)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "[[NewTarget]]\n" {
		t.Errorf("content = %q, want %q", string(content), "[[NewTarget]]\n")
	}

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

func TestApplyFileRewritesRollbackFailuresHaveDeterministicPathOrder(t *testing.T) {
	vault := t.TempDir()
	var rewrites []rewriteEntry
	for i, path := range []string{"Two.md", "Three.md", "One.md"} {
		if err := os.WriteFile(filepath.Join(vault, path), []byte("[[Old]]\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		rewrites = append(rewrites, rewriteEntry{
			rawLink:    "[[Old]]",
			linkType:   LinkTypeWikilink,
			lineStart:  1,
			sourcePath: path,
			sourceID:   int64(i + 1),
			newRawLink: "[[New]]",
		})
	}

	oldRewriteWriteFile := rewriteWriteFile
	writeCalls := 0
	primaryErr := errors.New("primary rewrite blocked")
	rewriteWriteFile = func(path string, data []byte, perm os.FileMode) error {
		writeCalls++
		if writeCalls == 3 {
			if err := oldRewriteWriteFile(path, data, perm); err != nil {
				return err
			}
			return primaryErr
		}
		return oldRewriteWriteFile(path, data, perm)
	}
	t.Cleanup(func() { rewriteWriteFile = oldRewriteWriteFile })

	oldRollbackWriteFile := rollbackWriteFile
	rollbackWriteFile = func(path string, _ []byte, _ os.FileMode) error {
		return fmt.Errorf("restore blocked for %s", filepath.Base(path))
	}
	t.Cleanup(func() { rollbackWriteFile = oldRollbackWriteFile })

	_, _, failures, err := applyFileRewritesWithRollbackFailures(vault, rewrites)
	if err == nil || !strings.Contains(err.Error(), "primary rewrite blocked") {
		t.Fatalf("primary error = %v", err)
	}
	if len(failures) != 3 {
		t.Fatalf("rollback failures = %#v, want 3", failures)
	}
	if failures[0].path != "One.md" || failures[1].path != "Three.md" || failures[2].path != "Two.md" {
		t.Fatalf("rollback failure paths = [%s, %s, %s], want [One.md, Three.md, Two.md]", failures[0].path, failures[1].path, failures[2].path)
	}

	wrappedErr := wrapRollbackFailures(err, failures)
	if !errors.Is(wrappedErr, primaryErr) {
		t.Fatalf("wrapped error does not retain primary error: %v", wrappedErr)
	}
	wrapped := wrappedErr.Error()
	oneIndex := strings.Index(wrapped, "could not restore One.md")
	threeIndex := strings.Index(wrapped, "could not restore Three.md")
	twoIndex := strings.Index(wrapped, "could not restore Two.md")
	if oneIndex < 0 || threeIndex < 0 || twoIndex < 0 || oneIndex >= threeIndex || threeIndex >= twoIndex {
		t.Fatalf("rollback detail order is not deterministic:\n%s", wrapped)
	}
}

func TestApplyFileRewritesRestoresCurrentFileAfterPartialWriteError(t *testing.T) {
	vault := t.TempDir()
	filePath := "Source.md"
	fullPath := filepath.Join(vault, filePath)
	original := []byte("[[Old]]\n")
	if err := os.WriteFile(fullPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fullPath, 0o600); err != nil {
		t.Fatal(err)
	}

	originalMtime := setRollbackTestMtime(t, fullPath)

	primaryErr := errors.New("partial rewrite blocked")
	oldRewriteWriteFile := rewriteWriteFile
	rewriteWriteFile = func(path string, data []byte, perm os.FileMode) error {
		if err := oldRewriteWriteFile(path, data, perm); err != nil {
			return err
		}
		return primaryErr
	}
	t.Cleanup(func() { rewriteWriteFile = oldRewriteWriteFile })

	_, _, failures, err := applyFileRewritesWithRollbackFailures(vault, []rewriteEntry{{
		edgeID: 1, rawLink: "[[Old]]", linkType: LinkTypeWikilink, lineStart: 1,
		sourcePath: filePath, sourceID: 1, newRawLink: "[[New]]",
	}})
	if !errors.Is(err, primaryErr) {
		t.Fatalf("error = %v, want primary error", err)
	}
	if len(failures) != 0 {
		t.Fatalf("rollback failures = %#v, want none", failures)
	}
	if got := mustReadFile(t, fullPath); string(got) != string(original) {
		t.Fatalf("content after rollback = %q, want %q", got, original)
	}
	info, statErr := os.Stat(fullPath)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("permission after rollback = %o, want %o", got, 0o600)
	}
	if !info.ModTime().Equal(originalMtime) {
		t.Fatalf("mtime after rollback = %v, want %v", info.ModTime(), originalMtime)
	}
}

func TestApplyFileRewritesPreflightsAllCandidatesBeforeWriting(t *testing.T) {
	vault := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(vault, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("First.md", "[[Old]]\n")
	unsupported := "---\nref: \"\\u005b\\u005bOld\\u005d\\u005d\"\n---\n"
	write("Later.md", unsupported)

	oldWrite := rewriteWriteFile
	writes := 0
	rewriteWriteFile = func(path string, content []byte, perm os.FileMode) error {
		writes++
		return oldWrite(path, content, perm)
	}
	t.Cleanup(func() { rewriteWriteFile = oldWrite })

	_, _, _, err := applyFileRewritesWithRollbackFailures(vault, []rewriteEntry{
		{sourcePath: "First.md", sourceID: 1, rawLink: "[[Old]]", newRawLink: "[[New]]", linkType: LinkTypeWikilink, lineStart: 1},
		{sourcePath: "Later.md", sourceID: 2, rawLink: "[[Old]]", newRawLink: "[[New]]", linkType: LinkTypeFrontmatterWikilink, lineStart: 2},
	})
	if err == nil || !strings.Contains(err.Error(), "correspondence") {
		t.Fatalf("error = %v, want frontmatter correspondence rejection", err)
	}
	if writes != 0 {
		t.Fatalf("writes = %d, want 0 before candidate rejection", writes)
	}
	if got := string(mustReadFile(t, filepath.Join(vault, "First.md"))); got != "[[Old]]\n" {
		t.Fatalf("First.md = %q, want unchanged", got)
	}
	if got := string(mustReadFile(t, filepath.Join(vault, "Later.md"))); got != unsupported {
		t.Fatalf("Later.md = %q, want unchanged", got)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func TestReplaceOutsideInlineCodeEscapedBackticks(t *testing.T) {
	for _, tt := range []struct {
		name string
		line string
		want string
	}{
		{"no backslash", "`[[A]]` [[A]]", "`[[A]]` [[B]]"},
		{"one backslash", "\\`[[A]]", "\\`[[B]]"},
		{"two backslashes", "\\\\`[[A]]` [[A]]", "\\\\`[[A]]` [[B]]"},
		{"three backslashes", "\\\\\\`[[A]]", "\\\\\\`[[B]]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := replaceBodyLink(tt.line, "[[A]]", "[[B]]", LinkTypeWikilink); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReplaceOutsideInlineCodeDelimiterRuns(t *testing.T) {
	for _, code := range []string{
		"`LINK`", "``LINK``", "```LINK ` LINK `` LINK```",
		"``LINK ` LINK ``` LINK``", "``LINK ` LINK",
	} {
		t.Run(code, func(t *testing.T) {
			line, want := "LINK "+code, "NEW "+code
			if code[len(code)-1] == '`' {
				line += " LINK"
				want += " NEW"
			}
			if got := replaceBodyLink(line, "LINK", "NEW", LinkTypeWikilink); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// setRollbackTestMtime uses an old timestamp so a fresh rollback write cannot
// accidentally satisfy the stale check's second precision.
func setRollbackTestMtime(t *testing.T, path string) time.Time {
	t.Helper()
	old := time.Unix(1234567890, 123456789)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime()
}

func TestRollbackReportsMtimeRestoreFailure(t *testing.T) {
	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprintf("local=%t", local), func(t *testing.T) {
			vault := t.TempDir()
			full := filepath.Join(vault, "Source.md")
			if err := os.WriteFile(full, []byte("[[Old]]\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			mtime := setRollbackTestMtime(t, full)
			primary := errors.New("primary rewrite blocked")
			oldRestore := rollbackWriteFile
			rollbackWriteFile = func(path string, data []byte, perm os.FileMode) error {
				if err := oldRestore(path, data, perm); err != nil {
					return err
				}
				// The write succeeds, but the subsequent timestamp restore cannot.
				return os.Remove(path)
			}
			t.Cleanup(func() { rollbackWriteFile = oldRestore })
			var failures []rollbackFailure
			err := primary
			if local {
				oldWrite := rewriteWriteFile
				rewriteWriteFile = func(string, []byte, os.FileMode) error { return primary }
				t.Cleanup(func() { rewriteWriteFile = oldWrite })
				_, _, failures, err = applyFileRewritesWithRollbackFailures(vault, []rewriteEntry{{sourcePath: "Source.md", rawLink: "[[Old]]", newRawLink: "[[New]]", linkType: LinkTypeWikilink, lineStart: 1}})
			} else {
				failures = restoreBackupFiles(vault, []rewriteBackup{{path: "Source.md", content: []byte("[[Old]]\n"), perm: 0o600, mtime: mtime}})
			}
			if len(failures) != 1 || !os.IsNotExist(failures[0].err) {
				t.Fatalf("mtime failures = %#v", failures)
			}
			wrapped := wrapRollbackFailures(err, failures)
			if !errors.Is(wrapped, primary) {
				t.Fatalf("lost primary error: %v", wrapped)
			}
			assertRollbackFailureReported(t, wrapped, primary.Error(), "Source.md")
		})
	}
}

func TestRewriteContentCandidateMarkdownBoundaries(t *testing.T) {
	for _, prefix := range []string{
		"[[Target|[shown](Old.md)]]",
		"[outer [shown](Old.md)](https://example.com)",
		"`[shown](Old.md)`",
	} {
		t.Run(prefix, func(t *testing.T) {
			content := prefix + " ![shown](Old.md) [shown](Old.md)\n"
			got, err := rewriteContentCandidate([]byte(content), []rewriteEntry{{rawLink: "[shown](Old.md)", newRawLink: "[[Old|shown]]", linkType: LinkTypeMarkdown, lineStart: 1}})
			want := prefix + " ![[Old|shown]] [[Old|shown]]\n"
			if err != nil || string(got) != want {
				t.Fatalf("got %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestRewriteContentCandidatePreservesMaskedLabels(t *testing.T) {
	for _, line := range []string{"[`shown`](Old.md)", "[outer [[Target]]](Old.md)"} {
		t.Run(line, func(t *testing.T) {
			var rewrites []rewriteEntry
			for _, link := range parseLinks(line).Links {
				if link.linkType == LinkTypeMarkdown {
					rewrites = append(rewrites, rewriteEntry{rawLink: link.rawLink, newRawLink: convertMarkdownToWikilink(link.rawLink), linkType: link.linkType, lineStart: link.lineStart})
				}
			}
			if len(rewrites) != 1 || rewrites[0].rawLink == line {
				t.Fatalf("expected masked Markdown raw, got %+v", rewrites)
			}
			got, err := rewriteContentCandidate([]byte(line), rewrites)
			if err != nil || string(got) != line {
				t.Fatalf("got %q, %v; want original %q", got, err, line)
			}
		})
	}
}

func TestRewriteRawLinkBackticksByContext(t *testing.T) {
	for _, tt := range []struct{ raw, target, want string }{
		{"[[A|shown]]", "Z`Q.md", "[[Z`Q|shown]]"},
		{"[[A#H`I|shown]]", "B.md", "[[B#H`I|shown]]"},
	} {
		if got := rewriteRawLink(tt.raw, LinkTypeWikilink, tt.target); got != "" {
			t.Fatalf("body rewrite = %q, want rejection", got)
		}
		if got := rewriteRawLink(tt.raw, LinkTypeFrontmatterWikilink, tt.target); got != tt.want {
			t.Fatalf("frontmatter rewrite = %q, want %q", got, tt.want)
		}
	}
	for _, target := range []string{"A#B.md", "A|B.md", "A]]B.md", "A\nB.md"} {
		if got := rewriteRawLink("[[A]]", LinkTypeFrontmatterWikilink, target); got != "" {
			t.Fatalf("unsafe rewrite = %q", got)
		}
	}
}

func TestRewriteRawLinkClosingBracketBoundary(t *testing.T) {
	for _, tt := range []struct {
		name, raw, path, want, target, subpath string
		table                                  bool
	}{
		{"bare trailing bracket", "[[A]]", "B].md", "", "", "", false},
		{"internal single bracket", "[[A]]", "B]C.md", "[[B]C]]", "B]C", "", false},
		{"target separated by subpath", "[[A#H]]", "B].md", "[[B]#H]]", "B]", "#H", false},
		{"target separated by alias", "[[A|shown]]", "B].md", "[[B]|shown]]", "B]", "", false},
		{"subpath trailing bracket", "[[A#H]|shown]]", "B.md", "[[B#H]|shown]]", "B", "#H]", false},
		{"table alias", `[[A#H]\|shown]]`, "B].md", `[[B]#H]\|shown]]`, "B]", "#H]", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, kind := range []LinkType{LinkTypeWikilink, LinkTypeFrontmatterWikilink} {
				got := rewriteRawLink(tt.raw, kind, tt.path, tt.table)
				if got != tt.want {
					t.Fatalf("rewrite = %q, want %q", got, tt.want)
				}
				if got == "" {
					continue
				}
				links := parseWikiLinks(got, 1, tt.table)
				if len(links) != 1 || links[0].rawLink != got || links[0].target != tt.target || links[0].subpath != tt.subpath {
					t.Fatalf("reparse = %+v", links)
				}
			}
		})
	}
	// A synthetic candidate is needed: the existing parser cannot index a
	// subpath ending in ] without an alias in the first place.
	if got := rewriteRawLink("[[A#H]]]", LinkTypeWikilink, "B.md"); got != "" {
		t.Fatalf("trailing subpath = %q", got)
	}
}
