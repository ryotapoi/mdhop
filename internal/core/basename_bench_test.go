package core

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

// CHANGE-VERIFY: Retain the original full-row lookup for reproducible before/after measurements.
func benchmarkBasenameFullScan(db dbExecer, typ NodeType, key string) ([]basenameMatch, error) {
	rows, err := db.Query(`SELECT id, name, path FROM nodes WHERE type=?`, typ)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var matches []basenameMatch
	for rows.Next() {
		var m basenameMatch
		var name string
		if err := rows.Scan(&m.id, &name, &m.path); err != nil {
			return nil, err
		}
		if strings.ToLower(normalizeTextNFC(name)) == key {
			m.path = NormalizePath(m.path)
			matches = append(matches, m)
		}
	}
	return matches, rows.Err()
}

func BenchmarkBasenameLookup(b *testing.B) {
	cases := []struct {
		size                      int
		background, target, shape string
		typ                       NodeType
	}{
		{1000, "ASCII", "ASCII", "unique", NodeTypeNote},
		{10000, "ASCII", "ASCII", "unique", NodeTypeNote},
		{100000, "ASCII", "ASCII", "unique", NodeTypeNote},
		{100000, "ASCII", "NFC", "root", NodeTypeNote},
		{100000, "ASCII", "NFD", "ambiguous", NodeTypeNote},
		{100000, "ASCII", "ASCII", "miss", NodeTypeNote},
		{100000, "NFC", "NFC", "unique", NodeTypeNote},
		{100000, "NFD", "NFD", "unique", NodeTypeNote},
		{100000, "ASCII", "ASCII", "unique", NodeTypeAsset},
	}
	for _, tc := range cases {
		b.Run(fmt.Sprintf("%d/%s/%s/%s/%s", tc.size, tc.background, tc.target, tc.shape, tc.typ), func(b *testing.B) {
			file := filepath.Join(b.TempDir(), "index.sqlite")
			db, err := openDBAt(file)
			if err != nil {
				b.Fatal(err)
			}
			defer db.Close()
			if err := initSchema(db); err != nil {
				b.Fatal(err)
			}
			tx, err := db.Begin()
			if err != nil {
				b.Fatal(err)
			}
			stmt, err := tx.Prepare(`INSERT INTO nodes(node_key,type,name,path) VALUES(?,?,?,?)`)
			if err != nil {
				b.Fatal(err)
			}
			for i := 0; i < tc.size; i++ {
				name := fmt.Sprintf("Note%06d", i)
				if tc.background != "ASCII" {
					name = "Café" + name
				}
				if tc.background == "NFD" {
					name = norm.NFD.String(name)
				}
				if _, err := stmt.Exec(fmt.Sprintf("row%d", i), tc.typ, name, fmt.Sprintf("sub/%d.md", i)); err != nil {
					b.Fatal(err)
				}
			}
			target := "Target"
			if tc.target != "ASCII" {
				target = "Café"
			}
			stored := target
			if tc.target == "NFD" {
				stored = norm.NFD.String(target)
			}
			if tc.shape != "miss" {
				paths := []string{"sub/Target.md"}
				if tc.shape == "root" {
					paths = append(paths, "Target.md")
				}
				if tc.shape == "ambiguous" {
					paths = append(paths, "other/Target.md")
				}
				for i, p := range paths {
					if _, err := stmt.Exec(fmt.Sprintf("target%d", i), tc.typ, stored, p); err != nil {
						b.Fatal(err)
					}
				}
			}
			if err := stmt.Close(); err != nil {
				b.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				b.Fatal(err)
			}
			key := strings.ToLower(target)
			for _, impl := range []struct {
				name   string
				lookup func(dbExecer, NodeType, string) ([]basenameMatch, error)
			}{{"full_scan", benchmarkBasenameFullScan}, {"prefilter", queryBasenameMatches}} {
				for _, mode := range []string{"warm", "open"} {
					if mode == "open" && (tc.size != 100000 || tc.shape != "unique" || tc.typ != NodeTypeNote || tc.target != tc.background) {
						continue
					}
					b.Run(impl.name+"/"+mode, func(b *testing.B) {
						b.ReportAllocs()
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							conn := db
							if mode == "open" {
								var err error
								conn, err = openDBAt(file)
								if err != nil {
									b.Fatal(err)
								}
							}
							matches, err := impl.lookup(conn, tc.typ, key)
							if mode == "open" {
								if closeErr := conn.Close(); closeErr != nil {
									b.Fatal(closeErr)
								}
							}
							if err != nil {
								b.Fatal(err)
							}
							want := 1
							if tc.shape == "miss" {
								want = 0
							}
							if tc.shape == "root" || tc.shape == "ambiguous" {
								want = 2
							}
							if len(matches) != want {
								b.Fatalf("matches=%d, want %d", len(matches), want)
							}
						}
					})
				}
			}
		})
	}
}
