package core

import (
	"os"
	"path/filepath"
	"testing"
)

func setupExcludeVault(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_query_exclude")
	buildForQuery(t, vault)
	return vault
}

func TestQueryHideDirectTargets(t *testing.T) {
	vault := setupExcludeVault(t)
	filter, err := NewQueryFilter(Config{}, QueryFilterOptions{Hide: ExcludeConfig{Paths: []string{"daily/*"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, relation := range []string{FieldQueryBacklinks, FieldQueryOutgoing} {
		result, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{Relations: []string{relation}, Filter: filter})
		if err != nil {
			t.Fatal(err)
		}
		nodes := result.Backlinks
		if relation == FieldQueryOutgoing {
			nodes = result.Outgoing
		}
		for _, node := range nodes {
			if node.Path == "daily/D.md" {
				t.Errorf("%s retained hidden target", relation)
			}
		}
	}
}

func TestQueryHideDoesNotHideEntry(t *testing.T) {
	vault := setupExcludeVault(t)
	filter, err := NewQueryFilter(Config{}, QueryFilterOptions{Hide: ExcludeConfig{Paths: []string{"daily/*"}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Query(vault, EntrySpec{File: "daily/D.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, Filter: filter})
	if err != nil {
		t.Fatal(err)
	}
	if result.Entry.Path != "daily/D.md" {
		t.Fatalf("entry = %+v", result.Entry)
	}
}

func TestQueryHideTwoHopTargetAndRelation(t *testing.T) {
	vault := setupExcludeVault(t)
	base, err := Query(vault, EntrySpec{File: "B.md"}, QueryOptions{Relations: []string{FieldQueryTwoHop}})
	if err != nil {
		t.Fatal(err)
	}
	sawDaily := false
	for _, target := range base.TwoHop {
		for _, via := range target.Relation {
			if via.Name == "#daily" {
				sawDaily = true
			}
		}
	}
	if !sawDaily {
		t.Fatal("fixture has no #daily shared relation")
	}
	filter, err := NewQueryFilter(Config{}, QueryFilterOptions{Hide: ExcludeConfig{Tags: []string{"daily"}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Query(vault, EntrySpec{File: "B.md"}, QueryOptions{Relations: []string{FieldQueryTwoHop}, Filter: filter})
	if err != nil {
		t.Fatal(err)
	}
	hiddenSeen := false
	for _, target := range result.TwoHop {
		if target.HiddenRelation {
			hiddenSeen = true
		}
		for _, via := range target.Relation {
			if via.Name == "#daily" {
				t.Fatal("hidden relation exposed")
			}
		}
	}
	if !hiddenSeen {
		t.Fatal("hidden relation marker absent")
	}
}

func TestQueryViaFilterExcludesConnector(t *testing.T) {
	vault := setupExcludeVault(t)
	filter, err := NewQueryFilter(Config{}, QueryFilterOptions{ViaExclude: ExcludeConfig{Tags: []string{"daily"}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Query(vault, EntrySpec{File: "B.md"}, QueryOptions{Relations: []string{FieldQueryTwoHop}, Filter: filter})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range result.TwoHop {
		for _, via := range target.Relation {
			if via.Name == "#daily" {
				t.Fatal("excluded connector exposed")
			}
		}
	}
}

func TestQueryFilterConfigIsExplicit(t *testing.T) {
	vault := setupExcludeVault(t)
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), []byte("query:\n  hide:\n    paths: ['daily/*']\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unfiltered, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}})
	if err != nil {
		t.Fatal(err)
	}
	foundDaily := false
	for _, node := range unfiltered.Backlinks {
		if node.Path == "daily/D.md" {
			foundDaily = true
		}
	}
	if !foundDaily {
		t.Fatal("core unexpectedly loaded query config")
	}
	cfg, err := LoadConfig(vault)
	if err != nil {
		t.Fatal(err)
	}
	filter, err := NewQueryFilter(cfg, QueryFilterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{Relations: []string{FieldQueryBacklinks}, Filter: filter})
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range filtered.Backlinks {
		if node.Path == "daily/D.md" {
			t.Fatal("explicit filter ignored")
		}
	}
}
