package core

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestBasenameUnicodeAndLiteralCandidates(t *testing.T) {
	tests := []struct {
		name, target string
		names        []string
		want         []int64
	}{
		{"ASCII case", "target", []string{"TARGET", "Other"}, []int64{1}},
		{"legacy NFD", "Café", []string{"Cafe\u0301", "Cafeteria"}, []int64{1}},
		{"Unicode case", "école", []string{"ÉCOLE", "School"}, []int64{1}},
		{"Kelvin to ASCII", "k", []string{"K", "K", "Other"}, []int64{1, 2}},
		{"dotted I to ASCII", "i", []string{"İ", "I", "Other"}, []int64{1, 2}},
		{"long s stays distinct", "s", []string{"ſ", "S"}, []int64{2}},
		{"literal patterns", "A%_*?[x]", []string{"A%_*?[x]", "A123", "Ax"}, []int64{1}},
		{"NUL before Unicode", "a\x00é", []string{"A\x00É", "A\x00E"}, []int64{1}},
		{"invalid UTF-8", "�", []string{"\xff", "Other"}, []int64{1}},
		{"miss", "Missing", []string{"Other", "別名"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t, filepath.Join(t.TempDir(), "index.sqlite"))
			defer db.Close()
			if err := initSchema(db); err != nil {
				t.Fatal(err)
			}
			for i, name := range tt.names {
				if _, err := db.Exec(`INSERT INTO nodes(id,node_key,type,name,path) VALUES(?,?,'note',?,?)`, i+1, name, name, "sub/"+name+".md"); err != nil {
					t.Fatal(err)
				}
			}
			key := strings.ToLower(normalizeTextNFC(tt.target))
			matches, err := queryBasenameMatches(db, NodeTypeNote, key)
			if err != nil {
				t.Fatal(err)
			}
			var ids []int64
			for _, m := range matches {
				ids = append(ids, m.id)
			}
			// SQL does not promise candidate order.
			slices.Sort(ids)
			if !reflect.DeepEqual(ids, tt.want) {
				t.Fatalf("IDs=%v, want %v", ids, tt.want)
			}
			if len(tt.want) == 1 {
				id, _, err := findEntryByName(db, tt.target)
				if err != nil || id != tt.want[0] {
					t.Fatalf("query id=%d err=%v", id, err)
				}
				id, _, err = resolveBasenameFromDB(db, tt.target, linkOccur{})
				if err != nil || id != tt.want[0] {
					t.Fatalf("resolve id=%d err=%v", id, err)
				}
			}
		})
	}
}
