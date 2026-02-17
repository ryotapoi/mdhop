package core

import "testing"

func TestIsRootFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"A.md", true},
		{"sub/A.md", false},
		{"sub/deep/A.md", false},
		{"README.md", true},
	}
	for _, tt := range tests {
		if got := isRootFile(tt.path); got != tt.want {
			t.Errorf("isRootFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsAmbiguousBasenameLink(t *testing.T) {
	tests := []struct {
		name           string
		target         string
		basenameCounts map[string]int
		pathSet        map[string]string
		want           bool
	}{
		{
			name:           "count <= 1 → not ambiguous",
			target:         "A",
			basenameCounts: map[string]int{"a": 1},
			pathSet:        map[string]string{},
			want:           false,
		},
		{
			name:           "count == 0 → not ambiguous",
			target:         "A",
			basenameCounts: map[string]int{},
			pathSet:        map[string]string{},
			want:           false,
		},
		{
			name:           "count > 1, root file exists → not ambiguous",
			target:         "A",
			basenameCounts: map[string]int{"a": 2},
			pathSet:        map[string]string{"a": "A.md"},
			want:           false,
		},
		{
			name:           "count > 1, no root file → ambiguous",
			target:         "A",
			basenameCounts: map[string]int{"a": 2},
			pathSet:        map[string]string{"a": "sub/A.md"},
			want:           true,
		},
		{
			name:           "case insensitive target",
			target:         "Note",
			basenameCounts: map[string]int{"note": 2},
			pathSet:        map[string]string{"note": "Note.md"},
			want:           false,
		},
		{
			name:           "empty target → not ambiguous",
			target:         "",
			basenameCounts: map[string]int{},
			pathSet:        map[string]string{},
			want:           false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAmbiguousBasenameLink(tt.target, tt.basenameCounts, tt.pathSet)
			if got != tt.want {
				t.Errorf("isAmbiguousBasenameLink(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

func TestHasRootInPathSet(t *testing.T) {
	tests := []struct {
		name    string
		bk      string
		pathSet map[string]string
		want    bool
	}{
		{"root file exists", "a", map[string]string{"a": "A.md"}, true},
		{"subdir file", "a", map[string]string{"a": "sub/A.md"}, false},
		{"key missing", "a", map[string]string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasRootInPathSet(tt.bk, tt.pathSet)
			if got != tt.want {
				t.Errorf("hasRootInPathSet(%q) = %v, want %v", tt.bk, got, tt.want)
			}
		})
	}
}
