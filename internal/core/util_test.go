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
		name string
		target string
		rm     *resolveMaps
		want   bool
	}{
		{
			name:   "count <= 1 → not ambiguous",
			target: "A",
			rm:     &resolveMaps{basenameCounts: map[string]int{"a": 1}, pathSet: map[string]string{}, assetBasenameCounts: map[string]int{}, assetPathSet: map[string]string{}},
			want:   false,
		},
		{
			name:   "count == 0 → not ambiguous",
			target: "A",
			rm:     &resolveMaps{basenameCounts: map[string]int{}, pathSet: map[string]string{}, assetBasenameCounts: map[string]int{}, assetPathSet: map[string]string{}},
			want:   false,
		},
		{
			name:   "count > 1, root file exists → not ambiguous",
			target: "A",
			rm:     &resolveMaps{basenameCounts: map[string]int{"a": 2}, pathSet: map[string]string{"a": "A.md"}, assetBasenameCounts: map[string]int{}, assetPathSet: map[string]string{}},
			want:   false,
		},
		{
			name:   "count > 1, no root file → ambiguous",
			target: "A",
			rm:     &resolveMaps{basenameCounts: map[string]int{"a": 2}, pathSet: map[string]string{"a": "sub/A.md"}, assetBasenameCounts: map[string]int{}, assetPathSet: map[string]string{}},
			want:   true,
		},
		{
			name:   "case insensitive target",
			target: "Note",
			rm:     &resolveMaps{basenameCounts: map[string]int{"note": 2}, pathSet: map[string]string{"note": "Note.md"}, assetBasenameCounts: map[string]int{}, assetPathSet: map[string]string{}},
			want:   false,
		},
		{
			name:   "empty target → not ambiguous",
			target: "",
			rm:     &resolveMaps{basenameCounts: map[string]int{}, pathSet: map[string]string{}, assetBasenameCounts: map[string]int{}, assetPathSet: map[string]string{}},
			want:   false,
		},
		{
			name:   "note unique, asset ambiguous → not ambiguous (note takes priority)",
			target: "A",
			rm:     &resolveMaps{basenameCounts: map[string]int{"a": 1}, pathSet: map[string]string{}, assetBasenameCounts: map[string]int{"a": 2}, assetPathSet: map[string]string{"a": "sub/A"}},
			want:   false,
		},
		{
			name:   "no note, asset ambiguous → ambiguous",
			target: "image.png",
			rm:     &resolveMaps{basenameCounts: map[string]int{}, pathSet: map[string]string{}, assetBasenameCounts: map[string]int{"image.png": 2}, assetPathSet: map[string]string{"image.png": "sub/image.png"}},
			want:   true,
		},
		{
			name:   "no note, asset unique → not ambiguous",
			target: "image.png",
			rm:     &resolveMaps{basenameCounts: map[string]int{}, pathSet: map[string]string{}, assetBasenameCounts: map[string]int{"image.png": 1}, assetPathSet: map[string]string{}},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAmbiguousBasenameLink(tt.target, tt.rm)
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
