package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMoveTemplate_BacklogExample(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"Projects/Alpha.v1.md": "---\nclient: Acme\nupdated: 2026-07-04\ntags: [project, active]\n---\n# Alpha\n",
	})

	result, err := MoveTemplate(vault, MoveTemplateOptions{
		From:     "Projects/Alpha.v1.md",
		Template: "99-Archive/02-Projects/{client|others}/{updated:year}/{basename}",
	})
	if err != nil {
		t.Fatalf("move template: %v", err)
	}
	want := "99-Archive/02-Projects/Acme/2026/Alpha.v1.md"
	if len(result.Moved) != 1 || result.Moved[0] != (MovedFile{From: "Projects/Alpha.v1.md", To: want}) {
		t.Fatalf("moved = %+v, want one move to %q", result.Moved, want)
	}
	if _, err := os.Stat(filepath.Join(vault, want)); err != nil {
		t.Fatalf("expanded destination should exist: %v", err)
	}
}

func TestPlanMoveTemplate_DatePartsUseNormalizedSortValue(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"mdhop.yaml": "meta:\n  types:\n    updated: date\n",
		"Project.md": "---\nupdated: 2026/7/4\n---\n# Project\n",
	})

	plan, err := PlanMoveTemplate(vault, MoveTemplateOptions{
		From:     "Project.md",
		Template: "archive/{updated:year}/{updated:month}/{updated:day}/{basename}",
	})
	if err != nil {
		t.Fatalf("plan move template: %v", err)
	}
	if len(plan.Moved) != 1 || plan.Moved[0] != (MovedFile{From: "Project.md", To: "archive/2026/07/04/Project.md"}) {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestPlanMoveTemplate_FallbackLiteral(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"Project.md": "---\nupdated: 2026-07-04\n---\n# Project\n",
	})

	plan, err := PlanMoveTemplate(vault, MoveTemplateOptions{
		From:     "Project.md",
		Template: "archive/{client|others}/{basename}",
	})
	if err != nil {
		t.Fatalf("plan move template: %v", err)
	}
	if len(plan.Moved) != 1 || plan.Moved[0] != (MovedFile{From: "Project.md", To: "archive/others/Project.md"}) {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestPlanMoveTemplate_DatePartFallbackOnlyWhenMissing(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"Missing.md": "---\nupdated: 2026-07-04\n---\n# Missing\n",
		"Invalid.md": "---\nclosed: someday\n---\n# Invalid\n",
	})

	plan, err := PlanMoveTemplate(vault, MoveTemplateOptions{
		From:     "Missing.md",
		Template: "archive/{closed:year|2099}/{basename}",
	})
	if err != nil {
		t.Fatalf("plan move template with fallback: %v", err)
	}
	if len(plan.Moved) != 1 || plan.Moved[0] != (MovedFile{From: "Missing.md", To: "archive/2099/Missing.md"}) {
		t.Fatalf("plan = %+v", plan)
	}

	_, err = PlanMoveTemplate(vault, MoveTemplateOptions{
		From:     "Invalid.md",
		Template: "archive/{closed:year|2099}/{basename}",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be parsed as date") {
		t.Fatalf("error = %v, want invalid date error", err)
	}
}

