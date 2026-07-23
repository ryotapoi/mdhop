package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ryotapoi/mdhop/internal/core"
)

func TestPrintDiagnoseText_AssetConflictsAndBrokenAnchors(t *testing.T) {
	result := &core.DiagnoseResult{
		AssetBasenameConflicts: []core.BasenameConflict{{
			Name:  "logo.png",
			Paths: []string{"assets/logo.png", "images/logo.png"},
		}},
		BrokenAnchors: []core.BrokenAnchor{{
			SourcePath: "notes/Source.md",
			RawLink:    "[[Target#Missing]]",
			TargetPath: "notes/Target.md",
			Fragment:   "Missing",
		}},
	}

	var buf bytes.Buffer
	if err := printDiagnoseText(&buf, result, []string{
		core.FieldDiagnoseAssetBasenameConflicts,
		core.FieldDiagnoseAnchors,
	}); err != nil {
		t.Fatalf("printDiagnoseText: %v", err)
	}

	const want = "asset_basename_conflicts:\n" +
		"- name: logo.png\n" +
		"  paths:\n" +
		"  - assets/logo.png\n" +
		"  - images/logo.png\n" +
		"broken_anchors:\n" +
		"- source_path: notes/Source.md\n" +
		"  raw_link: \"[[Target#Missing]]\"\n" +
		"  target_path: notes/Target.md\n" +
		"  fragment: Missing\n"
	if got := buf.String(); got != want {
		t.Errorf("text output = %q, want %q", got, want)
	}
}

func TestPrintDiagnoseJSON_AssetBasenameConflicts(t *testing.T) {
	result := &core.DiagnoseResult{
		AssetBasenameConflicts: []core.BasenameConflict{{
			Name:  "logo.png",
			Paths: []string{"assets/logo.png", "images/logo.png"},
		}},
	}

	var buf bytes.Buffer
	if err := printDiagnoseJSON(&buf, result, []string{core.FieldDiagnoseAssetBasenameConflicts}); err != nil {
		t.Fatalf("printDiagnoseJSON: %v", err)
	}

	var output struct {
		AssetBasenameConflicts []diagnoseJSONConflict `json:"asset_basename_conflicts"`
	}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := output.AssetBasenameConflicts; len(got) != 1 || got[0].Name != "logo.png" || !equalStrings(got[0].Paths, []string{"assets/logo.png", "images/logo.png"}) {
		t.Errorf("asset_basename_conflicts = %#v, want [{Name:logo.png Paths:[assets/logo.png images/logo.png]}]", got)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
