package core

import (
	"sort"
	"strings"
	"testing"
)

func TestParseWhere_Eq(t *testing.T) {
	wc, err := ParseWhere([]string{"priority=1"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wc.Conditions) != 1 {
		t.Fatalf("conditions = %d, want 1", len(wc.Conditions))
	}
	c := wc.Conditions[0]
	if c.Key != "priority" || c.Op != WhereOpEq || c.Value != "1" {
		t.Errorf("got {%q, %d, %q}, want {priority, Eq, 1}", c.Key, c.Op, c.Value)
	}
}

func TestParseWhere_Neq(t *testing.T) {
	wc, err := ParseWhere([]string{"status!=done"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "status" || c.Op != WhereOpNeq || c.Value != "done" {
		t.Errorf("got {%q, %d, %q}, want {status, Neq, done}", c.Key, c.Op, c.Value)
	}
}

func TestParseWhere_Like(t *testing.T) {
	wc, err := ParseWhere([]string{"status~act%"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "status" || c.Op != WhereOpLike || c.Value != "act%" {
		t.Errorf("got {%q, %d, %q}, want {status, Like, act%%}", c.Key, c.Op, c.Value)
	}
}

func TestParseWhere_Comparisons(t *testing.T) {
	metaCfg := MetaConfig{
		Types: map[string]MetaTypeInfo{
			"priority": {Name: MetaTypeNumber},
		},
	}
	tests := []struct {
		expr   string
		wantOp WhereOp
		wantV  string
	}{
		{"priority>1", WhereOpGt, "100000000000000000001.00000000"},
		{"priority<3", WhereOpLt, "100000000000000000003.00000000"},
		{"priority>=5", WhereOpGte, "100000000000000000005.00000000"},
		{"priority<=10", WhereOpLte, "100000000000000000010.00000000"},
	}
	for _, tt := range tests {
		wc, err := ParseWhere([]string{tt.expr}, metaCfg)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tt.expr, err)
		}
		c := wc.Conditions[0]
		if c.Op != tt.wantOp {
			t.Errorf("%s: op = %d, want %d", tt.expr, c.Op, tt.wantOp)
		}
		if c.Value != tt.wantV {
			t.Errorf("%s: value = %q, want %q", tt.expr, c.Value, tt.wantV)
		}
	}
}

func TestParseWhere_Exists(t *testing.T) {
	wc, err := ParseWhere([]string{"priority"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "priority" || c.Op != WhereOpExists || c.Value != "" {
		t.Errorf("got {%q, %d, %q}, want {priority, Exists, \"\"}", c.Key, c.Op, c.Value)
	}
}

func TestParseWhere_EmptyKey(t *testing.T) {
	_, err := ParseWhere([]string{"=value"}, MetaConfig{})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestParseWhere_EmptyValue(t *testing.T) {
	_, err := ParseWhere([]string{"key="}, MetaConfig{})
	if err == nil {
		t.Fatal("expected error for empty value")
	}
}

func TestParseWhere_NormalizationFailure(t *testing.T) {
	metaCfg := MetaConfig{
		Types: map[string]MetaTypeInfo{
			"priority": {Name: MetaTypeNumber},
		},
	}
	_, err := ParseWhere([]string{"priority>abc"}, metaCfg)
	if err == nil {
		t.Fatal("expected error for normalization failure")
	}
}

func TestParseWhere_OperatorPriority(t *testing.T) {
	metaCfg := MetaConfig{
		Types: map[string]MetaTypeInfo{
			"key": {Name: MetaTypeNumber},
		},
	}
	wc, err := ParseWhere([]string{"key>=5"}, metaCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Op != WhereOpGte {
		t.Errorf("op = %d, want %d (Gte)", c.Op, WhereOpGte)
	}
}

func TestParseWhere_ValueContainsEquals(t *testing.T) {
	wc, err := ParseWhere([]string{"title=A=B"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "title" || c.Op != WhereOpEq || c.Value != "A=B" {
		t.Errorf("got {%q, %d, %q}, want {title, Eq, A=B}", c.Key, c.Op, c.Value)
	}
}

func TestParseWhere_Empty(t *testing.T) {
	wc, err := ParseWhere(nil, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wc != nil {
		t.Errorf("expected nil, got %v", wc)
	}
}

func TestParseWhere_UndeclaredKeyComparison(t *testing.T) {
	// Undeclared key → string fallback (no error, lexicographic comparison).
	wc, err := ParseWhere([]string{"unknown>abc"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "unknown" || c.Op != WhereOpGt || c.Value != "abc" {
		t.Errorf("got {%q, %d, %q}, want {unknown, Gt, abc}", c.Key, c.Op, c.Value)
	}
}

// --- MetaFilterSQL tests ---

func TestWhereClause_Nil(t *testing.T) {
	var wc *WhereClause
	sql, args := wc.MetaFilterSQL("n.id")
	if sql != "" || args != nil {
		t.Errorf("nil: sql=%q args=%v, want empty", sql, args)
	}
}

func TestWhereClause_SingleEq(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "status", Op: WhereOpEq, Value: "active"},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "AND n.id IN") {
		t.Errorf("sql should contain AND n.id IN, got %q", sql)
	}
	if !strings.Contains(sql, "m.sort_value = ?") {
		t.Errorf("sql should contain m.sort_value = ?, got %q", sql)
	}
	if len(args) != 2 { // key + value
		t.Errorf("args = %v, want 2 elements", args)
	}
	if args[0] != "status" || args[1] != "active" {
		t.Errorf("args = %v, want [status active]", args)
	}
}

func TestWhereClause_SameKeyOR(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "priority", Op: WhereOpEq, Value: "2"},
		{Key: "priority", Op: WhereOpEq, Value: "3"},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	// Should have a single subquery (not INTERSECT).
	if strings.Contains(sql, "INTERSECT") {
		t.Errorf("same-key should not use INTERSECT: %q", sql)
	}
	// Should have OR.
	if !strings.Contains(sql, " OR ") {
		t.Errorf("same-key should use OR: %q", sql)
	}
	if len(args) != 3 { // key + value1 + value2
		t.Errorf("args = %v, want 3 elements", args)
	}
}

func TestWhereClause_DiffKeyAND(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "status", Op: WhereOpEq, Value: "active"},
		{Key: "priority", Op: WhereOpEq, Value: "1"},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "INTERSECT") {
		t.Errorf("different keys should use INTERSECT: %q", sql)
	}
	if len(args) != 4 { // key1 + val1 + key2 + val2
		t.Errorf("args = %v, want 4 elements", args)
	}
}

func TestWhereClause_Exists(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "priority", Op: WhereOpExists},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "m.key = ?") {
		t.Errorf("EXISTS should check key: %q", sql)
	}
	// Should NOT contain sort_value or value comparisons.
	if strings.Contains(sql, "sort_value") {
		t.Errorf("EXISTS should not compare sort_value: %q", sql)
	}
	if len(args) != 1 {
		t.Errorf("args = %v, want 1 element", args)
	}
}

func TestWhereClause_Neq(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "status", Op: WhereOpNeq, Value: "done"},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "NOT IN") {
		t.Errorf("!= should use NOT IN: %q", sql)
	}
	if len(args) != 3 { // key + key + value
		t.Errorf("args = %v, want 3 elements", args)
	}
}

func TestWhereClause_Like(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "status", Op: WhereOpLike, Value: "act%"},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "m.value LIKE ?") {
		t.Errorf("~ should use m.value LIKE: %q", sql)
	}
	if len(args) != 2 { // key + pattern
		t.Errorf("args = %v, want 2 elements", args)
	}
}

func TestWhereClause_Gt(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "priority", Op: WhereOpGt, Value: "100000000000000000001.00000000", valueType: "number"},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "m.sort_value > ?") {
		t.Errorf("> should use m.sort_value > ?: %q", sql)
	}
	if !strings.Contains(sql, "m.value_type = ?") {
		t.Errorf("> should have type guard: %q", sql)
	}
	if len(args) != 3 { // key + value + type
		t.Errorf("args = %v, want 3 elements", args)
	}
}