func TestPlanMoveTemplate_Errors(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		template string
		wantErr  string
	}{
		{
			name:     "missing field without fallback",
			content:  "---\nupdated: 2026-07-04\n---\n# Project\n",
			template: "archive/{client}/{basename}",
			wantErr:  "template field missing: client",
		},
		{
			name:     "invalid date year extraction",
			content:  "---\nclient: Acme\nupdated: someday\n---\n# Project\n",
			template: "archive/{client}/{updated:year}/{basename}",
			wantErr:  "cannot be parsed as date",
		},
		{
			name:     "placeholder value contains slash",
			content:  "---\nclient: Acme/Research\n---\n# Project\n",
			template: "{client}",
			wantErr:  "placeholder value contains /",
		},
		{
			name:     "template literal escapes vault",
			content:  "---\nclient: Acme\n---\n# Project\n",
			template: "../{basename}",
			wantErr:  "escapes vault",
		},
		{
			name:     "multiple referenced field values",
			content:  "---\nclient: [Acme, Beta]\n---\n# Project\n",
			template: "archive/{client}/{basename}",
			wantErr:  "multiple values: client",
		},
		{
			name:     "unterminated placeholder",
			content:  "---\nclient: Acme\n---\n# Project\n",
			template: "archive/{client",
			wantErr:  "unterminated template placeholder",
		},
		{
			name:     "nested placeholder",
			content:  "---\nclient: Acme\n---\n# Project\n",
			template: "archive/{client/{basename}",
			wantErr:  "nested template placeholder",
		},
		{
			name:     "empty expanded path",
			content:  "---\nclient: Acme\n---\n# Project\n",
			template: ".",
			wantErr:  "expanded destination path is empty",
		},
		{
			name:     "directory expanded path",
			content:  "---\nclient: archive\n---\n# Project\n",
			template: "{client}/",
			wantErr:  "expanded destination path must be a file path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vault := newMoveVault(t, map[string]string{
				"Project.md": tt.content,
			})

			_, err := PlanMoveTemplate(vault, MoveTemplateOptions{
				From:     "Project.md",
				Template: tt.template,
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestMoveTemplate_DirectoryModeRejectsDuplicateDestination(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"src/A.md": "---\nclient: Acme\n---\n# A\n",
		"src/B.md": "---\nclient: Acme\n---\n# B\n",
	})

	_, err := MoveTemplate(vault, MoveTemplateOptions{
		From:      "src/",
		Template:  "archive/{client}/Project.md",
		Directory: true,
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate expanded destination path") {
		t.Fatalf("error = %v, want duplicate destination", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "src", "A.md")); err != nil {
		t.Fatalf("src/A.md should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "src", "B.md")); err != nil {
		t.Fatalf("src/B.md should remain: %v", err)
	}
}

func TestMoveTemplate_DirectoryModeMovesAllNotes(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"src/A.md": "---\nclient: Acme\nupdated: 2026-07-04\n---\n# A\n",
		"src/B.md": "---\nclient: Beta\nupdated: 2026-08-05\n---\n# B\n",
	})

	result, err := MoveTemplate(vault, MoveTemplateOptions{
		From:      "src/",
		Template:  "archive/{client}/{updated:year}/{basename}",
		Directory: true,
	})
	if err != nil {
		t.Fatalf("move template dir: %v", err)
	}
	wantMoved := []MovedFile{
		{From: "src/A.md", To: "archive/Acme/2026/A.md"},
		{From: "src/B.md", To: "archive/Beta/2026/B.md"},
	}
	if len(result.Moved) != len(wantMoved) {
		t.Fatalf("moved = %+v, want %+v", result.Moved, wantMoved)
	}
	for i := range wantMoved {
		if result.Moved[i] != wantMoved[i] {
			t.Fatalf("moved[%d] = %+v, want %+v", i, result.Moved[i], wantMoved[i])
		}
		if _, err := os.Stat(filepath.Join(vault, wantMoved[i].To)); err != nil {
			t.Fatalf("%s should exist: %v", wantMoved[i].To, err)
		}
	}
}

func TestMoveTemplate_DirectoryModePrevalidatesAllNotes(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"src/A.md": "---\nclient: Acme\n---\n# A\n",
		"src/B.md": "---\n---\n# B\n",
	})

	_, err := MoveTemplate(vault, MoveTemplateOptions{
		From:      "src/",
		Template:  "archive/{client}/{basename}",
		Directory: true,
	})
	if err == nil || !strings.Contains(err.Error(), "template field missing: client") {
		t.Fatalf("error = %v, want missing field", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "src", "A.md")); err != nil {
		t.Fatalf("src/A.md should remain after failed prevalidation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "archive", "Acme", "A.md")); !os.IsNotExist(err) {
		t.Fatalf("archive/Acme/A.md should not be created, err=%v", err)
	}
}

