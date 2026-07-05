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
	// LIKE patterns may have meaningful leading/trailing whitespace — only the
	// key is trimmed, never the pattern itself.
	wc, err := ParseWhere([]string{"status~ act%"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := wc.Conditions[0]
	if c.Value != " act%" {
		t.Errorf("value = %q, want %q (leading space preserved)", c.Value, " act%")
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

// --- ParseWhere && tests ---

func TestParseWhere_And_TwoConds(t *testing.T) {
	metaCfg := MetaConfig{
		Types: map[string]MetaTypeInfo{
			"created": {Name: MetaTypeDate},
		},
	}
	wc, err := ParseWhere([]string{"created>=2025-02-01 && created<=2025-02-28"}, metaCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wc.Conditions) != 0 {
		t.Errorf("Conditions = %d, want 0", len(wc.Conditions))
	}
	if len(wc.AndGroups) != 1 {
		t.Fatalf("AndGroups = %d, want 1", len(wc.AndGroups))
	}
	g := wc.AndGroups[0]
	if len(g) != 2 {
		t.Fatalf("group len = %d, want 2", len(g))
	}
	if g[0].Key != "created" || g[0].Op != WhereOpGte {
		t.Errorf("g[0] = {%q, %d}, want {created, Gte}", g[0].Key, g[0].Op)
	}
	if g[1].Key != "created" || g[1].Op != WhereOpLte {
		t.Errorf("g[1] = {%q, %d}, want {created, Lte}", g[1].Key, g[1].Op)
	}
}

func TestParseWhere_And_ThreeConds(t *testing.T) {
	metaCfg := MetaConfig{
		Types: map[string]MetaTypeInfo{
			"a": {Name: MetaTypeNumber},
		},
	}
	wc, err := ParseWhere([]string{"a>1 && a<5 && b=x"}, metaCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wc.Conditions) != 0 {
		t.Errorf("Conditions = %d, want 0", len(wc.Conditions))
	}
	if len(wc.AndGroups) != 1 {
		t.Fatalf("AndGroups = %d, want 1", len(wc.AndGroups))
	}
	g := wc.AndGroups[0]
	if len(g) != 3 {
		t.Fatalf("group len = %d, want 3", len(g))
	}
	if g[0].Key != "a" || g[0].Op != WhereOpGt {
		t.Errorf("g[0] = {%q, %d}, want {a, Gt}", g[0].Key, g[0].Op)
	}
	if g[1].Key != "a" || g[1].Op != WhereOpLt {
		t.Errorf("g[1] = {%q, %d}, want {a, Lt}", g[1].Key, g[1].Op)
	}
	if g[2].Key != "b" || g[2].Op != WhereOpEq || g[2].Value != "x" {
		t.Errorf("g[2] = {%q, %d, %q}, want {b, Eq, x}", g[2].Key, g[2].Op, g[2].Value)
	}
}

func TestParseWhere_And_Mixed(t *testing.T) {
	metaCfg := MetaConfig{
		Types: map[string]MetaTypeInfo{
			"created": {Name: MetaTypeDate},
		},
	}
	wc, err := ParseWhere([]string{"status=active", "created>=2025-02-01 && created<=2025-02-28"}, metaCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wc.Conditions) != 1 {
		t.Errorf("Conditions = %d, want 1", len(wc.Conditions))
	}
	if wc.Conditions[0].Key != "status" || wc.Conditions[0].Op != WhereOpEq {
		t.Errorf("Conditions[0] = {%q, %d}, want {status, Eq}", wc.Conditions[0].Key, wc.Conditions[0].Op)
	}
	if len(wc.AndGroups) != 1 {
		t.Fatalf("AndGroups = %d, want 1", len(wc.AndGroups))
	}
	if len(wc.AndGroups[0]) != 2 {
		t.Fatalf("group len = %d, want 2", len(wc.AndGroups[0]))
	}
}

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

func TestParseWhere_And_Exists(t *testing.T) {
	wc, err := ParseWhere([]string{"priority && status"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wc.AndGroups) != 1 {
		t.Fatalf("AndGroups = %d, want 1", len(wc.AndGroups))
	}
	g := wc.AndGroups[0]
	if len(g) != 2 {
		t.Fatalf("group len = %d, want 2", len(g))
	}
	if g[0].Key != "priority" || g[0].Op != WhereOpExists {
		t.Errorf("g[0] = {%q, %d}, want {priority, Exists}", g[0].Key, g[0].Op)
	}
	if g[1].Key != "status" || g[1].Op != WhereOpExists {
		t.Errorf("g[1] = {%q, %d}, want {status, Exists}", g[1].Key, g[1].Op)
	}
}

func TestParseWhere_And_ExistsAndValue(t *testing.T) {
	wc, err := ParseWhere([]string{"priority && status=active"}, MetaConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wc.AndGroups) != 1 {
		t.Fatalf("AndGroups = %d, want 1", len(wc.AndGroups))
	}
	g := wc.AndGroups[0]
	if len(g) != 2 {
		t.Fatalf("group len = %d, want 2", len(g))
	}
	if g[0].Op != WhereOpExists {
		t.Errorf("g[0].Op = %d, want Exists", g[0].Op)
	}
	if g[1].Key != "status" || g[1].Op != WhereOpEq || g[1].Value != "active" {
		t.Errorf("g[1] = {%q, %d, %q}, want {status, Eq, active}", g[1].Key, g[1].Op, g[1].Value)
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
	if len(wc.AndGroups) != 0 {
		t.Errorf("AndGroups = %d, want 0", len(wc.AndGroups))
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

func TestWhereClause_MultipleFlagsSameKeyAND(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "priority", Op: WhereOpEq, Value: "2"},
		{Key: "priority", Op: WhereOpEq, Value: "3"},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "INTERSECT") {
		t.Errorf("same-key repeated flags should use INTERSECT: %q", sql)
	}
	if strings.Contains(sql, " OR ") {
		t.Errorf("same-key repeated flags should not use implicit OR: %q", sql)
	}
	if len(args) != 4 { // key + value for each flag
		t.Errorf("args = %v, want 4 elements", args)
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

func TestWhereClause_CoalesceExists(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "coalesce(reviewed, updated)", CoalesceKeys: []string{"reviewed", "updated"}, Op: WhereOpExists},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "m.key IN (?,?)") {
		t.Errorf("coalesce EXISTS should check any key with placeholders: %q", sql)
	}
	if len(args) != 2 || args[0] != "reviewed" || args[1] != "updated" {
		t.Errorf("args = %v, want [reviewed updated]", args)
	}
}

func TestWhereClause_CoalesceComparisonPriorityGuard(t *testing.T) {
	wc := &WhereClause{Conditions: []WhereCond{
		{Key: "coalesce(reviewed, updated)", CoalesceKeys: []string{"reviewed", "updated"}, Op: WhereOpLte, Value: "2025-07-04", valueType: "date"},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, " UNION ") {
		t.Errorf("coalesce comparison should union priority branches: %q", sql)
	}
	if !strings.Contains(sql, "NOT EXISTS") || !strings.Contains(sql, "mh.key IN (?)") {
		t.Errorf("lower-priority branch should guard higher-priority key existence: %q", sql)
	}
	wantArgs := []any{"reviewed", "2025-07-04", "date", "updated", "2025-07-04", "date", "reviewed"}
	if len(args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("args = %v, want %v", args, wantArgs)
		}
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

// --- AndGroup MetaFilterSQL tests ---

func TestWhereClause_AndGroup_SameKey(t *testing.T) {
	wc := &WhereClause{AndGroups: [][]WhereCond{
		{
			{Key: "created", Op: WhereOpGte, Value: "2025-02-01", valueType: "date"},
			{Key: "created", Op: WhereOpLte, Value: "2025-02-28", valueType: "date"},
		},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if sql == "" {
		t.Fatal("expected non-empty SQL for AndGroup-only clause")
	}
	if !strings.Contains(sql, "INTERSECT") {
		t.Errorf("same-key AND group should use INTERSECT: %q", sql)
	}
	if strings.Contains(sql, " OR ") {
		t.Errorf("AND group should not use OR: %q", sql)
	}
	// 2 subqueries: each has key + value + type = 3 args, total 6.
	if len(args) != 6 {
		t.Errorf("args = %v, want 6 elements", args)
	}
}

func TestWhereClause_AndGroup_DiffKeys(t *testing.T) {
	wc := &WhereClause{AndGroups: [][]WhereCond{
		{
			{Key: "priority", Op: WhereOpGt, Value: "100000000000000000001.00000000", valueType: "number"},
			{Key: "status", Op: WhereOpEq, Value: "active"},
		},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "INTERSECT") {
		t.Errorf("different keys in AND group should use INTERSECT: %q", sql)
	}
	// priority subquery: key + value + type = 3, status subquery: key + value = 2, total 5.
	if len(args) != 5 {
		t.Errorf("args = %v, want 5 elements", args)
	}
}

func TestWhereClause_AndGroup_WithSingles(t *testing.T) {
	wc := &WhereClause{
		Conditions: []WhereCond{
			{Key: "priority", Op: WhereOpEq, Value: "2"},
			{Key: "priority", Op: WhereOpEq, Value: "3"},
		},
		AndGroups: [][]WhereCond{
			{
				{Key: "created", Op: WhereOpGte, Value: "2025-02-01", valueType: "date"},
				{Key: "created", Op: WhereOpLte, Value: "2025-02-28", valueType: "date"},
			},
		},
	}
	sql, args := wc.MetaFilterSQL("n.id")
	// Should have INTERSECT (each single flag plus created>= plus created<=).
	if !strings.Contains(sql, "INTERSECT") {
		t.Errorf("mixed should use INTERSECT: %q", sql)
	}
	// Conditions: (key+val)*2 = 4, AndGroup: (key+val+type)*2 = 6, total 10.
	if len(args) != 10 {
		t.Errorf("args len = %d, want 10", len(args))
	}
}

func TestWhereClause_AndGroup_Only(t *testing.T) {
	wc := &WhereClause{AndGroups: [][]WhereCond{
		{
			{Key: "status", Op: WhereOpEq, Value: "active"},
		},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if sql == "" {
		t.Fatal("expected non-empty SQL for AndGroups-only clause")
	}
	if strings.Contains(sql, "INTERSECT") {
		t.Errorf("single condition should not use INTERSECT: %q", sql)
	}
	// key + value = 2.
	if len(args) != 2 {
		t.Errorf("args = %v, want 2 elements", args)
	}
}

func TestWhereClause_AndGroup_Like(t *testing.T) {
	wc := &WhereClause{AndGroups: [][]WhereCond{
		{
			{Key: "status", Op: WhereOpLike, Value: "act%"},
			{Key: "priority", Op: WhereOpGt, Value: "100000000000000000001.00000000", valueType: "number"},
		},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "LIKE") {
		t.Errorf("should contain LIKE: %q", sql)
	}
	if !strings.Contains(sql, "INTERSECT") {
		t.Errorf("should use INTERSECT: %q", sql)
	}
	// LIKE: key + pattern = 2, Gt: key + value + type = 3, total 5.
	if len(args) != 5 {
		t.Errorf("args len = %d, want 5", len(args))
	}
}

func TestWhereClause_AndGroup_Neq(t *testing.T) {
	wc := &WhereClause{AndGroups: [][]WhereCond{
		{
			{Key: "status", Op: WhereOpNeq, Value: "done"},
			{Key: "status", Op: WhereOpNeq, Value: "active"},
		},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "INTERSECT") {
		t.Errorf("neq AND group should use INTERSECT: %q", sql)
	}
	// Each neq subquery: key + key + value = 3, total 6.
	if len(args) != 6 {
		t.Errorf("args len = %d, want 6", len(args))
	}
}

// --- OrGroup MetaFilterSQL tests ---

func TestWhereClause_OrGroup_DiffKeys(t *testing.T) {
	wc := &WhereClause{OrGroups: [][]WhereCond{
		{
			{Key: "status", Op: WhereOpEq, Value: "active"},
			{Key: "priority", Op: WhereOpGt, Value: "100000000000000000001.00000000", valueType: "number"},
		},
	}}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, " UNION ") {
		t.Errorf("OR group should use UNION: %q", sql)
	}
	if strings.Contains(sql, "INTERSECT") {
		t.Errorf("single OR group should not use INTERSECT: %q", sql)
	}
	if len(args) != 5 {
		t.Errorf("args len = %d, want 5", len(args))
	}
}

func TestWhereClause_OrGroup_WithOtherFlag(t *testing.T) {
	wc := &WhereClause{
		Conditions: []WhereCond{
			{Key: "created", Op: WhereOpGte, Value: "2025-02-01", valueType: "date"},
		},
		OrGroups: [][]WhereCond{
			{
				{Key: "status", Op: WhereOpEq, Value: "active"},
				{Key: "priority", Op: WhereOpEq, Value: "3"},
			},
		},
	}
	sql, args := wc.MetaFilterSQL("n.id")
	if !strings.Contains(sql, "INTERSECT") {
		t.Errorf("OR group plus separate flag should be ANDed with INTERSECT: %q", sql)
	}
	if !strings.Contains(sql, " UNION ") {
		t.Errorf("OR group should still use UNION: %q", sql)
	}
	if len(args) != 7 {
		t.Errorf("args len = %d, want 7", len(args))
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

// --- AND group integration tests ---

func TestQueryBacklinksWhere_AndSameKey(t *testing.T) {
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
