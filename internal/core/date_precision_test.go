package core

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestDatePrecisionQueries(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"mdhop.toml": "[meta.types]\ndate='date'\nbackup='date'\n"})
	values := []string{"2026-01-01T00:00:00", "2026-01-01T00:00:00.100", "2026-01-01T00:00:00.900", "2026-01-01T00:00:01"}
	for i, value := range values {
		writeInterpretationFiles(t, vault, map[string]string{fmt.Sprintf("%d.md", i): "---\ndate: '" + value + "'\n---\n"})
	}
	buildVault(t, vault)
	cfg := searchVaultConfig(t, vault)
	check := func(expr string, want []string) {
		t.Helper()
		wc, err := ParseWhere([]string{expr}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Search(vault, SearchOptions{Where: wc, Fields: []string{"meta"}})
		if err != nil || !slices.Equal(searchPaths(got.Items), want) {
			t.Fatalf("%s: got %+v err %v want %v", expr, got, err, want)
		}
		if expr == "date=2026-01-01T00:00:00.10" && got.Items[0].Meta[0].Value != values[1] {
			t.Fatal("raw value lost")
		}
	}
	for _, tc := range []struct {
		op   string
		want []string
	}{
		{"=", []string{"1.md"}}, {"!=", []string{"0.md", "2.md", "3.md"}}, {"<", []string{"0.md"}}, {"<=", []string{"0.md", "1.md"}}, {">", []string{"2.md", "3.md"}}, {">=", []string{"1.md", "2.md", "3.md"}},
	} {
		check("date"+tc.op+"2026-01-01T00:00:00.10", tc.want)
	}
	check("coalesce(date,backup)=2026-01-01T00:00:00.1000", []string{"1.md"})
	check("date=2026-01-01", nil)
	check("date=2026-01-01T00:00:00.100Z", nil)
	writeInterpretationFiles(t, vault, map[string]string{"multi.md": "---\ndate: ['2026-01-01T00:00:00.050', '2026-01-01T00:00:00.950']\n---\n", "backup.md": "---\nbackup: '2026-01-01T00:00:00.100'\n---\n"})
	buildVault(t, vault)
	check("coalesce(date,backup)=2026-01-01T00:00:00.1000", []string{"1.md", "backup.md"})
	for _, tc := range []struct {
		sort string
		want []string
	}{
		{"date", []string{"0.md", "multi.md", "1.md", "2.md", "3.md", "backup.md"}},
		{"-date", []string{"3.md", "multi.md", "2.md", "1.md", "0.md", "backup.md"}},
	} {
		got, err := Search(vault, SearchOptions{Sort: tc.sort})
		if err != nil || !slices.Equal(searchPaths(got.Items), tc.want) {
			t.Fatalf("sort %s: %+v %v", tc.sort, got, err)
		}
	}
}

func TestDatePrecisionMutationAndLegacyRebuild(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"mdhop.toml": "[meta.types]\ndate='date'\n", "a.md": "---\ndate: '2026-01-01T00:00:00.100'\n---\n"})
	buildVault(t, vault)
	cfg := searchVaultConfig(t, vault)
	assertDate := func(path, value string) {
		t.Helper()
		wc, err := ParseWhere([]string{"date=" + value}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Search(vault, SearchOptions{Where: wc, Fields: []string{"meta"}})
		if err != nil || !slices.Equal(searchPaths(got.Items), []string{path}) || got.Items[0].Meta[0].Value != value {
			t.Fatalf("date %s: %+v %v", value, got, err)
		}
	}
	assertDate("a.md", "2026-01-01T00:00:00.100")
	writeInterpretationFiles(t, vault, map[string]string{"b.md": "---\ndate: '2026-01-01T00:00:00.900Z'\n---\n"})
	if _, err := Add(vault, AddOptions{Files: []string{"b.md"}}); err != nil {
		t.Fatal(err)
	}
	assertDate("b.md", "2026-01-01T00:00:00.900Z")
	writeInterpretationFiles(t, vault, map[string]string{"b.md": "---\ndate: '2026-01-01T09:00:00.123456789+09:00'\n---\n"})
	if _, err := Update(vault, UpdateOptions{Files: []string{"b.md"}}); err != nil {
		t.Fatal(err)
	}
	assertDate("b.md", "2026-01-01T09:00:00.123456789+09:00")
	if _, err := Set(vault, SetOptions{File: "b.md", Key: "date", Value: "2026-01-01T00:00:00.000000001Z"}); err != nil {
		t.Fatal(err)
	}
	assertDate("b.md", "2026-01-01T00:00:00.000000001Z")
	buildVault(t, vault)
	before := indexSnapshot(t, vault)
	buildVault(t, vault)
	if !reflect.DeepEqual(before, indexSnapshot(t, vault)) {
		t.Fatal("incremental differs")
	}
	if _, err := BuildFull(vault); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, indexSnapshot(t, vault)) {
		t.Fatal("full differs")
	}
	db, err := openDBAt(dbPath(vault))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE meta SET sort_value='b:2026-01-01T00:00:00' WHERE key='date'"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version=11"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	source := readVaultFile(t, vault, "a.md")
	if _, err := Search(vault, SearchOptions{}); err == nil {
		t.Fatal("legacy search accepted")
	}
	if _, err := Update(vault, UpdateOptions{Files: []string{"a.md"}}); err == nil {
		t.Fatal("legacy update accepted")
	}
	if _, err := Add(vault, AddOptions{Files: []string{"a.md"}}); err == nil {
		t.Fatal("legacy add accepted")
	}
	if _, err := Set(vault, SetOptions{File: "a.md", Key: "date", Value: "2026-01-02"}); err == nil {
		t.Fatal("legacy set accepted")
	}
	if source != readVaultFile(t, vault, "a.md") {
		t.Fatal("failed mutation changed source")
	}
	buildVault(t, vault)
	assertDate("a.md", "2026-01-01T00:00:00.100")
	assertDate("b.md", "2026-01-01T00:00:00.000000001Z")
}
