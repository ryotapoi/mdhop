package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandMoveTemplate_BacklogExample(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"Projects/Alpha.v1.md": "---\nclient: Acme\nupdated: 2026-07-04\ntags: [project, active]\n---\n# Alpha\n",
	})

	to, err := ExpandMoveTemplate(vault, MoveTemplateOptions{
		From:     "Projects/Alpha.v1.md",
		Template: "99-Archive/02-Projects/{client|others}/{updated:year}/{basename}",
	})
	if err != nil {
		t.Fatalf("expand template: %v", err)
	}
	want := "99-Archive/02-Projects/Acme/2026/Alpha.v1.md"
	if to != want {
		t.Fatalf("expanded path = %q, want %q", to, want)
	}

	if _, err := Move(vault, MoveOptions{From: "Projects/Alpha.v1.md", To: to}); err != nil {
		t.Fatalf("move expanded path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, want)); err != nil {
		t.Fatalf("expanded destination should exist: %v", err)
	}
}

func TestExpandMoveTemplate_FallbackLiteral(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"Project.md": "---\nupdated: 2026-07-04\n---\n# Project\n",
	})

	to, err := ExpandMoveTemplate(vault, MoveTemplateOptions{
		From:     "Project.md",
		Template: "archive/{client|others}/{basename}",
	})
	if err != nil {
		t.Fatalf("expand template: %v", err)
	}
	if to != "archive/others/Project.md" {
		t.Fatalf("expanded path = %q", to)
	}
}

func TestExpandMoveTemplate_Errors(t *testing.T) {
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
			name:     "unsafe expanded path",
			content:  "---\nclient: ../outside.md\n---\n# Project\n",
			template: "{client}",
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

			_, err := ExpandMoveTemplate(vault, MoveTemplateOptions{
				From:     "Project.md",
				Template: tt.template,
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestExpandMoveTemplate_RequiresRegisteredNote(t *testing.T) {
	vault := newMoveVault(t, map[string]string{
		"Project.md": "# Project\n",
		"image.png":  "not really a png\n",
	})

	_, err := ExpandMoveTemplate(vault, MoveTemplateOptions{
		From:     "image.png",
		Template: "archive/{basename}",
	})
	if err == nil || !strings.Contains(err.Error(), "registered note") {
		t.Fatalf("error = %v, want registered note error", err)
	}
}
