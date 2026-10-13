package core

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// semverOracle deliberately uses arbitrary precision integers rather than the
// production comparator's digit lengths and lexical comparisons.
func semverOracle(a, b string) int {
	x, y := strings.Split(strings.TrimLeft(a, "vV"), "."), strings.Split(strings.TrimLeft(b, "vV"), ".")
	for i := 0; i < max(len(x), len(y)); i++ {
		left, right := new(big.Int), new(big.Int)
		if i < len(x) {
			left.SetString(x[i], 10)
		}
		if i < len(y) {
			right.SetString(y[i], 10)
		}
		if cmp := left.Cmp(right); cmp != 0 {
			return cmp
		}
	}
	return 0
}

var semverBoundaryValues = []string{"0", "000.0.0000", "1", "1.0", "01.00.000", "V1.0.0.0", "0001", "1.0.0.0.5", "1.1", "100", "1.0.99999999", "1.0.100000000", "1.0.202610121457", "1.0.999999999999999999999999999999999999", "1.0.1000000000000000000000000000000000000"}

func TestSemverComparatorOracleAndTotalOrder(t *testing.T) {
	values := []string{"b:2", "b:10", "b:s:1", "b:invalid", "b:", "b:0.0"}
	for _, a := range semverBoundaryValues {
		normalized, warning := normalizeSemver(a)
		if warning != "" {
			t.Fatal(warning)
		}
		values = append(values, storedSortValue(normalized, "semver"))
		for _, b := range semverBoundaryValues {
			other, _ := normalizeSemver(b)
			if got, want := compareSemver(normalized, other), semverOracle(a, b); got != want {
				t.Fatalf("compare(%q,%q) = %d want %d", a, b, got, want)
			}
		}
	}
	for _, a := range values {
		for _, b := range values {
			if compareSortValues(a, b) != -compareSortValues(b, a) {
				t.Fatalf("asymmetry %q %q", a, b)
			}
			for _, c := range values {
				if compareSortValues(a, b) <= 0 && compareSortValues(b, c) <= 0 && compareSortValues(a, c) > 0 {
					t.Fatalf("nontransitive %q %q %q", a, b, c)
				}
			}
		}
	}
	for _, invalid := range []string{"-1", "1..2", ".1", "1.", "1.2-beta", "1+build", "v", "1.a"} {
		got, warning := normalizeSemver(invalid)
		if got != invalid || warning == "" {
			t.Fatalf("invalid %q = %q, %q", invalid, got, warning)
		}
	}
}