// --- Integration tests (vault_query_where) ---

func setupWhereVault(t *testing.T) string {
	t.Helper()
	vault := copyVaultForQuery(t, "vault_query_where")
	buildForQuery(t, vault)
	return vault
}

func whereNodeNames(nodes []NodeInfo) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}
	sort.Strings(names)
	return names
}

func assertNames(t *testing.T, label string, got []NodeInfo, want []string) {
	t.Helper()
	gotNames := whereNodeNames(got)
	sort.Strings(want)
	if len(gotNames) != len(want) {
		t.Errorf("%s: got %v, want %v", label, gotNames, want)
		return
	}
	for i := range gotNames {
		if gotNames[i] != want[i] {
			t.Errorf("%s: got %v, want %v", label, gotNames, want)
			return
		}
	}
}

func TestQueryBacklinksWhere_StatusEq(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status=active"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// B (active), E (active); C is done, D has no status.
	assertNames(t, "status=active", res.Backlinks, []string{"B", "E"})
}

func loadMetaCfg(t *testing.T, vault string) MetaConfig {
	t.Helper()
	cfg, err := LoadConfig(vault)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg.Meta
}

func TestQueryBacklinksWhere_PriorityGt(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"priority>1"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// B (2>1), C (3>1). D has no priority. E has priority=abc → value_type="string" → type guard excludes.
	assertNames(t, "priority>1", res.Backlinks, []string{"B", "C"})
}