func TestPlanMoveTemplate_DoesNotChangeDiskOrDB(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"Project.md": "---\nclient: Acme\n---\n# Project\n",
	})

	plan, err := PlanMoveTemplate(vault, MoveTemplateOptions{
		From:     "Project.md",
		Template: "archive/{client}/{basename}",
	})
	if err != nil {
		t.Fatalf("plan move template: %v", err)
	}
	if len(plan.Moved) != 1 || plan.Moved[0].To != "archive/Acme/Project.md" {
		t.Fatalf("plan = %+v", plan)
	}
	if _, err := os.Stat(filepath.Join(vault, "Project.md")); err != nil {
		t.Fatalf("Project.md should remain after planning: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "archive", "Acme", "Project.md")); !os.IsNotExist(err) {
		t.Fatalf("archive/Acme/Project.md should not exist after planning, err=%v", err)
	}
	if _, err := Query(vault, EntrySpec{File: "Project.md"}, QueryOptions{}); err != nil {
		t.Fatalf("Project.md should remain in DB after planning: %v", err)
	}
}

func TestPlanMoveTemplate_PreflightsRewriteCandidatesLikeExecution(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{
			name: "unsafe external incoming rewrite",
			files: map[string]string{
				"B.md":     "# B\n",
				"Other.md": "---\nref: \"\\u005b\\u005bB\\u005d\\u005d\"\n---\n",
			},
		},
		{
			name: "unsafe moved note outgoing rewrite",
			files: map[string]string{
				"B.md":      "---\nref: \"\\u005b\\u005b./Target\\u005d\\u005d\"\n---\n",
				"Target.md": "# Target\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vault := newMoveVault(t, tt.files)
			originalB := readVaultFile(t, vault, "B.md")
			originalDB := queryNodes(t, dbPath(vault), NodeTypeNote)
			opts := MoveTemplateOptions{From: "B.md", Template: "archive/Renamed.md"}

			if _, err := PlanMoveTemplate(vault, opts); err == nil || !strings.Contains(err.Error(), "correspondence") {
				t.Fatalf("plan error = %v, want correspondence rejection", err)
			}
			if got := readVaultFile(t, vault, "B.md"); got != originalB {
				t.Fatalf("B.md changed during rejected plan:\n%s", got)
			}
			if _, err := os.Stat(filepath.Join(vault, "archive")); !os.IsNotExist(err) {
				t.Fatalf("archive directory exists after rejected plan, err=%v", err)
			}
			if got := queryNodes(t, dbPath(vault), NodeTypeNote); !reflect.DeepEqual(got, originalDB) {
				t.Fatalf("DB changed during rejected plan: got %+v, want %+v", got, originalDB)
			}

			if _, err := MoveTemplate(vault, opts); err == nil || !strings.Contains(err.Error(), "correspondence") {
				t.Fatalf("execution error = %v, want correspondence rejection", err)
			}
			if got := readVaultFile(t, vault, "B.md"); got != originalB {
				t.Fatalf("B.md changed during rejected execution:\n%s", got)
			}
		})
	}
}

func TestPlanMoveTemplate_AllowsSafeRewriteCandidate(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"B.md":     "# B\n",
		"Other.md": "---\nref: \"[[B]]\"\n---\n",
	})
	opts := MoveTemplateOptions{From: "B.md", Template: "archive/Renamed.md"}

	if _, err := PlanMoveTemplate(vault, opts); err != nil {
		t.Fatalf("plan move template: %v", err)
	}
	if _, err := MoveTemplate(vault, opts); err != nil {
		t.Fatalf("move template: %v", err)
	}
}

func TestPlanMoveTemplate_RequiresRegisteredNote(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"Project.md": "# Project\n",
		"image.png":  "not really a png\n",
	})

	_, err := PlanMoveTemplate(vault, MoveTemplateOptions{
		From:     "image.png",
		Template: "archive/{basename}",
	})
	if err == nil || !strings.Contains(err.Error(), "registered note") {
		t.Fatalf("error = %v, want registered note error", err)
	}
}

func TestTemplateSourceNoteError_WrappedNoRows(t *testing.T) {
	err := templateSourceNoteError(
		fmt.Errorf("lookup source note: %w", sql.ErrNoRows),
		"missing.md",
	)
	if got, want := err.Error(), "source must be a registered note for --to-template: missing.md"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