func TestSemverDiskIndexQueriesAndConnections(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"mdhop.toml": "[meta.types]\nversion = 'semver'\nbackup = 'semver'\nlabel = 'string'\namount = 'number'\nrank = { ordered = ['low', 'high'] }\ndate = 'date'\n"})
	for i, value := range semverBoundaryValues {
		literal := fmt.Sprintf("%q", value)
		if value == "0" || value == "1" || value == "100" {
			literal = value
		}
		writeInterpretationFiles(t, vault, map[string]string{fmt.Sprintf("%02d.md", i): "---\nversion: " + literal + "\n---\n"})
	}
	writeInterpretationFiles(t, vault, map[string]string{
		"missing.md": "", "fallback.md": "---\nversion: invalid\n---\n",
		"multi.md":   "---\nversion: ['1.0.0.0.5', '1.0']\n---\n",
		"backup.md":  "---\nbackup: '1.0.0'\n---\n",
		"types-a.md": "---\nlabel: '2'\namount: -1.2\nrank: low\ndate: 2026-01-01\n---\n",
		"types-b.md": "---\nlabel: '10'\namount: 0.5\nrank: high\ndate: 2026-02-01\n---\n",
	})
	buildVault(t, vault)
	cfg := searchVaultConfig(t, vault)
	for _, op := range []string{"=", "!=", "<", "<=", ">", ">="} {
		for _, rhs := range semverBoundaryValues {
			wc, err := ParseWhere([]string{"version" + op + rhs}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Search(vault, SearchOptions{Where: wc})
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			accepts := func(c int) bool {
				switch op {
				case "=":
					return c == 0
				case "!=":
					return c != 0
				case "<":
					return c < 0
				case "<=":
					return c <= 0
				case ">":
					return c > 0
				default:
					return c >= 0
				}
			}
			for i, value := range semverBoundaryValues {
				if accepts(semverOracle(value, rhs)) {
					want = append(want, fmt.Sprintf("%02d.md", i))
				}
			}
			equalMulti := semverOracle("1", rhs) == 0 || semverOracle("1.0.0.0.5", rhs) == 0
			if (op == "!=" && !equalMulti) || (op != "!=" && (accepts(semverOracle("1", rhs)) || accepts(semverOracle("1.0.0.0.5", rhs)))) {
				want = append(want, "multi.md")
			}
			if op == "!=" {
				want = append(want, "fallback.md")
			}
			slices.Sort(want)
			if !slices.Equal(searchPaths(got.Items), want) {
				t.Fatalf("version%s%s: got %v want %v", op, rhs, searchPaths(got.Items), want)
			}
		}
	}
	for _, desc := range []bool{false, true} {
		want := make([]string, len(semverBoundaryValues))
		for i := range want {
			want[i] = fmt.Sprintf("%02d.md", i)
		}
		valueFor := func(path string) string {
			if path == "multi.md" {
				if desc {
					return "1.0.0.0.5"
				}
				return "1"
			}
			var i int
			fmt.Sscanf(path, "%d.md", &i)
			return semverBoundaryValues[i]
		}
		want = append(want, "multi.md")
		slices.SortFunc(want, func(a, b string) int {
			cmp := semverOracle(valueFor(a), valueFor(b))
			if desc {
				cmp = -cmp
			}
			if cmp == 0 {
				return strings.Compare(a, b)
			}
			return cmp
		})
		if desc {
			want = append(want, "fallback.md")
		} else {
			want = append([]string{"fallback.md"}, want...)
		}
		want = append(want, "backup.md", "missing.md", "types-a.md", "types-b.md")
		key := "version"
		if desc {
			key = "-version"
		}
		got, err := Search(vault, SearchOptions{Sort: key})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(searchPaths(got.Items), want) {
			t.Fatalf("sort %s got %v want %v", key, searchPaths(got.Items), want)
		}
	}
	for _, expr := range []string{"coalesce(version,backup)=1", "coalesce(version,backup)!=1"} {
		wc, err := ParseWhere([]string{expr}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Search(vault, SearchOptions{Where: wc})
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(searchPaths(got.Items), "backup.md") != !strings.Contains(expr, "!=") {
			t.Fatalf("coalesce %s: %v", expr, searchPaths(got.Items))
		}
	}
	for _, key := range []string{"label", "amount", "rank", "date"} {
		got, err := Search(vault, SearchOptions{Sort: key})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"types-a.md", "types-b.md"}
		if key == "label" {
			want = []string{"types-b.md", "types-a.md"}
		}
		if !slices.Equal(searchPaths(got.Items)[:2], want) {
			t.Fatalf("other type %s: %v", key, searchPaths(got.Items))
		}
	}
	// Simultaneous independent handles and a reopen must all see the collation.
	for i := 0; i < 3; i++ {
		db, err := openDBAt(dbPath(vault))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM meta WHERE key='version' AND sort_value=?", "s:1.0.0.0").Scan(&count); err != nil || count != 6 {
			t.Fatalf("connection %d: count %d err %v", i, count, err)
		}
		rows, err := db.Query("SELECT value FROM meta INDEXED BY idx_meta_key_sort_value WHERE key='version' AND value_type='semver' ORDER BY sort_value")
		if err != nil {
			t.Fatal(err)
		}
		previous := "0"
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				t.Fatal(err)
			}
			if semverOracle(previous, v) > 0 {
				t.Fatalf("index order %q > %q", previous, v)
			}
			previous = v
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	// Build never rewrites the source values or quoted/numeric spellings.
	if got := readVaultFile(t, vault, "02.md"); got != "---\nversion: 1\n---\n" {
		t.Fatalf("source changed %q", got)
	}
}

func TestSemverMutationAndRebuild(t *testing.T) {
	vault := t.TempDir()
	writeInterpretationFiles(t, vault, map[string]string{"mdhop.toml": "[meta.types]\nversion='semver'\n", "a.md": "---\nversion: 1\n---\n"})
	buildVault(t, vault)
	writeInterpretationFiles(t, vault, map[string]string{"b.md": "---\nversion: '1.0.0.0.5'\n---\n"})
	if _, err := Add(vault, AddOptions{Files: []string{"b.md"}}); err != nil {
		t.Fatal(err)
	}
	writeInterpretationFiles(t, vault, map[string]string{"b.md": "---\nversion: '1.0.202610121457'\n---\n"})
	if _, err := Update(vault, UpdateOptions{Files: []string{"b.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Set(vault, SetOptions{File: "b.md", Key: "version", Value: "1.0.999999999999999999999999999999"}); err != nil {
		t.Fatal(err)
	}
	cfg := searchVaultConfig(t, vault)
	wc, err := ParseWhere([]string{"version>1.0.202610121457"}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Search(vault, SearchOptions{Where: wc, Fields: []string{"meta"}})
	if err != nil || got.Total != 1 {
		t.Fatalf("mutation result %+v %v", got, err)
	}
	if got.Items[0].Meta[0].Value != "1.0.999999999999999999999999999999" {
		t.Fatal("raw value lost")
	}
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
	if _, err := db.Exec("PRAGMA user_version=10"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	source, err := os.ReadFile(filepath.Join(vault, "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Search(vault, SearchOptions{}); err == nil {
		t.Fatal("old search accepted")
	}
	if _, err := Set(vault, SetOptions{File: "a.md", Key: "version", Value: "2"}); err == nil {
		t.Fatal("old mutation accepted")
	}
	if !slices.Equal(source, []byte(readVaultFile(t, vault, "a.md"))) {
		t.Fatal("failed mutation changed source")
	}
	buildVault(t, vault)
	if _, err := Search(vault, SearchOptions{}); err != nil {
		t.Fatal(err)
	}
}
