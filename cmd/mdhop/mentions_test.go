package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mentionsCLIVault(t *testing.T) string {
	t.Helper()
	vault := t.TempDir()
	files := map[string]string{
		"Plan.md":  "Alpha Ghost\n",
		"Alpha.md": "see Plan\n",
		"Other.md": "[[Ghost]]\n",
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(vault, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := runBuild([]string{"--vault", vault}); err != nil {
		t.Fatal(err)
	}
	return vault
}

func TestMentionsJSONShape(t *testing.T) {
	vault := mentionsCLIVault(t)
	out := captureStdout(t, func() error {
		return runMentions([]string{"--vault", vault, "--file", "Plan.md", "--format", "json"})
	})
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parse %q: %v", out, err)
	}
	want := map[string]string{
		"entry":    `{"type":"note","name":"Plan","path":"Plan.md","exists":true}`,
		"incoming": `[{"type":"note","name":"Alpha","path":"Alpha.md","exists":true,"matched_text":"Plan","location":{"start":{"line":1,"column":5},"end":{"line":1,"column":9}},"excerpt":"see Plan"}]`,
		"outgoing": `[{"type":"note","name":"Alpha","path":"Alpha.md","exists":true,"matched_text":"Alpha","location":{"start":{"line":1,"column":1},"end":{"line":1,"column":6}},"excerpt":"Alpha Ghost"},{"type":"phantom","name":"Ghost","matched_text":"Ghost","location":{"start":{"line":1,"column":7},"end":{"line":1,"column":12}},"excerpt":"Alpha Ghost"}]`,
		"total":    `{"incoming":1,"outgoing":2}`,
	}
	if len(got) != len(want) {
		t.Fatalf("keys: %s", out)
	}
	for key, w := range want {
		var compact bytes.Buffer
		if err := json.Compact(&compact, got[key]); err != nil {
			t.Fatal(err)
		}
		if compact.String() != w {
			t.Fatalf("%s: %s", key, compact.String())
		}
	}
}

func TestMentionsFilterFlags(t *testing.T) {
	vault := mentionsCLIVault(t)
	for _, name := range []string{"Beta.md", "Gamma.md"} {
		if err := os.WriteFile(filepath.Join(vault, name), []byte("Plan\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := runBuild([]string{"--vault", vault}); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() error {
		return runMentions([]string{"--vault", vault, "--file", "Plan.md", "--format", "json",
			"--path", "*a.md", "--hide-path", "Alpha.md", "--hide-path", "Gamma.md", "--no-phantom"})
	})
	var got struct {
		Incoming []struct{ Path string }
		Outgoing []struct{ Path string }
		Total    struct{ Incoming, Outgoing int }
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parse %q: %v", out, err)
	}
	if len(got.Incoming) != 1 || got.Incoming[0].Path != "Beta.md" || len(got.Outgoing) != 0 ||
		got.Total.Incoming != 1 || got.Total.Outgoing != 0 {
		t.Fatalf("filtered: %s", out)
	}
}

func TestMentionsWhereFlag(t *testing.T) {
	vault := mentionsCLIVault(t)
	files := map[string]string{
		"Plan.md":  "Alpha Beta Gamma Ghost\n",
		"Alpha.md": "---\nstatus: active\ntype: concept\n---\nsee Plan\n",
		"Beta.md":  "---\nstatus: active\ntype: log\n---\nPlan\n",
		"Gamma.md": "---\nstatus: stale\ntype: concept\n---\nPlan\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(vault, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := runBuild([]string{"--vault", vault}); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() error {
		return runMentions([]string{"--vault", vault, "--file", "Plan.md", "--format", "json",
			"--where", "status = active", "--where", "type = concept"})
	})
	var got struct {
		Incoming []struct{ Path string }
		Outgoing []struct{ Type, Path string }
		Total    struct{ Incoming, Outgoing int }
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parse %q: %v", out, err)
	}
	if len(got.Incoming) != 1 || got.Incoming[0].Path != "Alpha.md" ||
		len(got.Outgoing) != 1 || got.Outgoing[0].Type != "note" || got.Outgoing[0].Path != "Alpha.md" ||
		got.Total.Incoming != 1 || got.Total.Outgoing != 1 {
		t.Fatalf("where: %s", out)
	}
}

func TestMentionsPhantomTextOmitsOutgoing(t *testing.T) {
	vault := mentionsCLIVault(t)
	out := captureStdout(t, func() error {
		return runMentions([]string{"--vault", vault, "--phantom", "Ghost", "--limit", "1"})
	})
	want := "entry:\n  phantom \"Ghost\"\nincoming:\n  note \"Plan.md\"\n    matched_text: \"Ghost\"\n    location: 1:7-1:12\n    excerpt: \"Alpha Ghost\"\ntotal:\n  incoming: 1\n"
	if out != want {
		t.Fatalf("text:\n%q\nwant\n%q", out, want)
	}
	out = captureStdout(t, func() error {
		return runMentions([]string{"--vault", vault, "--phantom", "Ghost", "--format", "json"})
	})
	if strings.Contains(out, "outgoing") {
		t.Fatalf("phantom JSON has outgoing: %s", out)
	}
}

func TestMentionsRelationsFlag(t *testing.T) {
	vault := mentionsCLIVault(t)
	out := captureStdout(t, func() error {
		return runMentions([]string{"--vault", vault, "--file", "Plan.md", "--format", "json", "--relations", " outgoing "})
	})
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parse %q: %v", out, err)
	}
	var total bytes.Buffer
	if err := json.Compact(&total, got["total"]); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["incoming"]; ok || total.String() != `{"outgoing":2}` {
		t.Fatalf("outgoing JSON: %s", out)
	}
	out = captureStdout(t, func() error {
		return runMentions([]string{"--vault", vault, "--file", "Plan.md", "--relations", "incoming"})
	})
	want := "entry:\n  note \"Plan.md\"\nincoming:\n  note \"Alpha.md\"\n    matched_text: \"Plan\"\n    location: 1:5-1:9\n    excerpt: \"see Plan\"\ntotal:\n  incoming: 1\n"
	if out != want {
		t.Fatalf("text:\n%q\nwant\n%q", out, want)
	}
	// Both directions in any order and spacing match the default output.
	def := captureStdout(t, func() error {
		return runMentions([]string{"--vault", vault, "--file", "Plan.md", "--format", "json"})
	})
	out = captureStdout(t, func() error {
		return runMentions([]string{"--vault", vault, "--file", "Plan.md", "--format", "json", "--relations", "outgoing , incoming"})
	})
	if out != def {
		t.Fatalf("both:\n%s\nwant\n%s", out, def)
	}
}

