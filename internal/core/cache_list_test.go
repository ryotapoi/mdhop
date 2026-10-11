package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCacheHash(t *testing.T) {
	for _, tc := range []struct {
		name  string
		valid bool
	}{
		{strings.Repeat("a", 64), true}, {strings.Repeat("0", 64), true}, {strings.Repeat("f", 63), false},
		{strings.Repeat("A", 64), false}, {strings.Repeat("g", 64), false}, {strings.Repeat("f", 65), false},
	} {
		if got := isCacheHash(tc.name); got != tc.valid {
			t.Errorf("hash %q = %v", tc.name, got)
		}
	}
}

func TestListCachesContentsAndLifecycle(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	root := filepath.Join(cache, "mdhop", "vaults")
	low := filepath.Join(root, strings.Repeat("0", 64))
	high := filepath.Join(root, strings.Repeat("f", 64))
	if err := os.MkdirAll(filepath.Join(low, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(high, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "invalid"), 0700); err != nil {
		t.Fatal(err)
	}
	// A valid hash symlink is not a cache folder.
	if err := os.Symlink(low, filepath.Join(root, strings.Repeat("a", 64))); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	mustMigrationWrite(t, filepath.Join(outside, "large"), strings.Repeat("x", 100000))
	if err := os.Symlink(outside, filepath.Join(low, "link")); err != nil {
		t.Fatal(err)
	}
	files := []string{filepath.Join(low, "index.sqlite"), filepath.Join(low, "index.sqlite-wal"), filepath.Join(low, "index.sqlite.tmp-build"), filepath.Join(low, "nested", "sparse")}
	for _, p := range files {
		mustMigrationWrite(t, p, "content")
	}
	sparse, err := os.OpenFile(files[3], os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := sparse.Truncate(1 << 30); err != nil {
		t.Fatal(err)
	}
	sparse.Close()
	hard := filepath.Join(low, "hardlink")
	if err := os.Link(files[0], hard); err != nil {
		t.Fatal(err)
	}
	past := filepath.Join(t.TempDir(), "deleted-vault")
	mustMigrationWrite(t, filepath.Join(high, cacheOriginName), `{"vault":`+strconvJSON(t, past)+`}`)
	// Independent lstat values cover root directories, sparse files, links, and hardlinks.
	paths := append([]string{low, filepath.Join(low, "nested"), filepath.Join(low, "link"), hard}, files...)
	var want int64
	for _, p := range paths {
		var stat syscall.Stat_t
		if err := syscall.Lstat(p, &stat); err != nil {
			t.Fatal(err)
		}
		want += stat.Blocks * 512
	}
	entries, err := ListCaches()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].CachePath != low || entries[1].CachePath != high || entries[0].Vault != nil || entries[1].Vault == nil || *entries[1].Vault != past || entries[0].SizeBytes != want {
		t.Fatalf("entries=%+v want size=%d", entries, want)
	}
	extra := filepath.Join(low, "extra")
	mustMigrationWrite(t, extra, strings.Repeat("x", 20000))
	var stat syscall.Stat_t
	if err := syscall.Lstat(extra, &stat); err != nil {
		t.Fatal(err)
	}
	updated, err := ListCaches()
	if err != nil {
		t.Fatal(err)
	}
	// Directory allocation can change too; do not assume a fixed filesystem block size.
	if updated[0].SizeBytes < entries[0].SizeBytes+stat.Blocks*512 {
		t.Fatal("new content not included")
	}
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	reduced, err := ListCaches()
	if err != nil || reduced[0].SizeBytes >= updated[0].SizeBytes {
		t.Fatalf("removed content not reflected: %+v %v", reduced, err)
	}
	if err := os.RemoveAll(high); err != nil {
		t.Fatal(err)
	}
	updated, err = ListCaches()
	if err != nil || len(updated) != 1 {
		t.Fatalf("after deletion: %+v %v", updated, err)
	}
}

func strconvJSON(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestListCachesEmptyAndOriginErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	t.Setenv("XDG_CACHE_HOME", missing)
	entries, err := ListCaches()
	if err != nil || entries == nil || len(entries) != 0 {
		t.Fatalf("empty=%v %v", entries, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("listing created root")
	}
	dir := filepath.Join(missing, "mdhop", "vaults", strings.Repeat("b", 64))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{"{", `{}`, `{"vault":null}`, `{"vault":"relative"}`} {
		mustMigrationWrite(t, filepath.Join(dir, cacheOriginName), data)
		if _, err := ListCaches(); err == nil {
			t.Errorf("accepted %q", data)
		}
	}
	if err := os.Remove(filepath.Join(dir, cacheOriginName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, cacheOriginName), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ListCaches(); err == nil {
		t.Fatal("origin directory accepted")
	}
	if err := os.Remove(filepath.Join(dir, cacheOriginName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), filepath.Join(dir, cacheOriginName)); err != nil {
		t.Fatal(err)
	}
	if _, err := ListCaches(); err == nil {
		t.Fatal("unreadable symlink origin treated as absent")
	}

}

func TestBuildCacheOriginAndExplicitDB(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	vault := t.TempDir()
	mustMigrationWrite(t, filepath.Join(vault, "A.md"), "# A")
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(vault, alias); err != nil {
		t.Fatal(err)
	}
	dbp := dbPath(vault)
	origin := filepath.Join(filepath.Dir(dbp), cacheOriginName)
	// An explicit path at the default location still must not create a record.
	if _, err := Build(alias, Locations{DBPath: dbp}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(origin); !os.IsNotExist(err) {
		t.Fatal("explicit DB recorded")
	}
	if _, err := Build(alias); err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalVault(vault)
	if err != nil {
		t.Fatal(err)
	}
	got, err := readCacheOrigin(filepath.Dir(dbp))
	if err != nil || got == nil || *got != canonical {
		t.Fatalf("origin=%v %v", got, err)
	}
	fixedTime := time.Unix(1234567890, 0)
	if err := os.Chtimes(origin, fixedTime, fixedTime); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(origin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(vault, Locations{DBPath: dbp}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(origin)
	info, err := os.Stat(origin)
	if err != nil || !info.ModTime().Equal(fixedTime) {
		t.Fatalf("explicit DB changed origin mtime: %v %v", info, err)
	}
	if !reflect.DeepEqual(original, after) {
		t.Fatal("explicit DB updated origin")
	}
	external := filepath.Join(t.TempDir(), "external.sqlite")
	sentinel := filepath.Join(filepath.Dir(external), cacheOriginName)
	mustMigrationWrite(t, sentinel, "user file")
	if _, err := Build(vault, Locations{DBPath: external}); err != nil {
		t.Fatal(err)
	}
	assertMigrationFile(t, sentinel, "user file")
	moved := vault + "-moved"
	if err := os.Rename(vault, moved); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(moved)
	entries, err := ListCaches()
	if err != nil || len(entries) != 1 || *entries[0].Vault != canonical {
		t.Fatalf("moved=%+v %v", entries, err)
	}
	if err := os.RemoveAll(moved); err != nil {
		t.Fatal(err)
	}
	if _, err := ListCaches(); err != nil {
		t.Fatal(err)
	}
}

func TestCacheOriginFailurePreservesBuildAndMigrate(t *testing.T) {
	for _, migration := range []bool{false, true} {
		t.Run(map[bool]string{false: "build", true: "migrate"}[migration], func(t *testing.T) {
			vault := migrationVault(t)
			dbp := dbPath(vault)
			if err := os.MkdirAll(filepath.Dir(dbp), 0700); err != nil {
				t.Fatal(err)
			}
			mustMigrationWrite(t, dbp, "old DB")
			// Rename onto a directory fails reliably without relying on permissions.
			if err := os.Mkdir(filepath.Join(filepath.Dir(dbp), cacheOriginName), 0700); err != nil {
				t.Fatal(err)
			}
			if migration {
				mustMigrationWrite(t, filepath.Join(vault, "mdhop.yaml"), "{}\n")
				_, err := Migrate(vault)
				if err == nil {
					t.Fatal("migration succeeded")
				}
				assertMigrationFile(t, filepath.Join(vault, "mdhop.yaml"), "{}\n")
				if _, err := os.Stat(filepath.Join(vault, "mdhop.toml")); !os.IsNotExist(err) {
					t.Fatal("config not rolled back")
				}
			} else {
				if _, err := Build(vault); err == nil {
					t.Fatal("build succeeded")
				}
			}
			assertMigrationFile(t, dbp, "old DB")
			for _, pattern := range []string{dbp + ".tmp-*", filepath.Join(filepath.Dir(dbp), cacheOriginName+".tmp-*")} {
				matches, err := filepath.Glob(pattern)
				if err != nil || len(matches) != 0 {
					t.Fatalf("temporary entries=%v %v", matches, err)
				}
			}
		})
	}
}

func TestCacheOriginInsideVaultProtected(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(vault, "cache"))
	mustMigrationWrite(t, filepath.Join(vault, "A.md"), "# A")
	for i := 0; i < 2; i++ {
		if _, err := Build(vault); err != nil {
			t.Fatal(err)
		}
	}
	status, err := Status(vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Untracked)+len(status.Modified)+len(status.Deleted) != 0 {
		t.Fatalf("status=%+v", status)
	}
	origin := filepath.Join(filepath.Dir(dbPath(vault)), cacheOriginName)
	matcher := newIndexFileMatcher(vault, nil)
	for _, p := range []string{origin, origin + ".tmp-unique"} {
		got, err := matcher.matches(p)
		if err != nil || !got {
			t.Fatalf("protected=%v %v", got, err)
		}
	}
	rel, err := filepath.Rel(vault, origin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Add(vault, AddOptions{Files: []string{rel}}); err == nil {
		t.Fatal("origin registered")
	}
	// Directory mutation walks must preserve origin and its temporary resources.
	tempOrigin := origin + ".tmp-other-build"
	mustMigrationWrite(t, tempOrigin, "pending")
	original, err := os.ReadFile(origin)
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.ToSlash(filepath.Dir(rel))
	diskOnly, err := collectDiskOnlyFiles(vault, sourceDir, "moved", nil)
	if err != nil || len(diskOnly) != 0 {
		t.Fatalf("origin included in move walk: %v %v", diskOnly, err)
	}
	if err := cleanupDeletedDirectories(vault, []string{sourceDir}, nil, newVaultDiskPathResolver(vault)); err != nil {
		t.Fatal(err)
	}
	assertMigrationFile(t, origin, string(original))
	assertMigrationFile(t, tempOrigin, "pending")
	external := newIndexFileMatcher(vault, []Locations{{DBPath: dbPath(vault)}})
	got, err := external.matches(origin)
	if err != nil || got {
		t.Fatalf("explicit DB protects user origin: %v %v", got, err)
	}
}
