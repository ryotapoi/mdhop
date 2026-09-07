package core

import (
	"bytes"
	"strings"
	"testing"
)

func plannedFrontmatterRewrite(old, new string, line int) rewriteEntry {
	return rewriteEntry{linkType: LinkTypeFrontmatterWikilink, rawLink: old, newRawLink: new, lineStart: line}
}

func TestRewriteFrontmatterCandidateRewritesQuotedScalarsOnly(t *testing.T) {
	content := "---\n関連: \"[[B#Heading|表示]]\" # [[B]] comment\nlist:\n  - '[[B]]'\nflow: [\"[[B]]\", \"[[B]]\"]\n\"関連[[B]]\": \"other\"\nbare: [[B]]\nblock: |\n  [[B]]\ntags: \"[[B]]\"\n---\n[[B]]\n"
	rewrites := []rewriteEntry{
		plannedFrontmatterRewrite("[[B#Heading|表示]]", "[[C#Heading|表示]]", 2),
		plannedFrontmatterRewrite("[[B]]", "[[C]]", 4),
		plannedFrontmatterRewrite("[[B]]", "[[C]]", 5),
		plannedFrontmatterRewrite("[[B]]", "[[C]]", 5),
	}
	got, err := rewriteFrontmatterCandidate([]byte(content), rewrites)
	if err != nil {
		t.Fatalf("rewriteFrontmatterCandidate: %v", err)
	}
	want := strings.ReplaceAll(content, "[[B#Heading|表示]]", "[[C#Heading|表示]]")
	want = strings.Replace(want, "  - '[[B]]'", "  - '[[C]]'", 1)
	want = strings.Replace(want, "flow: [\"[[B]]\", \"[[B]]\"]", "flow: [\"[[C]]\", \"[[C]]\"]", 1)
	if string(got) != want {
		t.Fatalf("candidate:\n%s\nwant:\n%s", got, want)
	}
	if len(rewrites) != 4 || rewrites[0].rawLink != "[[B#Heading|表示]]" {
		t.Fatal("candidate generation changed its inputs")
	}
}

func TestRewriteFrontmatterCandidateRejectsUnsupportedSourceDifferences(t *testing.T) {
	for _, content := range []string{
		"---\nref: \"\\u005b\\u005bB\\u005d\\u005d\"\n---\n",
		"---\nref: '[[B''s]]'\n---\n",
		"---\nref: \"[[B]]\n  more\"\n---\n",
	} {
		t.Run(strings.Split(content, "\n")[1], func(t *testing.T) {
			original := []byte(content)
			_, err := rewriteFrontmatterCandidate(original, []rewriteEntry{plannedFrontmatterRewrite("[[B]]", "[[C]]", 2)})
			if err == nil || !strings.Contains(err.Error(), "frontmatter") {
				t.Fatalf("error = %v, want correspondence rejection", err)
			}
			if string(original) != content {
				t.Fatal("input changed after rejected candidate")
			}
		})
	}
}

func TestRewriteFrontmatterCandidateRejectsLaterOccurrenceInUnsupportedScalar(t *testing.T) {
	content := "---\nref: \"\\u005b\\u005bA\\u005d\\u005d [[B]]\"\n---\n"
	original := []byte(content)
	got, err := rewriteFrontmatterCandidate(original, []rewriteEntry{plannedFrontmatterRewrite("[[B]]", "[[C]]", 2)})
	if err == nil || !strings.Contains(err.Error(), "correspondence") {
		t.Fatalf("error = %v, want correspondence rejection", err)
	}
	if got != nil {
		t.Fatalf("candidate = %q, want nil", got)
	}
	if string(original) != content {
		t.Fatal("input changed after rejected candidate")
	}
}

func TestRewriteFrontmatterCandidateIgnoresUnplannedLossyScalar(t *testing.T) {
	content := "---\nsafe: \"[[B]]\"\nencoded: \"\\u005b\\u005bIgnored\\u005d\\u005d\"\n---\n"
	got, err := rewriteFrontmatterCandidate([]byte(content), []rewriteEntry{plannedFrontmatterRewrite("[[B]]", "[[C]]", 2)})
	if err != nil {
		t.Fatalf("rewriteFrontmatterCandidate: %v", err)
	}
	if want := "---\nsafe: \"[[C]]\"\nencoded: \"\\u005b\\u005bIgnored\\u005d\\u005d\"\n---\n"; string(got) != want {
		t.Fatalf("candidate = %q, want %q", got, want)
	}
}

func TestRewriteFrontmatterCandidateRejectsInvalidPlansAndDoesNotChain(t *testing.T) {
	content := "---\nref: \"[[B]] [[C]]\"\n---\n"
	got, err := rewriteFrontmatterCandidate([]byte(content), []rewriteEntry{
		plannedFrontmatterRewrite("[[B]]", "[[C]]", 2),
		plannedFrontmatterRewrite("[[C]]", "[[D]]", 2),
	})
	if err != nil {
		t.Fatalf("rewriteFrontmatterCandidate: %v", err)
	}
	if want := "---\nref: \"[[C]] [[D]]\"\n---\n"; string(got) != want {
		t.Fatalf("candidate = %q, want %q", got, want)
	}
	_, err = rewriteFrontmatterCandidate([]byte(content), []rewriteEntry{plannedFrontmatterRewrite("[[B]]", "[[C]]", 2), plannedFrontmatterRewrite("[[B]]", "[[C]]", 2)})
	if err == nil || !strings.Contains(err.Error(), "no matching") {
		t.Fatalf("error = %v, want excess-plan rejection", err)
	}
}

func TestRewriteFrontmatterCandidateAppliesSameScalarReplacementsFromOriginalOffsets(t *testing.T) {
	content := "---\nref: \"[[B]] [[B]]\"\n---\n"
	got, err := rewriteFrontmatterCandidate([]byte(content), []rewriteEntry{
		plannedFrontmatterRewrite("[[B]]", "[[Longer]]", 2),
		plannedFrontmatterRewrite("[[B]]", "[[C]]", 2),
	})
	if err != nil {
		t.Fatalf("rewriteFrontmatterCandidate: %v", err)
	}
	if want := "---\nref: \"[[Longer]] [[C]]\"\n---\n"; string(got) != want {
		t.Fatalf("candidate = %q, want %q", got, want)
	}
}

func TestRewriteFrontmatterCandidateDoesNotInspectInvalidFrontmatterWithoutPlan(t *testing.T) {
	content := []byte("---\ninvalid: [\n---\n[[B]]\n")
	got, err := rewriteFrontmatterCandidate(content, nil)
	if err != nil {
		t.Fatalf("rewriteFrontmatterCandidate: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("candidate = %q, want unchanged", got)
	}
}

func TestRewriteFrontmatterCandidateDoesNotInspectInvalidFrontmatterForBodyOnlyPlan(t *testing.T) {
	content := []byte("---\ninvalid: [\n---\n[[B]]\n")
	plan := []rewriteEntry{{linkType: LinkTypeWikilink, rawLink: "[[B]]", newRawLink: "[[C]]", lineStart: 4}}
	got, err := rewriteFrontmatterCandidate(content, plan)
	if err != nil {
		t.Fatalf("rewriteFrontmatterCandidate: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("candidate = %q, want unchanged", got)
	}
	if plan[0].rawLink != "[[B]]" || plan[0].newRawLink != "[[C]]" {
		t.Fatal("candidate generation changed the body plan")
	}
}
