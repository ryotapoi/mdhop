package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectedConfig(t *testing.T) {
	vault := t.TempDir()
	external := filepath.Join(t.TempDir(), "custom.toml")
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, defaultText, explicitText string
		explicit, missing, wantError    bool
		want                            string
	}{
		{name: "absent default"},
		{name: "default", defaultText: "[build]\nexclude_paths=['default/**']", want: "default/**"},
		{name: "explicit only", defaultText: "invalid = [", explicitText: "[build]\nexclude_paths=['explicit/**']", explicit: true, want: "explicit/**"},
		{name: "missing explicit", explicit: true, missing: true, wantError: true},
		{name: "invalid explicit", explicit: true, explicitText: "invalid = [", wantError: true},
		{name: "invalid default", defaultText: "invalid = [", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.Remove(filepath.Join(vault, "mdhop.toml"))
			os.Remove(external)
			if tc.defaultText != "" {
				write(filepath.Join(vault, "mdhop.toml"), tc.defaultText)
			}
			location := Locations{}
			if tc.explicit {
				location.ConfigPath = external
				if !tc.missing {
					write(external, tc.explicitText)
				}
			}
			cfg, err := LoadConfig(vault, location)
			if (err != nil) != tc.wantError {
				t.Fatalf("config error = %v", err)
			}
			if tc.want != "" && (len(cfg.Build.ExcludePaths) != 1 || cfg.Build.ExcludePaths[0] != tc.want) {
				t.Fatalf("config = %+v", cfg)
			}
		})
	}
	// A directory is a read failure rather than an absent optional config.
	os.Remove(filepath.Join(vault, "mdhop.toml"))
	if err := os.Mkdir(filepath.Join(vault, "mdhop.toml"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(vault); err == nil {
		t.Fatal("directory config accepted")
	}
	if _, err := LoadConfig(vault, Locations{ConfigPath: vault}); err == nil {
		t.Fatal("explicit directory config accepted")
	}
}

func TestExternalIndexAndConfig(t *testing.T) {
	vault := t.TempDir()
	external := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(vault, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("A.md", "---\nrank: 1\n---\n[[B]]\n")
	write("B.md", "B body\n")
	write("Excluded.md", "excluded\n")
	// Default DB remains distinguishable from the selected index.
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(external, "arbitrary.toml")
	if err := os.WriteFile(cfg, []byte("[build]\nexclude_paths=['Excluded.md']\n[meta.types]\nrank='number'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	location := Locations{DBPath: filepath.Join(external, "index ?#%.sqlite"), ConfigPath: cfg}
	if _, err := Build(vault, location); err != nil {
		t.Fatal(err)
	}
	stats, err := Stats(vault, StatsOptions{}, location)
	if err != nil || stats.NotesTotal != 2 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
	resolved, err := Resolve(vault, "A.md", "[[B]]", location)
	if err != nil || resolved.Path != "B.md" {
		t.Fatalf("resolve=%+v err=%v", resolved, err)
	}
	head := 1
	if _, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{IncludeHead: &head}, location); err != nil {
		t.Fatal(err)
	}
	if _, err := Set(vault, SetOptions{File: "A.md", Key: "rank", Value: "2"}, location); err != nil {
		t.Fatal(err)
	}
	db, err := openDBChecked(vault, location)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var typ, value string
	if err := db.QueryRow("SELECT value_type,value FROM meta WHERE key='rank'").Scan(&typ, &value); err != nil || typ != "number" || value != "2" {
		t.Fatalf("meta=%s/%s err=%v", typ, value, err)
	}
	// Parsing failure preserves the selected generation byte-for-byte.
	before, _ := os.ReadFile(location.DBPath)
	write("A.md", "[[../outside]]\n")
	if _, err := Build(vault, location); err == nil || !strings.Contains(err.Error(), "link escapes vault") {
		t.Fatalf("error=%v", err)
	}
	after, _ := os.ReadFile(location.DBPath)
	if !bytes.Equal(before, after) {
		t.Fatal("index replaced on failure")
	}
}

func TestIndexDoesNotIndexItself(t *testing.T) {
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, "A.md"), []byte("A\n"), 0600); err != nil {
		t.Fatal(err)
	}
	location := Locations{DBPath: filepath.Join(vault, "generated", "index.sqlite")}
	if err := os.MkdirAll(filepath.Dir(location.DBPath), 0700); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{".tmp-foreign", "-journal", "-wal", "-shm"} {
		if err := os.WriteFile(location.DBPath+suffix, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := Build(vault, location); err != nil {
			t.Fatal(err)
		}
		stats, err := Stats(vault, StatsOptions{}, location)
		if err != nil || stats.AssetsTotal != 0 {
			t.Fatalf("stats=%+v err=%v", stats, err)
		}
	}
}

func TestExternalBuildReadersAcrossReplacement(t *testing.T) {
	vault := t.TempDir()
	location := Locations{DBPath: filepath.Join(t.TempDir(), "index.sqlite")}
	existing := prepareBuildSnapshot(t, "existing")
	if _, err := buildPrepared(vault, existing, location); err != nil {
		t.Fatal(err)
	}
	old, err := openDBChecked(vault, location)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	// openDBChecked has already used the physical connection before replacement.
	assertOwner := func(db dbExecer, wants ...string) {
		t.Helper()
		var owner string
		var n int
		if err := db.QueryRow("SELECT value FROM meta LIMIT 1").Scan(&owner); err != nil {
			t.Fatal(err)
		}
		ok := false
		for _, want := range wants {
			ok = ok || want == owner
		}
		if !ok {
			t.Fatalf("owner=%s", owner)
		}
		if err := db.QueryRow("SELECT COUNT(*) FROM meta WHERE value=?", owner).Scan(&n); err != nil || n != 100 {
			t.Fatalf("mixed generation: n=%d err=%v", n, err)
		}
	}
	assertOwner(old, "existing")
	next := prepareBuildSnapshot(t, "next")
	done := make(chan error, 1)
	go func() { _, err := buildPrepared(vault, next, location); done <- err }()
	// Reads while the rebuild is running can see only completed generations.
	for i := 0; i < 15; i++ {
		db, err := openDBChecked(vault, location)
		if err != nil {
			t.Fatal(err)
		}
		assertOwner(db, "existing", "next")
		db.Close()
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	assertOwner(old, "existing")
	fresh, err := openDBChecked(vault, location)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	assertOwner(fresh, "next")
	// A failure after the private DB exists also leaves the published generation.
	before, _ := os.ReadFile(location.DBPath)
	failed := prepareBuildSnapshot(t, "failed")
	failed.notes[0].links = []linkOccur{{target: "../outside.md", rawLink: "[[../outside]]", isRelative: true}}
	if _, err := buildPrepared(vault, failed, location); !errors.Is(err, ErrLinkEscapesVault) {
		t.Fatalf("error=%v", err)
	}
	after, _ := os.ReadFile(location.DBPath)
	if !bytes.Equal(before, after) {
		t.Fatal("failed build changed index")
	}
	temps, _ := filepath.Glob(location.DBPath + ".tmp-*")
	if len(temps) != 0 {
		t.Fatalf("temps=%v", temps)
	}
}

func TestScanUsesSelectedConfigWithoutDB(t *testing.T) {
	vault := t.TempDir()
	config := filepath.Join(t.TempDir(), "chosen.toml")
	for name, text := range map[string]string{"A.md": "[[B]]\n", "B.md": "[[A]]\n", "mdhop.toml": "invalid = ["} {
		if err := os.WriteFile(filepath.Join(vault, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(config, []byte("[build]\nexclude_paths=['B.md']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	location := Locations{ConfigPath: config, DBPath: filepath.Join(t.TempDir(), "never.sqlite")}
	dry, err := Convert(vault, ConvertOptions{ToFormat: "markdown", DryRun: true}, location)
	if err != nil || len(dry.Rewritten) != 1 {
		t.Fatalf("dry=%+v err=%v", dry, err)
	}
	before, _ := os.ReadFile(filepath.Join(vault, "A.md"))
	if string(before) != "[[B]]\n" {
		t.Fatal("dry-run changed note")
	}
	if _, err := Convert(vault, ConvertOptions{ToFormat: "markdown"}, location); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := os.ReadFile(filepath.Join(vault, "B.md"))
	if string(unchanged) != "[[A]]\n" {
		t.Fatal("excluded note changed")
	}
	if _, err := os.Stat(location.DBPath); !os.IsNotExist(err) {
		t.Fatal("scan created DB")
	}
	if _, err := Build(vault, location); err != nil {
		t.Fatal(err)
	}
}

func TestSelectedIndexSurvivesDirectoryMutation(t *testing.T) {
	for _, operation := range []string{"delete", "move"} {
		t.Run(operation, func(t *testing.T) {
			vault := t.TempDir()
			directory := filepath.Join(vault, "notes")
			if err := os.Mkdir(directory, 0700); err != nil {
				t.Fatal(err)
			}
			for name, content := range map[string]string{"A.md": "A\n", "asset.txt": "asset"} {
				if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			location := Locations{DBPath: filepath.Join(directory, "index.sqlite")}
			if _, err := Build(vault, location); err != nil {
				t.Fatal(err)
			}
			// Model a separate build's private file, which neither mutation owns.
			auxiliary := location.DBPath + ".tmp-foreign"
			if err := os.WriteFile(auxiliary, []byte("foreign build"), 0600); err != nil {
				t.Fatal(err)
			}
			if operation == "delete" {
				if _, err := Delete(vault, DeleteOptions{Files: []string{"notes/"}, RemoveFiles: true}, location); err != nil {
					t.Fatal(err)
				}
				stats, err := Stats(vault, StatsOptions{}, location)
				if err != nil || stats.NotesTotal != 0 {
					t.Fatalf("stats=%+v err=%v", stats, err)
				}
				if _, err := os.Stat(filepath.Join(directory, "asset.txt")); !os.IsNotExist(err) {
					t.Fatal("ordinary asset not deleted")
				}
			} else {
				result, err := MoveDir(vault, MoveDirOptions{FromDir: "notes", ToDir: "other"}, location)
				if err != nil {
					t.Fatal(err)
				}
				for _, moved := range result.Moved {
					if strings.Contains(moved.From, "index.sqlite") {
						t.Fatalf("index moved: %+v", moved)
					}
				}
				if _, err := os.Stat(filepath.Join(vault, "other", "A.md")); err != nil {
					t.Fatal(err)
				}
				stats, err := Stats(vault, StatsOptions{}, location)
				if err != nil || stats.NotesTotal != 1 {
					t.Fatalf("stats=%+v err=%v", stats, err)
				}
				if _, err := Inspect(vault, "other/A.md", InspectOptions{}, location); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := os.Stat(location.DBPath); err != nil {
				t.Fatalf("selected DB disappeared: %v", err)
			}
			content, err := os.ReadFile(auxiliary)
			if err != nil || string(content) != "foreign build" {
				t.Fatalf("auxiliary=%s err=%v", content, err)
			}
		})
	}
}

func TestStatusExcludesSelectedIndex(t *testing.T) {
	vault := t.TempDir()
	directory := filepath.Join(vault, "notes")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "A.md"), []byte("A\n"), 0600); err != nil {
		t.Fatal(err)
	}
	location := Locations{DBPath: filepath.Join(directory, "index.sqlite")}
	if _, err := Build(vault, location); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(location.DBPath+".tmp-foreign", []byte("foreign build"), 0600); err != nil {
		t.Fatal(err)
	}
	status, err := Status(vault, location)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Untracked)+len(status.Modified)+len(status.Deleted) != 0 {
		t.Fatalf("status after build=%+v", status)
	}
	if err := os.WriteFile(filepath.Join(directory, "new.txt"), []byte("new asset"), 0600); err != nil {
		t.Fatal(err)
	}
	status, err = Status(vault, location)
	if err != nil || len(status.Untracked) != 1 || status.Untracked[0] != "notes/new.txt" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestDirectoryWalksExcludeIndexAuxiliaries(t *testing.T) {
	vault := t.TempDir()
	directory := filepath.Join(vault, "notes")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	location := Locations{DBPath: filepath.Join(directory, "index.sqlite")}
	for _, suffix := range []string{"", "-journal", "-wal", "-shm", ".tmp-foreign"} {
		if err := os.WriteFile(location.DBPath+suffix, []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "ordinary.txt"), []byte("ordinary"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := collectDiskOnlyFiles(vault, "notes", "other", nil, location)
	if err != nil || len(files) != 1 || files[0].from != "notes/ordinary.txt" {
		t.Fatalf("disk-only=%+v err=%v", files, err)
	}
	if err := cleanupDeletedDirectories(vault, []string{"notes"}, nil, newVaultDiskPathResolver(vault), location); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-journal", "-wal", "-shm", ".tmp-foreign"} {
		if content, err := os.ReadFile(location.DBPath + suffix); err != nil || string(content) != "keep" {
			t.Fatalf("auxiliary %s changed: %s err=%v", suffix, content, err)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "ordinary.txt")); !os.IsNotExist(err) {
		t.Fatal("ordinary file not removed")
	}
}

func TestSelectedIndexParentAlias(t *testing.T) {
	vault := t.TempDir()
	notes := filepath.Join(vault, "notes")
	if err := os.Mkdir(notes, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notes, "A.md"), []byte("A\n"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "vault-alias")
	if err := os.Symlink(vault, alias); err != nil {
		t.Fatal(err)
	}
	location := Locations{DBPath: filepath.Join(alias, "notes", "index.sqlite")}
	if _, err := Build(vault, location); err != nil {
		t.Fatal(err)
	}
	status, err := Status(vault, location)
	if err != nil || len(status.Untracked) != 0 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if _, err := MoveDir(vault, MoveDirOptions{FromDir: "notes", ToDir: "other"}, location); err != nil {
		t.Fatal(err)
	}
	if _, err := Stats(vault, StatsOptions{}, location); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(location.DBPath); err != nil {
		t.Fatal(err)
	}
	// Put the note back under the DB directory to verify delete cleanup too.
	if _, err := MoveDir(vault, MoveDirOptions{FromDir: "other", ToDir: "notes"}, location); err != nil {
		t.Fatal(err)
	}
	if _, err := Delete(vault, DeleteOptions{Files: []string{"notes/"}, RemoveFiles: true}, location); err != nil {
		t.Fatal(err)
	}
	if _, err := Stats(vault, StatsOptions{}, location); err != nil {
		t.Fatal(err)
	}
}
