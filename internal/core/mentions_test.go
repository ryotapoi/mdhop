package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mentionsVault(t *testing.T, files map[string]string) string {
	t.Helper()
	vault := t.TempDir()
	for path, content := range files {
		full := filepath.Join(vault, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	buildForQuery(t, vault)
	return vault
}

// mentionSummary renders "identifier@L:C-L:C=matched" for compact assertions.
func mentionSummary(mentions []Mention) []string {
	out := []string{}
	for _, m := range mentions {
		id := m.Path
		if m.Type == NodeTypePhantom {
			id = "phantom:" + m.Name
		}
		l := m.Location
		out = append(out, fmt.Sprintf("%s@%d:%d-%d:%d=%s", id, l.Start.Line, l.Start.Column, l.End.Line, l.End.Column, m.MatchedText))
	}
	return out
}

func assertMentions(t *testing.T, label string, got []Mention, want ...string) {
	t.Helper()
	summary := mentionSummary(got)
	if strings.Join(summary, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%s:\n got %q\nwant %q", label, summary, want)
	}
}

func TestMentionsIncomingExclusions(t *testing.T) {
	vault := mentionsVault(t, map[string]string{
		"E.md":        "[[Target]]\n",
		"Back.md":     "[[E]] E\n",
		"Target.md":   "E here\n",
		"Masked.md":   "---\na: E\n---\nsee `E` [[x]] [E](y) https://e.com/E mailto:E@x E!\n",
		"Fence.md":    "```\nE\n```\n",
		"Ref.md":      "[r]: E\n",
		"Word.md":     "Elephant E1\n",
		"Hidden/H.md": "E\n",
	})
	filter, err := NewQueryFilter(Config{Query: QueryConfig{Hide: ExcludeConfig{Paths: []string{"Hidden/*"}}}}, QueryFilterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Mentions(vault, MentionsEntry{File: "E.md"}, MentionsOptions{Filter: filter})
	if err != nil {
		t.Fatal(err)
	}
	// Back links to E and E links to Target, so neither direction returns them.
	assertMentions(t, "incoming", r.Incoming, "Masked.md@4:49-4:50=E", "Word.md@1:10-1:11=E")
	if *r.Total.Incoming != 2 {
		t.Fatalf("total incoming=%d", *r.Total.Incoming)
	}
	if r.Incoming[0].Excerpt != "see `E` [[x]] [E](y) https://e.com/E mailto:E@x E!" {
		t.Fatalf("excerpt=%q", r.Incoming[0].Excerpt)
	}
}

func TestMentionsOutgoingBoundariesAndSharedNames(t *testing.T) {
	vault := mentionsVault(t, map[string]string{
		"Plan.md":      "---\ntags: [Alpha]\n---\nAlpha Plan Back github git スキルアップ 仕様書 iOS26 Linked GHOST\n[[Linked]]\n",
		"Alpha.md":     "a\n",
		"sub/alpha.md": "b\n",
		"git.md":       "g\n",
		"スキル.md":       "s\n",
		"仕様.md":        "p\n",
		"ios.md":       "i\n",
		"Linked.md":    "l\n",
		"Back.md":      "[[Plan]]\n",
		"Other.md":     "[[Ghost]]\n",
	})
	r, err := Mentions(vault, MentionsEntry{File: "Plan.md"}, MentionsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Alpha.md@4:1-4:6=Alpha",
		"git.md@4:24-4:27=git",
		"ios.md@4:39-4:42=iOS",
		"sub/alpha.md@4:1-4:6=Alpha",
		"スキル.md@4:28-4:31=スキル",
		"仕様.md@4:35-4:37=仕様",
		"phantom:Ghost@4:52-4:57=GHOST",
	}
	assertMentions(t, "outgoing", r.Outgoing, want...)
	if r.Total.Outgoing == nil || *r.Total.Outgoing != len(want) {
		t.Fatalf("total outgoing=%v", r.Total.Outgoing)
	}

	r, err = Mentions(vault, MentionsEntry{File: "Plan.md"}, MentionsOptions{Partial: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := mentionSummary(r.Outgoing)[1]; got != "git.md@4:17-4:20=git" {
		t.Fatalf("partial git=%s", got)
	}

	limit := 2
	r, err = Mentions(vault, MentionsEntry{File: "Plan.md"}, MentionsOptions{Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	assertMentions(t, "limited", r.Outgoing, want[:2]...)
	if *r.Total.Outgoing != len(want) {
		t.Fatalf("limited total=%d", *r.Total.Outgoing)
	}
}

func TestMentionsPhantomEntryIncomingOnly(t *testing.T) {
	vault := mentionsVault(t, map[string]string{
		"Other.md": "[[Ghost]] ghost\n",
		"Note.md":  "a ghost story\n",
	})
	r, err := Mentions(vault, MentionsEntry{Phantom: "ghost"}, MentionsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertMentions(t, "incoming", r.Incoming, "Note.md@1:3-1:8=ghost")
	if r.Outgoing != nil || r.Total.Outgoing != nil {
		t.Fatalf("phantom outgoing=%v total=%v", r.Outgoing, r.Total.Outgoing)
	}
}

func TestMentionsNFDSourceAndCRLFLocation(t *testing.T) {
	vault := mentionsVault(t, map[string]string{
		"Café.md":   "c\n",
		"N.md":      "---\r\nx: 1\r\n---\r\nあ Cafe\u0301s CAFE\u0301\r\n",
		"Plural.md": "Café's Cafés\n",
	})
	r, err := Mentions(vault, MentionsEntry{File: "Café.md"}, MentionsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Cafés is a longer Latin word; columns count the NFD code points.
	assertMentions(t, "incoming", r.Incoming, "N.md@4:10-4:15=CAFE\u0301", "Plural.md@1:1-1:5=Café")
	if r.Incoming[0].Excerpt != "あ Cafe\u0301s CAFE\u0301" {
		t.Fatalf("excerpt=%q", r.Incoming[0].Excerpt)
	}
}

func TestMentionsErrors(t *testing.T) {
	vault := mentionsVault(t, map[string]string{
		"A.md":     "[[Ghost]] ![[img.png]]\n",
		"img.png":  "x",
		"Other.md": "o\n",
	})
	zero := 0
	cases := []struct {
		name  string
		entry MentionsEntry
		opts  MentionsOptions
		want  string
	}{
		{"none", MentionsEntry{}, MentionsOptions{}, "no entry specified"},
		{"both", MentionsEntry{File: "A.md", Phantom: "Ghost"}, MentionsOptions{}, "exactly one"},
		{"limit", MentionsEntry{File: "A.md"}, MentionsOptions{Limit: &zero}, "limit must be positive"},
		{"asset", MentionsEntry{File: "img.png"}, MentionsOptions{}, "requires an indexed note"},
		{"phantom", MentionsEntry{Phantom: "Missing"}, MentionsOptions{}, "phantom not in index"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Mentions(vault, tc.entry, tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want %q", err, tc.want)
			}
		})
	}
	if _, err := Mentions(vault, MentionsEntry{File: "Missing.md"}, MentionsOptions{}); !errors.Is(err, ErrFileNotRegistered) {
		t.Fatalf("unregistered err=%v", err)
	}
}

func TestMentionsSkipsDeletedNotes(t *testing.T) {
	vault := mentionsVault(t, map[string]string{
		"E.md":    "e\n",
		"Gone.md": "E\n",
		"Kept.md": "E\n",
	})
	if err := os.Remove(filepath.Join(vault, "Gone.md")); err != nil {
		t.Fatal(err)
	}
	r, err := Mentions(vault, MentionsEntry{File: "E.md"}, MentionsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertMentions(t, "incoming", r.Incoming, "Kept.md@1:1-1:2=E")
}

func TestMentionsLengthChangingCaseFolding(t *testing.T) {
	// İ and the Kelvin sign change byte length when lower-cased.
	vault := mentionsVault(t, map[string]string{
		"E.md": "e\n",
		"T.md": "İİ \u212a E mailto:E@x\n",
	})
	r, err := Mentions(vault, MentionsEntry{File: "E.md"}, MentionsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertMentions(t, "incoming", r.Incoming, "T.md@1:6-1:7=E")
}

func TestMentionsFilters(t *testing.T) {
	vault := mentionsVault(t, map[string]string{
		"E.md":           "Ghost Keep Drop Day\n",
		"notes/Keep.md":  "E\n",
		"notes/Drop.md":  "E\n",
		"daily/Day.md":   "E\n",
		"archive/Old.md": "E\n",
		"Other.md":       "[[Ghost]]\n",
	})
	newFilter := func(hide ...string) *QueryFilter {
		t.Helper()
		f, err := NewQueryFilter(Config{Query: QueryConfig{Hide: ExcludeConfig{Paths: []string{"archive/*"}}}},
			QueryFilterOptions{Hide: ExcludeConfig{Paths: hide}})
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	run := func(entry MentionsEntry, opts MentionsOptions) *MentionsResult {
		t.Helper()
		r, err := Mentions(vault, entry, opts)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	keep := "notes/Keep.md@1:1-1:2=E"
	drop := "notes/Drop.md@1:1-1:2=E"
	day := "daily/Day.md@1:1-1:2=E"
	ghost := "phantom:Ghost@1:1-1:6=Ghost"
	outKeep := "notes/Keep.md@1:7-1:11=Keep"
	outDrop := "notes/Drop.md@1:12-1:16=Drop"
	outDay := "daily/Day.md@1:17-1:20=Day"
	file := MentionsEntry{File: "E.md"}

	// --path narrows notes in both directions and keeps pathless phantoms.
	r := run(file, MentionsOptions{Filter: newFilter(), Path: []string{"notes/*"}})
	assertMentions(t, "path incoming", r.Incoming, drop, keep)
	assertMentions(t, "path outgoing", r.Outgoing, outDrop, outKeep, ghost)

	// Caller hide adds to [query.hide] and wins over a matching --path.
	r = run(file, MentionsOptions{Filter: newFilter("notes/Drop.md"), Path: []string{"notes/*", "archive/*"}})
	assertMentions(t, "hide incoming", r.Incoming, keep)
	assertMentions(t, "hide outgoing", r.Outgoing, outKeep, ghost)

	// --no-phantom drops phantoms from outgoing only.
	r = run(file, MentionsOptions{Filter: newFilter(), NoPhantom: true})
	assertMentions(t, "no-phantom incoming", r.Incoming, day, drop, keep)
	assertMentions(t, "no-phantom outgoing", r.Outgoing, outDay, outDrop, outKeep)
	r = run(MentionsEntry{Phantom: "Ghost"}, MentionsOptions{Filter: newFilter(), NoPhantom: true})
	assertMentions(t, "phantom entry", r.Incoming, "E.md@1:1-1:6=Ghost")

	// Totals count filtered mentions before the limit.
	limit := 1
	r = run(file, MentionsOptions{Filter: newFilter(), Path: []string{"notes/*"}, Limit: &limit})
	assertMentions(t, "limited incoming", r.Incoming, drop)
	assertMentions(t, "limited outgoing", r.Outgoing, outDrop)
	if *r.Total.Incoming != 2 || *r.Total.Outgoing != 3 {
		t.Fatalf("total=%d/%d", *r.Total.Incoming, *r.Total.Outgoing)
	}

	if _, err := Mentions(vault, file, MentionsOptions{Path: []string{"a[b]"}}); err == nil || err.Error() != "unsupported glob pattern (character class): a[b]" {
		t.Fatalf("glob err=%v", err)
	}
}

func TestMentionsWhere(t *testing.T) {
	vault := mentionsVault(t, map[string]string{
		"E.md":          "Keep Drop Bare Day Ghost\n",
		"notes/Keep.md": "---\nstatus: active\n---\nE\n",
		"notes/Drop.md": "---\nstatus: stale\n---\nE\n",
		"daily/Day.md":  "---\nstatus: active\n---\nE\n",
		"Bare.md":       "E\n",
		"Other.md":      "[[Ghost]]\n",
	})
	run := func(opts MentionsOptions, exprs ...string) *MentionsResult {
		t.Helper()
		wc, err := ParseWhere(exprs, MetaConfig{})
		if err != nil {
			t.Fatal(err)
		}
		opts.Where = wc
		r, err := Mentions(vault, MentionsEntry{File: "E.md"}, opts)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	keep := "notes/Keep.md@4:1-4:2=E"
	day := "daily/Day.md@4:1-4:2=E"
	outKeep := "notes/Keep.md@1:1-1:5=Keep"
	outDay := "daily/Day.md@1:16-1:19=Day"

	// Without --where, every candidate is returned.
	r := run(MentionsOptions{})
	assertMentions(t, "all incoming", r.Incoming, "Bare.md@1:1-1:2=E", day, "notes/Drop.md@4:1-4:2=E", keep)
	assertMentions(t, "all outgoing", r.Outgoing, "Bare.md@1:11-1:15=Bare", outDay, "notes/Drop.md@1:6-1:10=Drop", outKeep, "phantom:Ghost@1:20-1:25=Ghost")

	// Both directions keep matching notes only; notes without the key and
	// phantoms fail key!=value, as in query.
	r = run(MentionsOptions{}, "status != stale")
	assertMentions(t, "neq incoming", r.Incoming, day, keep)
	assertMentions(t, "neq outgoing", r.Outgoing, outDay, outKeep)

	// key NOT EXISTS keeps notes without the key but still drops phantoms.
	r = run(MentionsOptions{}, "status NOT EXISTS")
	assertMentions(t, "not-exists incoming", r.Incoming, "Bare.md@1:1-1:2=E")
	assertMentions(t, "not-exists outgoing", r.Outgoing, "Bare.md@1:11-1:15=Bare")

	// --where combines with --path, --no-phantom, and --limit by AND; totals
	// count filtered mentions before the limit.
	limit := 1
	r = run(MentionsOptions{Path: []string{"notes/*", "daily/*"}, NoPhantom: true, Limit: &limit}, "status = active")
	assertMentions(t, "limited incoming", r.Incoming, day)
	assertMentions(t, "limited outgoing", r.Outgoing, outDay)
	if *r.Total.Incoming != 2 || *r.Total.Outgoing != 2 {
		t.Fatalf("total=%d/%d", *r.Total.Incoming, *r.Total.Outgoing)
	}

	// --hide-path also applies on top of --where.
	filter, err := NewQueryFilter(Config{}, QueryFilterOptions{Hide: ExcludeConfig{Paths: []string{"daily/*"}}})
	if err != nil {
		t.Fatal(err)
	}
	r = run(MentionsOptions{Filter: filter}, "status = active")
	assertMentions(t, "hide incoming", r.Incoming, keep)
	assertMentions(t, "hide outgoing", r.Outgoing, outKeep)
	if *r.Total.Incoming != 1 || *r.Total.Outgoing != 1 {
		t.Fatalf("hide total=%d/%d", *r.Total.Incoming, *r.Total.Outgoing)
	}
}

func TestMentionsRelations(t *testing.T) {
	vault := mentionsVault(t, map[string]string{
		"E.md":     "A B Ghost\n",
		"A.md":     "E\n",
		"B.md":     "E\n",
		"Other.md": "[[Ghost]] E\n",
	})
	run := func(entry MentionsEntry, opts MentionsOptions) *MentionsResult {
		t.Helper()
		r, err := Mentions(vault, entry, opts)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	file := MentionsEntry{File: "E.md"}
	in := []string{"A.md@1:1-1:2=E", "B.md@1:1-1:2=E", "Other.md@1:11-1:12=E"}
	out := []string{"A.md@1:1-1:2=A", "B.md@1:3-1:4=B", "phantom:Ghost@1:5-1:10=Ghost"}

	r := run(file, MentionsOptions{Relations: []string{"incoming"}})
	assertMentions(t, "incoming only", r.Incoming, in...)
	if *r.Total.Incoming != 3 || r.Outgoing != nil || r.Total.Outgoing != nil {
		t.Fatalf("incoming only: outgoing=%v total=%v", r.Outgoing, r.Total.Outgoing)
	}

	r = run(file, MentionsOptions{Relations: []string{"outgoing"}})
	assertMentions(t, "outgoing only", r.Outgoing, out...)
	if *r.Total.Outgoing != 3 || r.Incoming != nil || r.Total.Incoming != nil {
		t.Fatalf("outgoing only: incoming=%v total=%v", r.Incoming, r.Total.Incoming)
	}

	r = run(file, MentionsOptions{Relations: []string{"outgoing", "incoming"}})
	assertMentions(t, "both incoming", r.Incoming, in...)
	assertMentions(t, "both outgoing", r.Outgoing, out...)

	// --limit applies to each selected direction; total counts before it.
	limit := 1
	r = run(file, MentionsOptions{Relations: []string{"outgoing"}, Limit: &limit})
	assertMentions(t, "limited outgoing", r.Outgoing, out[0])
	if *r.Total.Outgoing != 3 || r.Total.Incoming != nil {
		t.Fatalf("limited total=%v/%d", r.Total.Incoming, *r.Total.Outgoing)
	}
	r = run(file, MentionsOptions{Relations: []string{"incoming", "outgoing"}, Limit: &limit})
	assertMentions(t, "limited both incoming", r.Incoming, in[0])
	assertMentions(t, "limited both outgoing", r.Outgoing, out[0])

	// A phantom entry has no body, so only incoming can be selected.
	phantom := MentionsEntry{Phantom: "Ghost"}
	r = run(phantom, MentionsOptions{Relations: []string{"incoming"}})
	assertMentions(t, "phantom incoming", r.Incoming, "E.md@1:5-1:10=Ghost")
	if r.Outgoing != nil || r.Total.Outgoing != nil {
		t.Fatalf("phantom outgoing=%v", r.Outgoing)
	}

	for _, tc := range []struct {
		entry     MentionsEntry
		relations []string
		want      string
	}{
		{file, []string{}, "relations: select at least one relation"},
		{file, []string{"backlinks"}, `relations: unknown relation "backlinks"`},
		{file, []string{"incoming", "incoming"}, `relations: duplicate relation "incoming"`},
		{phantom, []string{"outgoing"}, "relations: outgoing requires a --file entry"},
		{phantom, []string{"incoming", "outgoing"}, "relations: outgoing requires a --file entry"},
	} {
		if _, err := Mentions(vault, tc.entry, MentionsOptions{Relations: tc.relations}); err == nil || err.Error() != tc.want {
			t.Fatalf("%v: err=%v, want %q", tc.relations, err, tc.want)
		}
	}
}

func TestMentionsSort(t *testing.T) {
	vault := mentionsVault(t, map[string]string{
		"E.md":     "A B C D Ghost\n",
		"A.md":     "---\ndate: 2026-01-02\n---\nE\n",
		"B.md":     "---\ndate: 2026-03-01\n---\nE [[C]]\n",
		"C.md":     "---\ndate: 2026-02-01\n---\nE\n",
		"D.md":     "E [[C]] [[A]]\n",
		"Other.md": "[[Ghost]] [[C]]\n",
	})
	run := func(opts MentionsOptions) *MentionsResult {
		t.Helper()
		r, err := Mentions(vault, MentionsEntry{File: "E.md"}, opts)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	inA, inB, inC, inD := "A.md@4:1-4:2=E", "B.md@4:1-4:2=E", "C.md@4:1-4:2=E", "D.md@1:1-1:2=E"
	outA, outB, outC, outD := "A.md@1:1-1:2=A", "B.md@1:3-1:4=B", "C.md@1:5-1:6=C", "D.md@1:7-1:8=D"
	ghost := "phantom:Ghost@1:9-1:14=Ghost"

	// An empty sort keeps the default order.
	r := run(MentionsOptions{Sort: ""})
	assertMentions(t, "default outgoing", r.Outgoing, outA, outB, outC, outD, ghost)

	// D has no date and Ghost is a phantom: both stay last in either direction.
	r = run(MentionsOptions{Sort: "date"})
	assertMentions(t, "date outgoing", r.Outgoing, outA, outC, outB, outD, ghost)
	r = run(MentionsOptions{Sort: "-date"})
	assertMentions(t, "-date incoming", r.Incoming, inB, inC, inA, inD)
	assertMentions(t, "-date outgoing", r.Outgoing, outB, outC, outA, outD, ghost)

	// Computed fields come from the index; a phantom has none, and equal
	// counts (B and D have 0) fall back to path order.
	r = run(MentionsOptions{Sort: "-incoming_count"})
	assertMentions(t, "-incoming_count outgoing", r.Outgoing, outC, outA, outB, outD, ghost)
	r = run(MentionsOptions{Sort: "incoming_count"})
	assertMentions(t, "incoming_count incoming", r.Incoming, inB, inD, inA, inC)

	// --limit keeps the top N after sorting; total counts before the limit.
	limit := 2
	r = run(MentionsOptions{Sort: "-date", Limit: &limit})
	assertMentions(t, "limited incoming", r.Incoming, inB, inC)
	assertMentions(t, "limited outgoing", r.Outgoing, outB, outC)
	if *r.Total.Incoming != 4 || *r.Total.Outgoing != 5 {
		t.Fatalf("total=%d/%d", *r.Total.Incoming, *r.Total.Outgoing)
	}

	// An empty key fails before the index is opened.
	if _, err := Mentions(t.TempDir(), MentionsEntry{File: "E.md"}, MentionsOptions{Sort: "-"}); err == nil || err.Error() != "empty sort key" {
		t.Fatalf("err=%v", err)
	}
}
