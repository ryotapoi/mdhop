package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// buildReachableVault copies vault_reachable into a temp dir and builds it.
func buildReachableVault(t *testing.T) string {
	t.Helper()
	vault := copyVault(t, "vault_reachable")
	if _, err := Build(vault); err != nil {
		t.Fatalf("build: %v", err)
	}
	return vault
}

func TestReachableBasic(t *testing.T) {
	vault := buildReachableVault(t)
	res, err := Reachable(vault, ReachableOptions{From: "docs/index.md", Path: []string{"docs/*"}})
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	if res.From != "docs/index.md" {
		t.Errorf("From = %q, want docs/index.md", res.From)
	}
	// index: 0 hop. sub/md: wikilink and markdown link. leaf: 2 hops via
	// sub (false-positive check). fp: frontmatter_path. fw: frontmatter_wikilink.
	wantReachable := []string{"docs/fp.md", "docs/fw.md", "docs/index.md", "docs/leaf.md", "docs/md.md", "docs/sub.md"}
	if !reflect.DeepEqual(res.Reachable, wantReachable) {
		t.Errorf("Reachable = %v, want %v", res.Reachable, wantReachable)
	}
	// orphan shares tag #shared with index but tag edges are not traversed.
	wantUnreachable := []string{"docs/orphan.md"}
	if !reflect.DeepEqual(res.Unreachable, wantUnreachable) {
		t.Errorf("Unreachable = %v, want %v", res.Unreachable, wantUnreachable)
	}
	if res.Routes != nil {
		t.Errorf("Routes = %v, want nil without Route option", res.Routes)
	}
}

func TestReachableNoPathFilter(t *testing.T) {
	vault := buildReachableVault(t)
	res, err := Reachable(vault, ReachableOptions{From: "docs/index.md"})
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	// other/out.md links TO index, not from it → unreachable.
	wantUnreachable := []string{"docs/orphan.md", "other/out.md"}
	if !reflect.DeepEqual(res.Unreachable, wantUnreachable) {
		t.Errorf("Unreachable = %v, want %v", res.Unreachable, wantUnreachable)
	}
}

func TestReachableFromOutsidePathFilter(t *testing.T) {
	vault := buildReachableVault(t)
	// out.md is outside docs/* but still works as the traversal entry; it
	// appears in neither list.
	res, err := Reachable(vault, ReachableOptions{From: "other/out.md", Path: []string{"docs/*"}})
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	wantReachable := []string{"docs/fp.md", "docs/fw.md", "docs/index.md", "docs/leaf.md", "docs/md.md", "docs/sub.md"}
	if !reflect.DeepEqual(res.Reachable, wantReachable) {
		t.Errorf("Reachable = %v, want %v", res.Reachable, wantReachable)
	}
	for _, p := range append(res.Reachable, res.Unreachable...) {
		if p == "other/out.md" {
			t.Errorf("entry outside the path filter must not be listed: %v %v", res.Reachable, res.Unreachable)
		}
	}
}

func TestReachableExclude(t *testing.T) {
	vault := buildReachableVault(t)
	res, err := Reachable(vault, ReachableOptions{From: "docs/index.md", Path: []string{"docs/*"}, Exclude: []string{"docs/orphan.md"}})
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	if len(res.Unreachable) != 0 {
		t.Errorf("Unreachable = %v, want empty after exclude", res.Unreachable)
	}
}

func TestReachableRoute(t *testing.T) {
	vault := buildReachableVault(t)
	res, err := Reachable(vault, ReachableOptions{From: "docs/index.md", Path: []string{"docs/*"}, Route: true})
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	wantLeafRoute := []string{"docs/index.md", "docs/sub.md", "docs/leaf.md"}
	if !reflect.DeepEqual(res.Routes["docs/leaf.md"], wantLeafRoute) {
		t.Errorf("Routes[docs/leaf.md] = %v, want %v", res.Routes["docs/leaf.md"], wantLeafRoute)
	}
	wantSelfRoute := []string{"docs/index.md"}
	if !reflect.DeepEqual(res.Routes["docs/index.md"], wantSelfRoute) {
		t.Errorf("Routes[docs/index.md] = %v, want %v", res.Routes["docs/index.md"], wantSelfRoute)
	}
	if _, ok := res.Routes["docs/orphan.md"]; ok {
		t.Errorf("unreachable note must not have a route")
	}
}

func TestReachableRouteConnectorOutsidePathFilter(t *testing.T) {
	vault := buildReachableVault(t)
	// sub.md is excluded from the target set but still appears as a
	// connector in the route to leaf.md.
	res, err := Reachable(vault, ReachableOptions{From: "docs/index.md", Path: []string{"docs/*"}, Exclude: []string{"docs/sub.md"}, Route: true})
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	wantLeafRoute := []string{"docs/index.md", "docs/sub.md", "docs/leaf.md"}
	if !reflect.DeepEqual(res.Routes["docs/leaf.md"], wantLeafRoute) {
		t.Errorf("Routes[docs/leaf.md] = %v, want %v", res.Routes["docs/leaf.md"], wantLeafRoute)
	}
}

func TestReachableFromMissing(t *testing.T) {
	vault := buildReachableVault(t)
	_, err := Reachable(vault, ReachableOptions{})
	if err == nil || !strings.Contains(err.Error(), "--from") {
		t.Errorf("error = %v, want missing --from error", err)
	}
}

func TestReachableFromNotRegistered(t *testing.T) {
	vault := buildReachableVault(t)
	_, err := Reachable(vault, ReachableOptions{From: "docs/nope.md"})
	if !errors.Is(err, ErrFileNotRegistered) {
		t.Errorf("error = %v, want ErrFileNotRegistered", err)
	}
}

func TestReachableFromAsset(t *testing.T) {
	vault := buildReachableVault(t)
	_, err := Reachable(vault, ReachableOptions{From: "img/logo.png"})
	if !errors.Is(err, ErrFileNotRegistered) {
		t.Errorf("error = %v, want ErrFileNotRegistered for asset entry", err)
	}
}

func TestReachableInvalidGlob(t *testing.T) {
	vault := buildReachableVault(t)
	_, err := Reachable(vault, ReachableOptions{From: "docs/index.md", Path: []string{"docs/[ab]*"}})
	if err == nil {
		t.Errorf("error = nil, want invalid glob error")
	}
}
