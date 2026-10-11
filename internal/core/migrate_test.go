package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConvertLegacyConfig(t *testing.T) {
	data := []byte(`build:
  exclude_paths: ['private/**']
exclude:
  paths: ['archive/**']
  tags: [hidden]
query:
  hide:
    paths: ['secret/**']
    tags: [private]
  via:
    include:
      paths: ['public/**']
      tags: [topic]
    exclude: {}
meta:
  types:
    title: string
    score: number
    created: date
    version: semver
    priority: {ordered: [low, high]}
  profiles:
    - path: 'public/**'
      require: [title, score]
  link_keys: [parent]
`)
	converted, err := convertLegacyConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := decodeConfig(converted, "test")
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Build:   BuildConfig{ExcludePaths: []string{"private/**"}},
		Exclude: ExcludeConfig{Paths: []string{"archive/**"}, Tags: []string{"hidden"}},
		Query:   QueryConfig{Hide: ExcludeConfig{Paths: []string{"secret/**"}, Tags: []string{"private"}}, Via: QueryViaConfig{Include: ExcludeConfig{Paths: []string{"public/**"}, Tags: []string{"topic"}}, Exclude: &ExcludeConfig{}}},
		Meta:    MetaConfig{Types: map[string]MetaTypeInfo{"title": {Name: MetaTypeString}, "score": {Name: MetaTypeNumber}, "created": {Name: MetaTypeDate}, "version": {Name: MetaTypeSemver}, "priority": {Name: MetaTypeOrdered, OrderedValues: []string{"low", "high"}}}, Profiles: []MetaRequireProfile{{Path: "public/**", Require: []string{"title", "score"}}}, LinkKeys: []string{"parent"}},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got %#v, want %#v; TOML:\n%s", cfg, want, converted)
	}
	for _, input := range []string{"", "# empty\n", "{}", "null", "exclude: {tags: null}", "query: {via: {include: {}}}", "exclude: {tags: &tags [private]}\nquery: {hide: {tags: *tags}}"} {
		converted, err := convertLegacyConfig([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := decodeConfig(converted, "test")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Query.Via.Exclude != nil {
			t.Fatal("absent via.exclude became explicit")
		}
	}
}

func TestConvertLegacyConfigScalarStrings(t *testing.T) {
	converted, err := convertLegacyConfig([]byte("exclude: {tags: [2025, true, null]}\n"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := decodeConfig(converted, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Exclude.Tags, []string{"2025", "true", ""}) {
		t.Fatalf("tags: %v", cfg.Exclude.Tags)
	}
}

func TestConvertLegacyConfigRejectsLoss(t *testing.T) {
	for _, input := range []string{
		"unknown: 1", "build: {unknown: []}", "query: {via: {unknown: {}}}",
		"query: {via: {exclude: null}}", "meta: {types: {x: {ordered: [low], extra: []}}}",
		"meta: {types: {x: boolean}}", "meta: {types: {x: {ordered: [low, low]}}}",
		"meta: {link_keys: [tags]}", "meta: {profiles: [{require: []}]}",
		"build: {exclude_paths: '['}",
		"exclude: {paths: ['[a]']}",
		"query: {hide: {paths: ['[a]']}}",
		"query: {via: {include: {paths: ['[a]']}}}",
		"query: {via: {exclude: {paths: ['[a]']}}}",
		"exclude: {paths: ['[a]']}\nquery: {via: {exclude: {}}}",
		"---\n{}\n---\n{}", "meta: {types: {x: 3}}",
		"query: {hide: {<<: {tags: [a]}}}", "exclude: {}\nexclude: {}", "x: &x {x: *x}",
	} {
		t.Run(input, func(t *testing.T) {
			if data, err := convertLegacyConfig([]byte(input)); err == nil {
				t.Fatalf("accepted %q: %s", input, data)
			}
		})
	}
}

func migrationVault(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	vault := t.TempDir()
	mustMigrationWrite(t, filepath.Join(vault, "A.md"), "[[mdhop.yaml]] [[mdhop.toml]]\n")
	if err := os.Mkdir(filepath.Join(vault, ".mdhop"), 0700); err != nil {
		t.Fatal(err)
	}
	mustMigrationWrite(t, filepath.Join(vault, ".mdhop", "index.sqlite"), "old index")
	return vault
}
func mustMigrationWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func assertMigrationFile(t *testing.T, path, content string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != content {
		t.Fatalf("%s = %q, %v", path, data, err)
	}
}

func TestMigrateFinalIndex(t *testing.T) {
	for _, mode := range []string{"yaml", "toml", "none"} {
		t.Run(mode, func(t *testing.T) {
			vault := migrationVault(t)
			if mode == "yaml" {
				legacy := filepath.Join(vault, "mdhop.yaml")
				mustMigrationWrite(t, legacy, "query: {via: {exclude: {}}}\n")
				if err := os.Chmod(legacy, 0600); err != nil {
					t.Fatal(err)
				}
			}
			original := "# preserved formatting\n[query.via.exclude]\n"
			if mode == "toml" {
				mustMigrationWrite(t, filepath.Join(vault, "mdhop.toml"), original)
			}
			if _, err := Migrate(vault); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"mdhop.yaml", ".mdhop"} {
				if _, err := os.Lstat(filepath.Join(vault, name)); !os.IsNotExist(err) {
					t.Fatalf("%s remains: %v", name, err)
				}
			}
			if mode == "yaml" {
				info, err := os.Stat(filepath.Join(vault, "mdhop.toml"))
				if err != nil {
					t.Fatal(err)
				}
				if got := info.Mode().Perm(); got != 0600 {
					t.Fatalf("permissions = %o, want 600", got)
				}
				cfg, err := LoadConfig(vault)
				if err != nil || cfg.Query.Via.Exclude == nil {
					t.Fatalf("migrated configuration: %+v, %v", cfg, err)
				}
			}
			if mode == "toml" {
				assertMigrationFile(t, filepath.Join(vault, "mdhop.toml"), original)
			}
			if mode == "none" {
				if _, err := os.Stat(filepath.Join(vault, "mdhop.toml")); !os.IsNotExist(err) {
					t.Fatal("created unwanted config")
				}
			}
			status, err := Status(vault)
			if err != nil {
				t.Fatal(err)
			}
			if len(status.Deleted)+len(status.Untracked)+len(status.Modified) != 0 {
				t.Fatalf("status: %+v", status)
			}
			snapshot := func() []string {
				dbp, err := resolveDBPath(vault)
				if err != nil {
					t.Fatal(err)
				}
				db := openTestDB(t, dbp)
				defer db.Close()
				rows, err := db.Query("SELECT type || ':' || name || ':' || COALESCE(path, '') FROM nodes ORDER BY type,name,path")
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
				var got []string
				for rows.Next() {
					var value string
					if err := rows.Scan(&value); err != nil {
						t.Fatal(err)
					}
					got = append(got, value)
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				return got
			}
			resolved, err := Resolve(vault, "A.md", "[[mdhop.yaml]]")
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Type != NodeTypePhantom {
				t.Fatalf("removed YAML resolution: %+v", resolved)
			}
			before := snapshot()
			if !strings.Contains(strings.Join(before, "\n"), "phantom:mdhop.yaml:") {
				t.Fatalf("deleted YAML is not phantom: %v", before)
			}
			if mode != "none" && !strings.Contains(strings.Join(before, "\n"), "asset:mdhop.toml:mdhop.toml") {
				t.Fatalf("missing TOML asset: %v", before)
			}
			if _, err := Build(vault); err != nil {
				t.Fatal(err)
			}
			if after := snapshot(); !reflect.DeepEqual(before, after) {
				t.Fatalf("migration %v != normal build %v", before, after)
			}
		})
	}
}

func TestMigrateFailuresRetainLegacy(t *testing.T) {
	invalidGlobs := map[string]string{
		"exclude-glob":        "exclude: {paths: ['[a]']}\n",
		"hide-glob":           "query: {hide: {paths: ['[a]']}}\n",
		"include-glob":        "query: {via: {include: {paths: ['[a]']}}}\n",
		"via-exclude-glob":    "query: {via: {exclude: {paths: ['[a]']}}}\n",
		"unused-exclude-glob": "exclude: {paths: ['[a]']}\nquery: {via: {exclude: {}}}\n",
	}
	for _, mode := range []string{"conflict", "invalid", "save", "build", "cache", "cache-alias", "config-symlink", "exclude-glob", "hide-glob", "include-glob", "via-exclude-glob", "unused-exclude-glob"} {
		t.Run(mode, func(t *testing.T) {
			vault := migrationVault(t)
			yamlPath := filepath.Join(vault, "mdhop.yaml")
			original := "exclude: {tags: [hidden]}\n"
			if mode == "invalid" {
				original = "unsupported: true\n"
			}
			if input, ok := invalidGlobs[mode]; ok {
				original = input
			}
			mustMigrationWrite(t, yamlPath, original)
			dbp, err := resolveDBPath(vault)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(dbp), 0700); err != nil {
				t.Fatal(err)
			}
			mustMigrationWrite(t, dbp, "existing cache")
			switch mode {
			case "conflict":
				mustMigrationWrite(t, filepath.Join(vault, "mdhop.toml"), "# existing")
			case "save":
				if err := os.Chmod(vault, 0500); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(vault, 0700)
				if os.Geteuid() == 0 {
					t.Skip("permission checks require non-root")
				}
			case "build":
				mustMigrationWrite(t, filepath.Join(vault, "A.md"), "[[../outside]]\n")
			case "cache":
				t.Setenv("XDG_CACHE_HOME", filepath.Join(vault, ".mdhop"))
			case "cache-alias":
				alias := filepath.Join(t.TempDir(), "alias")
				if err := os.Symlink(filepath.Join(vault, ".mdhop"), alias); err != nil {
					t.Fatal(err)
				}
				t.Setenv("XDG_CACHE_HOME", alias)
			case "config-symlink":
				target := filepath.Join(t.TempDir(), "target")
				mustMigrationWrite(t, target, "untouched")
				if err := os.Symlink(target, filepath.Join(vault, "mdhop.toml")); err != nil {
					t.Fatal(err)
				}
				defer assertMigrationFile(t, target, "untouched")
			}
			if _, err := Migrate(vault); err == nil {
				t.Fatal("expected migration failure")
			}
			assertMigrationFile(t, yamlPath, original)
			assertMigrationFile(t, filepath.Join(vault, ".mdhop", "index.sqlite"), "old index")
			assertMigrationFile(t, dbp, "existing cache")
			if mode == "build" || invalidGlobs[mode] != "" {
				if _, err := os.Lstat(filepath.Join(vault, "mdhop.toml")); !os.IsNotExist(err) {
					t.Fatal("generated TOML not rolled back")
				}
			}
			temps, err := filepath.Glob(dbp + ".tmp-*")
			if err != nil || len(temps) != 0 {
				t.Fatalf("temporary DBs: %v %v", temps, err)
			}
		})
	}
}

func TestMigrateCleanupFailureReportsState(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks require non-root")
	}
	vault := migrationVault(t)
	mustMigrationWrite(t, filepath.Join(vault, "mdhop.yaml"), "{}")
	oldDir := filepath.Join(vault, ".mdhop")
	if err := os.Chmod(oldDir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(oldDir, 0700)
	_, err := Migrate(vault)
	if err == nil || !strings.Contains(err.Error(), "index rebuilt") || !strings.Contains(err.Error(), oldDir) {
		t.Fatalf("cleanup error: %v", err)
	}
	assertMigrationFile(t, filepath.Join(oldDir, "index.sqlite"), "old index")
	if _, err := os.Stat(filepath.Join(vault, "mdhop.toml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Status(vault); err != nil {
		t.Fatal(err)
	}
}
