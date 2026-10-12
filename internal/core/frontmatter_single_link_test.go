package core

import (
	"strings"
	"testing"
)

func TestFrontmatterSingleLinkValues(t *testing.T) {
	for _, value := range []string{"[[A]]", "[[A#Heading|alias]]", "[[#Heading]]", "[[A]] [[B]]", "prose [[A]]", "[[A]] prose", " [[A]]", "[[A]] ", "![[A]]"} {
		t.Run(value, func(t *testing.T) {
			want := value == "[[A]]" || value == "[[A#Heading|alias]]" || value == "[[#Heading]]"
			content := "---\nref: \"" + value + "\"\n---\n"
			parsed := parseLinks(content)
			links := filterByType(parsed.Links, LinkTypeFrontmatterWikilink)
			if (len(links) == 1) != want {
				t.Fatalf("links = %+v, want single=%v", links, want)
			}
			if len(parsed.Meta) != 1 || parsed.Meta[0].Value != value {
				t.Fatalf("meta = %+v", parsed.Meta)
			}
		})
	}
	if links := filterByType(parseLinks("[[A]] [[B]]").Links, LinkTypeWikilink); len(links) != 2 {
		t.Fatalf("body links = %+v", links)
	}
}

func TestMetaCheckSingleLinkValues(t *testing.T) {
	vault := t.TempDir()
	buildTestWrite(t, vault, "Existing.md", "# Existing\n")
	buildTestWrite(t, vault, "A.md", "---\nrelated: [\"[[Existing]]\", \"[[Missing]]\", \"[[Existing]] [[Ignored]]\", \"prose [[Ignored]]\", \"[[Ignored]] prose\", \" [[Ignored]]\"]\n---\n")
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []MetaValueKind{MetaKindAuto, MetaKindWikilink} {
		result, err := MetaCheck(vault, MetaCheckOptions{Keys: []string{"related"}, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if kind == MetaKindWikilink {
			want = 5
		}
		if len(result.Issues) != want {
			t.Fatalf("%s issues=%+v", kind, result.Issues)
		}
		for _, issue := range result.Issues {
			reason := ReasonNotWikilink
			if issue.Value == "[[Missing]]" {
				reason = ReasonNotFound
			}
			if issue.Reason != reason {
				t.Fatalf("issue=%+v", issue)
			}
		}
	}
	db := openTestDB(t, dbPath(vault))
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM nodes WHERE name='Ignored'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("Ignored nodes=%d, %v", count, err)
	}
}

func TestRewriteFrontmatterPreservesOrdinaryStrings(t *testing.T) {
	content := "---\nsafe: \"[[B]]\"\nmultiple: \"[[B]] [[Missing]]\"\nprose: \"text [[B]]\"\nencoded: \"\\u005b\\u005bB\\u005d\\u005d prose\"\n---\n"
	got, err := rewriteFrontmatterCandidate([]byte(content), []rewriteEntry{plannedFrontmatterRewrite("[[B]]", "[[C]]", 2)})
	if err != nil || string(got) != strings.Replace(content, "[[B]]", "[[C]]", 1) {
		t.Fatalf("candidate=%q err=%v", got, err)
	}
	for _, line := range []int{3, 4, 5} {
		got, err := rewriteFrontmatterCandidate([]byte(content), []rewriteEntry{plannedFrontmatterRewrite("[[B]]", "[[C]]", line)})
		if err == nil || got != nil {
			t.Fatalf("line %d candidate=%q err=%v", line, got, err)
		}
	}
}

func TestFrontmatterOrdinaryStringsDoNotBlockMutation(t *testing.T) {
	vault := newMoveVault(t, map[string]string{"A.md": "---\nsafe: \"[[B]]\"\ntext: \"[[Ambig]] [[../../outside]]\"\n---\n", "B.md": "# B\n", "one/Ambig.md": "# Ambig\n", "two/Ambig.md": "# Ambig\n"})
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Move(vault, MoveOptions{From: "B.md", To: "C.md"}); err != nil {
		t.Fatal(err)
	}
	if got := readVaultFile(t, vault, "A.md"); got != "---\nsafe: \"[[C]]\"\ntext: \"[[Ambig]] [[../../outside]]\"\n---\n" {
		t.Fatalf("source=%q", got)
	}
	edges := queryEdges(t, dbPath(vault), "A.md")
	if len(edges) != 1 || edges[0].rawLink != "[[C]]" {
		t.Fatalf("edges=%+v", edges)
	}
}

func TestBuildReplacesVersionSixFrontmatterEdges(t *testing.T) {
	vault := t.TempDir()
	buildTestWrite(t, vault, "A.md", "---\nref: \"[[Ghost]] prose\"\n---\n")
	if _, err := Build(vault); err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, dbPath(vault))
	for _, sql := range []string{
		"INSERT INTO nodes(node_key,type,name,exists_flag) VALUES('phantom:Ghost','phantom','Ghost',0)",
		"INSERT INTO edges(source_id,target_id,link_type,raw_link,frontmatter_key,line_start,line_end) SELECT a.id,g.id,'frontmatter_wikilink','[[Ghost]]','ref',2,2 FROM nodes a,nodes g WHERE a.path='A.md' AND g.name='Ghost'",
		"PRAGMA user_version=6",
	} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	if _, err := MetaCheck(vault, MetaCheckOptions{Keys: []string{"ref"}, Kind: MetaKindAuto}); err == nil {
		t.Fatal("legacy read accepted")
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err == nil {
		t.Fatal("legacy partial update accepted")
	}
	assertBuildMatchesFull(t, vault)
	db = openTestDB(t, dbPath(vault))
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM nodes WHERE name='Ghost'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("Ghost nodes=%d, %v", count, err)
	}
	if edges := queryEdges(t, dbPath(vault), "A.md"); len(edges) != 0 {
		t.Fatalf("edges=%+v", edges)
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"A.md"}}); err != nil {
		t.Fatal(err)
	}
}
