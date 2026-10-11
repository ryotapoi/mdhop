package core

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddRejectsSelectedIndexResources(t *testing.T) {
	for _, suffix := range []string{"", "-journal", "-wal", "-shm", ".tmp-foreign"} {
		t.Run(suffix, func(t *testing.T) {
			vault := t.TempDir()
			for _, dir := range []string{"generated", "existing", "new"} {
				if err := os.Mkdir(filepath.Join(vault, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, text string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(vault, path), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("existing/A.md", "A\n")
			write("Ref.md", "[[A]]\n")
			location := Locations{DBPath: filepath.Join(vault, "generated/index.sqlite")}
			if _, err := Build(vault, location); err != nil {
				t.Fatal(err)
			}
			write("new/A.md", "new\n")
			resource := "generated/index.sqlite" + suffix
			if suffix != "" {
				write(resource, "auxiliary")
			}
			before, err := os.ReadFile(location.DBPath)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Add(vault, AddOptions{Files: []string{"new/A.md", resource}, AutoDisambiguate: true}, location)
			if err == nil || !strings.Contains(err.Error(), "selected index resource") {
				t.Fatalf("add error = %v", err)
			}
			after, err := os.ReadFile(location.DBPath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("DB changed: %v", err)
			}
			ref, err := os.ReadFile(filepath.Join(vault, "Ref.md"))
			if err != nil || string(ref) != "[[A]]\n" {
				t.Fatalf("reference rewritten: %q, %v", ref, err)
			}
			// Only the selected index is protected, rather than every SQLite filename.
			write("other.sqlite", "ordinary file\n")
			if _, err := Add(vault, AddOptions{Files: []string{"other.sqlite"}}, location); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRegisteredIndexResourcesRejectMutation(t *testing.T) {
	operations := []string{"delete file", "delete directory", "move file", "move directory", "template file", "template directory", "plan file", "plan directory"}
	for _, kind := range []string{"note", "asset", "auxiliary note", "auxiliary asset"} {
		for _, operation := range operations {
			if strings.Contains(kind, "asset") && (strings.HasPrefix(operation, "template") || strings.HasPrefix(operation, "plan")) {
				continue
			}
			t.Run(kind+"/"+operation, func(t *testing.T) {
				vault := t.TempDir()
				if err := os.Mkdir(filepath.Join(vault, "generated"), 0700); err != nil {
					t.Fatal(err)
				}
				for name, content := range map[string]string{"A.md": "ordinary note\n", "asset.txt": "ordinary asset"} {
					if err := os.WriteFile(filepath.Join(vault, "generated", name), []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
				location := Locations{DBPath: filepath.Join(vault, "generated/index.sqlite")}
				if _, err := Build(vault, location); err != nil {
					t.Fatal(err)
				}
				resource := "generated/index.sqlite"
				if strings.HasPrefix(kind, "auxiliary") {
					resource += ".tmp-foreign"
					if err := os.WriteFile(filepath.Join(vault, resource), []byte("foreign build"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				// Model legacy registration directly; Add must never allow this fixture.
				db, err := openDBChecked(vault, location)
				if err != nil {
					t.Fatal(err)
				}
				stamp := time.Unix(1700000000, 0)
				if strings.Contains(kind, "asset") {
					_, err = upsertAsset(db, resource, filepath.Base(resource), stamp.Unix())
				} else {
					_, err = upsertNote(db, resource, basename(resource), stamp.Unix(), 1)
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				// Make the legacy record fresh so stale detection cannot mask protection.
				if err := os.Chtimes(filepath.Join(vault, resource), stamp, stamp); err != nil {
					t.Fatal(err)
				}
				before, err := os.ReadFile(location.DBPath)
				if err != nil {
					t.Fatal(err)
				}
				resourceBefore, err := os.ReadFile(filepath.Join(vault, resource))
				if err != nil {
					t.Fatal(err)
				}
				switch operation {
				case "delete file":
					_, err = Delete(vault, DeleteOptions{Files: []string{"generated/A.md", resource}, RemoveFiles: true}, location)
				case "delete directory":
					_, err = Delete(vault, DeleteOptions{Files: []string{"generated/"}, RemoveFiles: true}, location)
				case "move file":
					_, err = Move(vault, MoveOptions{From: resource, To: "other/index.sqlite"}, location)
				case "move directory":
					_, err = MoveDir(vault, MoveDirOptions{FromDir: "generated", ToDir: "other"}, location)
				default:
					opts := MoveTemplateOptions{From: resource, Template: "other/{basename}"}
					if strings.HasSuffix(operation, "directory") {
						opts.From = "generated/"
						opts.Directory = true
					}
					if strings.HasPrefix(operation, "plan") {
						_, err = PlanMoveTemplate(vault, opts, location)
					} else {
						_, err = MoveTemplate(vault, opts, location)
					}
				}
				if err == nil || !strings.Contains(err.Error(), "selected index resource") {
					t.Fatalf("mutation error = %v", err)
				}
				after, err := os.ReadFile(location.DBPath)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("DB changed: %v", err)
				}
				resourceAfter, err := os.ReadFile(filepath.Join(vault, resource))
				if err != nil || !bytes.Equal(resourceBefore, resourceAfter) {
					t.Fatalf("index resource changed: %v", err)
				}
				for name, want := range map[string]string{"A.md": "ordinary note\n", "asset.txt": "ordinary asset"} {
					got, err := os.ReadFile(filepath.Join(vault, "generated", name))
					if err != nil || string(got) != want {
						t.Fatalf("%s changed: %q, %v", name, got, err)
					}
				}
				if _, err := os.Stat(filepath.Join(vault, "other")); !os.IsNotExist(err) {
					t.Fatalf("destination created: %v", err)
				}
				db, err = openDBChecked(vault, location)
				if err != nil {
					t.Fatalf("DB cannot be reopened: %v", err)
				}
				defer db.Close()
				var count int
				if err := db.QueryRow("SELECT count(*) FROM nodes WHERE exists_flag = 1").Scan(&count); err != nil || count != 3 {
					t.Fatalf("registered count = %d, %v", count, err)
				}
			})
		}
	}
}