func TestQueryBacklinksWhere_SameKeyOR(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"priority=2", "priority=3"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	assertNames(t, "priority=2 OR priority=3", res.Backlinks, []string{"B", "C"})
}

func TestQueryBacklinksWhere_DiffKeyAND(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status=active", "priority>1"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// B: active + priority=2 (>1) → match. C: done. E: active but priority=abc (type guard).
	assertNames(t, "status=active AND priority>1", res.Backlinks, []string{"B"})
}

func TestQueryBacklinksWhere_Exists(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"priority"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// B (priority=2), C (priority=3), E (priority=abc). D has no priority.
	assertNames(t, "EXISTS priority", res.Backlinks, []string{"B", "C", "E"})
}

func TestQueryBacklinksWhere_Neq(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status!=done"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// B (active, not done), E (active, not done). C is done → excluded. D has no status → no meta → excluded.
	assertNames(t, "status!=done", res.Backlinks, []string{"B", "E"})
}

func TestQueryBacklinksWhere_Like(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status~act%"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// B (active matches "act%"), E (active matches). C (done doesn't match). D has no status.
	assertNames(t, "status~act%", res.Backlinks, []string{"B", "E"})
}

func TestQueryOutgoingWhere(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status=active"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// B links to A. A has status=active.
	res, err := Query(vault, EntrySpec{File: "B.md"}, QueryOptions{
		Fields: []string{"outgoing"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	assertNames(t, "outgoing status=active", res.Outgoing, []string{"A"})
}

func TestQueryTwoHopWhere(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status=active"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// B→A, A→{B,C,D,E}. TwoHop from B through A should filter targets by status=active.
	res, err := Query(vault, EntrySpec{File: "B.md"}, QueryOptions{
		Fields: []string{"twohop"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// Via=A, targets should only include active notes (E is active, C is done, D has no status).
	// B itself is excluded from twohop targets (it's the entry).
	found := false
	for _, th := range res.TwoHop {
		if th.Via.Name == "A" {
			found = true
			assertNames(t, "twohop targets via A", th.Targets, []string{"E"})
		}
	}
	if !found {
		t.Error("expected twohop via A")
	}
}

func TestQueryTagsWhere_Unaffected(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status=active"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Tags should not be affected by where.
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"tags"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// A.md has no inline tags → tags should be empty/nil but no error.
	if res.Tags != nil && len(res.Tags) != 0 {
		t.Errorf("tags should be empty, got %v", res.Tags)
	}
}

func TestQueryBacklinksWhere_Nil(t *testing.T) {
	vault := setupWhereVault(t)
	// nil Where → no filter (backward compat).
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  nil,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// All backlinks: B, C, D, E.
	assertNames(t, "nil where", res.Backlinks, []string{"B", "C", "D", "E"})
}

func TestQueryBacklinksWhere_TypeGuard(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"priority>1"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// E has priority=abc → value_type="string" (fallback) → type guard "number" excludes it.
	assertNames(t, "type guard priority>1", res.Backlinks, []string{"B", "C"})
}

func TestQueryBacklinksWhere_AliasNeq(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"aliases!=beta"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  wc,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// B has aliases=[beta,bravo] → contains beta → excluded.
	// C has aliases=[charlie] → no beta → included.
	// D has no aliases key → no meta → excluded (!=  means "key exists AND value doesn't match").
	// E has no aliases key → excluded.
	assertNames(t, "aliases!=beta", res.Backlinks, []string{"C"})
}
