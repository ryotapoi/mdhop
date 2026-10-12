package core

import "testing"

func TestMarkdownImageDisplayMoveAndResolve(t *testing.T) {
	raw := "[![logo](logo.png)](/long/Target.md)"
	v := newMoveVault(t, map[string]string{
		"A.md":           "# A\n" + raw + "\n",
		"long/Target.md": "# Target\n",
		"logo.png":       "image",
	})
	if r, err := ResolveAtLine(v, "A.md", raw, 2); err != nil || r.Path != "long/Target.md" {
		t.Fatalf("resolve=%+v err=%v", r, err)
	}
	if _, err := Move(v, MoveOptions{From: "long/Target.md", To: "New.md"}); err != nil {
		t.Fatal(err)
	}
	wantRaw := "[![logo](logo.png)](/New.md)"
	if got, want := readVaultFile(t, v, "A.md"), "# A\n"+wantRaw+"\n"; got != want {
		t.Fatalf("body=%q want=%q", got, want)
	}
	if r, err := ResolveAtLine(v, "A.md", wantRaw, 2); err != nil || r.Path != "New.md" {
		t.Fatalf("moved resolve=%+v err=%v", r, err)
	}
}
