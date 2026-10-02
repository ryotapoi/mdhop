package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestExpandRelativeDate(t *testing.T) {
	now := time.Date(2026, 6, 11, 9, 30, 0, 0, time.Local)
	tests := []struct {
		token string
		want  string
		ok    bool
	}{
		{"today", "2026-06-11", true},
		{"today-90d", "2026-03-13", true},
		{"today+1d", "2026-06-12", true},
		{"today-1w", "2026-06-04", true},
		{"today-3m", "2026-03-11", true},
		{"today-1y", "2025-06-11", true},
		{"today+2w", "2026-06-25", true},
		// Non-relative inputs are left to the caller.
		{"2026-03-01", "", false},
		{"yesterday", "", false},
		{"today-", "", false},
		{"today-5", "", false},
		{"today-5x", "", false},
		{"todayx", "", false},
		{"today-90days", "", false},
	}
	for _, tt := range tests {
		got, ok := ExpandRelativeDate(tt.token, now)
		if ok != tt.ok {
			t.Errorf("%s: ok = %v, want %v", tt.token, ok, tt.ok)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.token, got, tt.want)
		}
	}
}

func TestParseWhere_RelativeDate(t *testing.T) {
	// updated is not declared as date; relative date must still force date type.
	wc, err := ParseWhere([]string{"updated<today-90d"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "updated" || c.Op != WhereOpLt {
		t.Fatalf("got {%q, %d}, want {updated, Lt}", c.Key, c.Op)
	}
	if c.valueType != string(MetaTypeDate) {
		t.Errorf("valueType = %q, want %q", c.valueType, MetaTypeDate)
	}
	// Value must be a normalized date string (YYYY-MM-DD), not the raw token.
	if _, err := time.Parse("2006-01-02", c.Value); err != nil {
		t.Errorf("value = %q is not a normalized date: %v", c.Value, err)
	}
}

func TestParseWhere_RelativeDate_EqRejected(t *testing.T) {
	// Relative dates only make sense for range comparisons, but = / != still
	// expand the token (point-in-time match on that day's normalized value).
	wc, err := ParseWhere([]string{"updated=today"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if _, err := time.Parse("2006-01-02", c.Value); err != nil {
		t.Errorf("value = %q is not a normalized date: %v", c.Value, err)
	}
	if c.valueType != string(MetaTypeDate) {
		t.Errorf("valueType = %q, want date", c.valueType)
	}
}

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

func TestParseWhere_LeftmostOperator(t *testing.T) {
	tests := []struct {
		expr  string
		key   string
		op    WhereOp
		value string
	}{
		{"url~https://example.test/?q=x", "url", WhereOpLike, "https://example.test/?q=x"},
		{"title=a!=b", "title", WhereOpEq, "a!=b"},
		{"title>a=b", "title", WhereOpGt, "a=b"},
		{"title<a=b", "title", WhereOpLt, "a=b"},
		{"title>=a!=b", "title", WhereOpGte, "a!=b"},
		{"title<=a!=b", "title", WhereOpLte, "a!=b"},
		{"title!=a=b", "title", WhereOpNeq, "a=b"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			wc, err := ParseWhere([]string{tt.expr}, MetaConfig{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(wc.Conditions) != 1 {
				t.Fatalf("conditions = %d, want 1", len(wc.Conditions))
			}
			c := wc.Conditions[0]
			if c.Key != tt.key || c.Op != tt.op || c.Value != tt.value {
				t.Errorf("got {%q, %d, %q}, want {%q, %d, %q}", c.Key, c.Op, c.Value, tt.key, tt.op, tt.value)
			}
		})
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

func TestParseWhere_Like_PreservesWhitespace(t *testing.T) {
	// Spec: docs/specs/overview.md — `~` right-hand side whitespace handling.
	for _, tt := range []struct {
		name  string
		expr  string
		value string
	}{
		{"leading", "title~ act%", " act%"},
		{"trailing", "title~% ", "% "},
		{"both", " title ~ % ", " % "},
		{"whitespace only", "title~ ", " "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wc, err := ParseWhere([]string{tt.expr}, MetaConfig{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			c := wc.Conditions[0]
			if c.Key != "title" || c.Op != WhereOpLike || c.Value != tt.value {
				t.Errorf("got {%q, %d, %q}, want {title, Like, %q}", c.Key, c.Op, c.Value, tt.value)
			}
		})
	}
}

func TestParseWhere_Like_JoinedPreservesWhitespace(t *testing.T) {
	for _, separator := range []string{" && ", " || "} {
		t.Run(separator, func(t *testing.T) {
			wc, err := ParseWhere([]string{"title~ % " + separator + "title~% "}, MetaConfig{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			conds := wc.Conditions
			if separator == " || " {
				conds = wc.OrGroups[0]
			}
			for i, want := range []string{" % ", "% "} {
				if conds[i].Value != want {
					t.Errorf("condition %d value = %q, want %q", i, conds[i].Value, want)
				}
			}
		})
	}
}

func TestParseWhere_NonLikeTrimsWhitespace(t *testing.T) {
	for _, tt := range []struct {
		expr  string
		op    WhereOp
		value string
	}{
		{" title = active ", WhereOpEq, "active"},
		{" title ", WhereOpExists, ""},
	} {
		wc, err := ParseWhere([]string{tt.expr}, MetaConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		c := wc.Conditions[0]
		if c.Key != "title" || c.Op != tt.op || c.Value != tt.value {
			t.Errorf("%q: got {%q, %d, %q}, want {title, %d, %q}", tt.expr, c.Key, c.Op, c.Value, tt.op, tt.value)
		}
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

func TestParseWhere_NotExists(t *testing.T) {
	wc, err := ParseWhere([]string{"priority NOT EXISTS"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "priority" || c.Op != WhereOpNotExists || c.Value != "" {
		t.Errorf("got {%q, %d, %q}, want {priority, NotExists, \"\"}", c.Key, c.Op, c.Value)
	}
}

func TestParseWhere_NotExistsTrimsSpace(t *testing.T) {
	wc, err := ParseWhere([]string{" priority NOT EXISTS "}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "priority" || c.Op != WhereOpNotExists || c.Value != "" {
		t.Errorf("got {%q, %d, %q}, want {priority, NotExists, \"\"}", c.Key, c.Op, c.Value)
	}
}

func TestParseWhere_CoalesceComparison(t *testing.T) {
	metaCfg := MetaConfig{
		Types: map[string]MetaTypeInfo{
			"reviewed": {Name: MetaTypeDate},
			"updated":  {Name: MetaTypeString},
		},
	}
	wc, err := ParseWhere([]string{"coalesce(reviewed, updated) <= 2025-07-04"}, metaCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "coalesce(reviewed, updated)" || c.Op != WhereOpLte || c.Value != "2025-07-04" {
		t.Errorf("got {%q, %d, %q}, want {coalesce(reviewed, updated), Lte, 2025-07-04}", c.Key, c.Op, c.Value)
	}
	if got, want := strings.Join(c.CoalesceKeys, ","), "reviewed,updated"; got != want {
		t.Errorf("CoalesceKeys = %q, want %q", got, want)
	}
	if c.valueType != string(MetaTypeDate) {
		t.Errorf("valueType = %q, want date", c.valueType)
	}
	if got := c.keyValues["reviewed"]; got != (whereValue{value: "2025-07-04", valueType: "date"}) {
		t.Errorf("reviewed keyValue = %+v, want date-normalized date", got)
	}
	if got := c.keyValues["updated"]; got != (whereValue{value: "2025-07-04", valueType: "string"}) {
		t.Errorf("updated keyValue = %+v, want string-normalized date", got)
	}
}

func TestParseWhere_CoalesceExists(t *testing.T) {
	wc, err := ParseWhere([]string{"coalesce(reviewed, updated)"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "coalesce(reviewed, updated)" || c.Op != WhereOpExists {
		t.Errorf("got {%q, %d}, want {coalesce(reviewed, updated), Exists}", c.Key, c.Op)
	}
	if got, want := strings.Join(c.CoalesceKeys, ","), "reviewed,updated"; got != want {
		t.Errorf("CoalesceKeys = %q, want %q", got, want)
	}
}

func TestParseWhere_CoalesceNotExists(t *testing.T) {
	wc, err := ParseWhere([]string{"coalesce(reviewed, updated) NOT EXISTS"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Key != "coalesce(reviewed, updated)" || c.Op != WhereOpNotExists {
		t.Errorf("got {%q, %d}, want {coalesce(reviewed, updated), NotExists}", c.Key, c.Op)
	}
	if got, want := strings.Join(c.CoalesceKeys, ","), "reviewed,updated"; got != want {
		t.Errorf("CoalesceKeys = %q, want %q", got, want)
	}
}

func TestParseWhere_CoalesceInvalid(t *testing.T) {
	tests := []string{
		"coalesce()<=2025-07-04",
		"coalesce(reviewed)<=2025-07-04",
		"coalesce(reviewed, )<=2025-07-04",
		"coalesce(reviewed, updated<=2025-07-04",
		"coalesce(reviewed, reviewed)<=2025-07-04",
		"coalesce(reviewed,  reviewed)<=2025-07-04",
	}
	for _, expr := range tests {
		t.Run(expr, func(t *testing.T) {
			_, err := ParseWhere([]string{expr}, MetaConfig{})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "where:") {
				t.Errorf("error = %v, want where-prefixed error", err)
			}
		})
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

func TestParseWhere_NotExistsEmptyKey(t *testing.T) {
	_, err := ParseWhere([]string{" NOT EXISTS"}, MetaConfig{})
	if err == nil {
		t.Fatal("expected error for empty NOT EXISTS key")
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

// --- ParseWhere && tests ---

func TestParseWhere_And_EmptyPart(t *testing.T) {
	_, err := ParseWhere([]string{"status=active && "}, MetaConfig{})
	if err == nil {
		t.Fatal("expected error for trailing empty part")
	}
}

func TestParseWhere_And_LeadingEmptyPart(t *testing.T) {
	_, err := ParseWhere([]string{" && status=active"}, MetaConfig{})
	if err == nil {
		t.Fatal("expected error for leading empty part")
	}
}

func TestParseWhere_And_OnlySeparator(t *testing.T) {
	_, err := ParseWhere([]string{" && "}, MetaConfig{})
	if err == nil {
		t.Fatal("expected error for only-separator input")
	}
}

func TestParseWhere_And_NoSpaceNotSplit(t *testing.T) {
	metaCfg := MetaConfig{
		Types: map[string]MetaTypeInfo{
			"created": {Name: MetaTypeDate},
		},
	}
	// "&&" without surrounding spaces should NOT be treated as a separator.
	// "created>=X&&created<=Y" is parsed as a single expression:
	// key=created, op=>=, value=X&&created<=Y → date normalization error.
	_, err := ParseWhere([]string{"created>=X&&created<=Y"}, metaCfg)
	if err == nil {
		t.Fatal("expected normalization error for non-date value")
	}
}

// --- ParseWhere || tests ---

func TestParseWhere_Or_TwoConds(t *testing.T) {
	wc, err := ParseWhere([]string{"status=active || priority>1"}, MetaConfig{
		Types: map[string]MetaTypeInfo{
			"priority": {Name: MetaTypeNumber},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wc.Conditions) != 0 {
		t.Errorf("Conditions = %d, want 0", len(wc.Conditions))
	}
	if len(wc.OrGroups) != 1 {
		t.Fatalf("OrGroups = %d, want 1", len(wc.OrGroups))
	}
	g := wc.OrGroups[0]
	if len(g) != 2 {
		t.Fatalf("group len = %d, want 2", len(g))
	}
	if g[0].Key != "status" || g[0].Op != WhereOpEq || g[0].Value != "active" {
		t.Errorf("g[0] = {%q, %d, %q}, want {status, Eq, active}", g[0].Key, g[0].Op, g[0].Value)
	}
	if g[1].Key != "priority" || g[1].Op != WhereOpGt {
		t.Errorf("g[1] = {%q, %d}, want {priority, Gt}", g[1].Key, g[1].Op)
	}
}

func TestParseWhere_Or_Coalesce(t *testing.T) {
	wc, err := ParseWhere([]string{"coalesce(reviewed, updated)<=2025-07-04 || status=done"}, MetaConfig{
		Types: map[string]MetaTypeInfo{
			"reviewed": {Name: MetaTypeDate},
			"updated":  {Name: MetaTypeDate},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wc.OrGroups) != 1 {
		t.Fatalf("OrGroups = %d, want 1", len(wc.OrGroups))
	}
	c := wc.OrGroups[0][0]
	if c.Key != "coalesce(reviewed, updated)" || c.Op != WhereOpLte {
		t.Errorf("coalesce condition = {%q, %d}, want {coalesce(reviewed, updated), Lte}", c.Key, c.Op)
	}
	if got, want := strings.Join(c.CoalesceKeys, ","), "reviewed,updated"; got != want {
		t.Errorf("CoalesceKeys = %q, want %q", got, want)
	}
}

func TestParseWhere_Or_MixedWithAndRejected(t *testing.T) {
	_, err := ParseWhere([]string{"status=active || priority>1 && created<today"}, MetaConfig{})
	if err == nil {
		t.Fatal("expected error for mixed || and &&")
	}
	if !strings.Contains(err.Error(), "cannot mix && and ||") {
		t.Errorf("error = %v, want mixed-separator message", err)
	}
}

func TestParseWhere_Or_EmptyPart(t *testing.T) {
	_, err := ParseWhere([]string{"status=active || "}, MetaConfig{})
	if err == nil {
		t.Fatal("expected error for trailing empty OR part")
	}
}

func TestParseWhere_Or_LeadingEmptyPart(t *testing.T) {
	_, err := ParseWhere([]string{" || status=active"}, MetaConfig{})
	if err == nil {
		t.Fatal("expected error for leading empty OR part")
	}
}

func TestParseWhere_Or_NoSpaceNotSplit(t *testing.T) {
	wc, err := ParseWhere([]string{"status=active||status=done"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wc.Conditions) != 1 {
		t.Fatalf("Conditions = %d, want 1", len(wc.Conditions))
	}
	if wc.Conditions[0].Value != "active||status=done" {
		t.Errorf("value = %q, want literal unsplit value", wc.Conditions[0].Value)
	}
}

// --- MetaFilterSQL tests ---

func TestWhereClause_Nil(t *testing.T) {
	var wc *WhereClause
	sql, args := wc.MetaFilterSQL("n.id")
	if sql != "" || args != nil {
		t.Errorf("nil: sql=%q args=%v, want empty", sql, args)
	}

	sql, args = (&WhereClause{}).MetaFilterSQL("n.id")
	if sql != "" || args != nil {
		t.Errorf("empty: sql=%q args=%v, want empty", sql, args)
	}
}

func TestWhereClause_NotExists(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "priority", Op: WhereOpNotExists},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "NOT EXISTS") {
		t.Errorf("NOT EXISTS should use anti-exists subquery: %q", sql)
	}
	if !strings.Contains(sql, "n2.type = 'note'") || !strings.Contains(sql, "n2.exists_flag = 1") {
		t.Errorf("NOT EXISTS should only return existing notes: %q", sql)
	}
	if len(args) != 1 {
		t.Errorf("args = %v, want 1 element", args)
	}
	if args[0] != "priority" {
		t.Errorf("args = %v, want [priority]", args)
	}
}

func TestWhereClause_CoalesceComparisonPerKeyTypes(t *testing.T) {
	metaCfg := MetaConfig{
		Types: map[string]MetaTypeInfo{
			"reviewed": {Name: MetaTypeDate},
			"updated":  {Name: MetaTypeString},
		},
	}
	wc, err := ParseWhere([]string{"coalesce(reviewed, updated)<=2025-7-4"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "m.value_type = ?") {
		t.Fatalf("coalesce comparison should use type guards: %q", sql)
	}
	wantArgs := []any{"reviewed", "2025-07-04", "date", "updated", "2025-7-4", "string", "reviewed"}
	if len(args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("args = %v, want %v", args, wantArgs)
		}
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

func TestQueryBacklinksWhere_MultipleFlagsSameKeyAND(t *testing.T) {
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
	assertNames(t, "priority=2 AND priority=3", res.Backlinks, nil)
}

func TestQueryBacklinksWhere_SameKeyOrExpression(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"priority=2 || priority=3"}, metaCfg)
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
	assertNames(t, "priority=2 || priority=3", res.Backlinks, []string{"B", "C"})
}

func TestQueryBacklinksWhere_OrExpression(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status=done || priority=2"}, metaCfg)
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
	// B matches priority=2. C matches status=done.
	assertNames(t, "status=done || priority=2", res.Backlinks, []string{"B", "C"})
}

func TestQueryBacklinksWhere_OrExpressionAndSeparateFlag(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status=active || status=done", "priority=2"}, metaCfg)
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
	// The OR expression admits B/C/E by status, then the separate flag ANDs priority=2.
	assertNames(t, "(status active OR done) AND priority=2", res.Backlinks, []string{"B"})
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

func TestQueryBacklinksWhere_NotExists(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"priority NOT EXISTS"}, metaCfg)
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
	// D links to A and has no priority key. B/C/E all have priority.
	assertNames(t, "priority NOT EXISTS", res.Backlinks, []string{"D"})
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

func TestQueryBacklinksWhere_CoalescePriority(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"coalesce(reviewed, updated)<=2025-07-04"}, metaCfg)
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
	// B matches by reviewed. E has no reviewed, so it falls back to updated and matches.
	// C would match by updated under a naive OR, but reviewed exists and is too recent.
	assertNames(t, "coalesce(reviewed, updated)<=2025-07-04", res.Backlinks, []string{"B", "E"})
}

func TestQueryBacklinksWhere_CoalesceExists(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"coalesce(reviewed, updated)"}, metaCfg)
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
	assertNames(t, "coalesce(reviewed, updated)", res.Backlinks, []string{"B", "C", "E"})
}

func TestQueryBacklinksWhere_CoalesceDifferingTypesFallback(t *testing.T) {
	vault := copyVaultForQuery(t, "vault_query_where")
	cfg := []byte("meta:\n  types:\n    priority: number\n    status: string\n    created: date\n    reviewed: date\n")
	if err := os.WriteFile(filepath.Join(vault, "mdhop.yaml"), cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	buildForQuery(t, vault)
	metaCfg := loadMetaCfg(t, vault)
	if _, ok := metaCfg.Types["updated"]; ok {
		t.Fatal("updated must be undeclared for this regression test")
	}
	wc, err := ParseWhere([]string{"coalesce(reviewed, updated)<=2025-07-04"}, metaCfg)
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
	// E has only updated. With updated undeclared, it is stored as value_type=string
	// and must still be compared by the updated branch's own string type.
	assertNames(t, "coalesce reviewed date, updated string", res.Backlinks, []string{"B", "E"})
}

func TestQueryBacklinksWhere_CoalesceEqAndNeqParenthesized(t *testing.T) {
	vault := copyVaultForQuery(t, "vault_query_where")
	note := "---\npriority: 9\nstatus: active\ncreated: 2025-04-01\nupdated: 2025-01-01\n---\n\n# F\n\nLinks to A:\n- [[A]]\n"
	if err := os.WriteFile(filepath.Join(vault, "F.md"), []byte(note), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	buildForQuery(t, vault)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{
		"coalesce(reviewed, updated)=2024-01-01",
		"coalesce(reviewed, updated)!=2026-01-01",
	}, metaCfg)
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
	assertNames(t, "coalesce eq intersect neq", res.Backlinks, []string{"B", "E"})
}

func TestQueryBacklinksWhere_OrExpressionCoalesce(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"coalesce(reviewed, updated)<=2025-07-04 || status=done"}, metaCfg)
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
	// B/E match coalesce; C matches status=done despite coalesce selecting reviewed=2026-01-01.
	assertNames(t, "coalesce old OR status done", res.Backlinks, []string{"B", "C", "E"})
}

// --- AND integration tests ---

func TestQueryBacklinksWhere_AndExpressionMatchesRepeatedFlags(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"priority>=2 && priority<=3"}, metaCfg)
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
	// B (priority=2, >=2 AND <=3), C (priority=3, >=2 AND <=3).
	// E has priority=abc → type guard excludes. D has no priority.
	assertNames(t, "priority>=2 && priority<=3", res.Backlinks, []string{"B", "C"})

	repeatedFlags, err := ParseWhere([]string{"priority>=2", "priority<=3"}, metaCfg)
	if err != nil {
		t.Fatalf("parse repeated flags: %v", err)
	}
	flagResult, err := Query(vault, EntrySpec{File: "A.md"}, QueryOptions{
		Fields: []string{"backlinks"},
		Where:  repeatedFlags,
	})
	if err != nil {
		t.Fatalf("query repeated flags: %v", err)
	}
	assertNames(t, "repeated flags", flagResult.Backlinks, []string{"B", "C"})
}

func TestSearchWhere_AndSameKey(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"created>=2025-02-01 && created<=2025-02-28"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Search(vault, SearchOptions{
		Where: wc,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	// A: 2025-01-15 (before range). B: 2025-02-10 (in range). C: 2025-02-20 (in range).
	// D: no created. E: 2025-03-05 (after range).
	var nodes []NodeInfo
	for _, item := range res.Items {
		nodes = append(nodes, item.Node)
	}
	assertNames(t, "created range", nodes, []string{"B", "C"})
}

func TestSearchWhere_MultipleFlagsSameKeyAND(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status=active", "status=done"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Search(vault, SearchOptions{
		Where: wc,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	var nodes []NodeInfo
	for _, item := range res.Items {
		nodes = append(nodes, item.Node)
	}
	assertNames(t, "search status active AND status done", nodes, nil)
}

func TestSearchWhere_OrExpression(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status=done || priority=2"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Search(vault, SearchOptions{
		Where: wc,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	var nodes []NodeInfo
	for _, item := range res.Items {
		nodes = append(nodes, item.Node)
	}
	assertNames(t, "search status done OR priority 2", nodes, []string{"B", "C"})
}

func TestSearchWhere_NotExists(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"priority NOT EXISTS"}, metaCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Search(vault, SearchOptions{
		Where: wc,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	var nodes []NodeInfo
	for _, item := range res.Items {
		nodes = append(nodes, item.Node)
	}
	assertNames(t, "search priority NOT EXISTS", nodes, []string{"D"})
}

func TestQueryBacklinksWhere_AndMixedKeys(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status!=done && priority>1"}, metaCfg)
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
	// status!=done: B(active), E(active). C(done) excluded. D(no meta) excluded.
	// priority>1: B(2), C(3). E(abc) excluded by type guard.
	// Intersection: B only.
	assertNames(t, "status!=done && priority>1", res.Backlinks, []string{"B"})
}

func TestQueryBacklinksWhere_AndNeqSameKey(t *testing.T) {
	vault := setupWhereVault(t)
	metaCfg := loadMetaCfg(t, vault)
	wc, err := ParseWhere([]string{"status!=done && status!=active"}, metaCfg)
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
	// B(active) excluded by status!=active. C(done) excluded by status!=done.
	// E(active) excluded by status!=active. D has no status → excluded (no meta row).
	// All excluded → empty result.
	assertNames(t, "status!=done && status!=active", res.Backlinks, nil)
}