func TestMentionsSortFlag(t *testing.T) {
	vault := mentionsCLIVault(t)
	files := map[string]string{
		"Plan.md":  "Alpha Beta Ghost\n",
		"Alpha.md": "---\ndate: 2026-01-01\n---\nsee Plan\n",
		"Beta.md":  "---\ndate: 2026-02-01\n---\nPlan\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(vault, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := runBuild([]string{"--vault", vault}); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() error {
		return runMentions([]string{"--vault", vault, "--file", "Plan.md", "--format", "json", "--sort", "-meta.date", "--limit", "2"})
	})
	var got struct {
		Incoming []struct{ Path string }
		Outgoing []struct{ Type, Name string }
		Total    struct{ Incoming, Outgoing int }
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parse %q: %v", out, err)
	}
	if len(got.Incoming) != 2 || got.Incoming[0].Path != "Beta.md" || got.Incoming[1].Path != "Alpha.md" ||
		len(got.Outgoing) != 2 || got.Outgoing[0].Name != "Beta" || got.Outgoing[1].Name != "Alpha" ||
		got.Total.Incoming != 2 || got.Total.Outgoing != 3 {
		t.Fatalf("sorted: %s", out)
	}
}

func TestMentionsCLIErrors(t *testing.T) {
	vault := mentionsCLIVault(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "no entry specified"},
		{[]string{"--file", "Plan.md", "--limit", "0"}, "limit must be positive"},
		{[]string{"--file", "Plan.md", "--path", "a[b]"}, "unsupported glob pattern (character class): a[b]"},
		{[]string{"--file", "Plan.md", "--hide-path", "a[b]"}, "query hide: unsupported glob pattern (character class): a[b]"},
		{[]string{"--file", "Plan.md", "--where", "a=1 && b=2 || c=3"}, "cannot mix && and ||"},
		{[]string{"--file", "Plan.md", "--relations", ""}, "relations: select at least one relation"},
		{[]string{"--file", "Plan.md", "--relations", "bad"}, `relations: unknown relation "bad"`},
		{[]string{"--file", "Plan.md", "--relations", "incoming,incoming"}, `relations: duplicate relation "incoming"`},
		{[]string{"--phantom", "Ghost", "--relations", "outgoing"}, "relations: outgoing requires a --file entry"},
		{[]string{"--file", "Plan.md", "--sort", "-"}, "empty sort key"},
	} {
		err := runMentions(append([]string{"--vault", vault}, tc.args...))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%v: err=%v, want %q", tc.args, err, tc.want)
		}
	}
	// Relations are validated before the index is opened.
	if err := runMentions([]string{"--vault", t.TempDir(), "--file", "Plan.md", "--relations", "bad"}); err == nil ||
		err.Error() != `relations: unknown relation "bad"` {
		t.Fatalf("no index: err=%v", err)
	}
}
