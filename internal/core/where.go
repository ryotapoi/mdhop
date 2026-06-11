package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// WhereOp represents a comparison operator for --where conditions.
type WhereOp int

const (
	WhereOpEq        WhereOp = iota // =
	WhereOpNeq                      // !=
	WhereOpLike                     // ~
	WhereOpGt                       // >
	WhereOpLt                       // <
	WhereOpGte                      // >=
	WhereOpLte                      // <=
	WhereOpExists                   // key only (no operator)
	WhereOpNotExists                // key NOT EXISTS
)

// WhereCond represents a single where condition.
type WhereCond struct {
	Key       string
	Op        WhereOp
	Value     string // normalized sort_value for =, !=, >, <, >=, <=; raw pattern for ~; empty for EXISTS
	valueType string // value_type for type-guarded comparisons (>, <, >=, <=)
}

// WhereClause holds parsed where conditions.
type WhereClause struct {
	Conditions []WhereCond   // from non-&& --where flags (same-key = OR)
	AndGroups  [][]WhereCond // from && --where flags (each group = all AND)
}

// operatorTable lists operators in priority order (longest first).
var operatorTable = []struct {
	str string
	op  WhereOp
}{
	{"!=", WhereOpNeq},
	{">=", WhereOpGte},
	{"<=", WhereOpLte},
	{"=", WhereOpEq},
	{"~", WhereOpLike},
	{">", WhereOpGt},
	{"<", WhereOpLt},
}

// ParseWhere parses a list of where expressions into a WhereClause.
// Empty input returns (nil, nil).
// Expressions containing " && " are split into AND groups where all
// conditions must be satisfied (even for the same key).
func ParseWhere(exprs []string, metaCfg MetaConfig) (*WhereClause, error) {
	if len(exprs) == 0 {
		return nil, nil
	}

	var conds []WhereCond
	var andGroups [][]WhereCond
	for _, expr := range exprs {
		parts := strings.Split(expr, " && ")
		if len(parts) == 1 {
			// Single condition — append to Conditions (same-key = OR).
			c, err := parseOneWhere(expr, metaCfg)
			if err != nil {
				return nil, err
			}
			conds = append(conds, c)
		} else {
			// Multiple conditions joined by && — all AND.
			var group []WhereCond
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					return nil, fmt.Errorf("where: empty condition in && expression %q", expr)
				}
				c, err := parseOneWhere(p, metaCfg)
				if err != nil {
					return nil, err
				}
				group = append(group, c)
			}
			andGroups = append(andGroups, group)
		}
	}
	if len(conds) == 0 && len(andGroups) == 0 {
		return nil, nil
	}
	return &WhereClause{Conditions: conds, AndGroups: andGroups}, nil
}

func parseOneWhere(expr string, metaCfg MetaConfig) (WhereCond, error) {
	expr = strings.TrimSpace(expr)

	if key, ok := parseNotExistsWhere(expr); ok {
		if key == "" {
			return WhereCond{}, fmt.Errorf("where: empty key in %q", expr)
		}
		return WhereCond{Key: key, Op: WhereOpNotExists, Value: ""}, nil
	}

	// Try each operator (longest first).
	for _, ot := range operatorTable {
		idx := strings.Index(expr, ot.str)
		if idx < 0 {
			continue
		}
		key := expr[:idx]
		value := expr[idx+len(ot.str):]

		if key == "" {
			return WhereCond{}, fmt.Errorf("where: empty key in %q", expr)
		}
		if value == "" {
			return WhereCond{}, fmt.Errorf("where: empty value in %q", expr)
		}

		if ot.op == WhereOpLike {
			return WhereCond{Key: key, Op: ot.op, Value: value}, nil
		}

		// Relative date tokens (today, today-90d, ...) are sugar for an absolute
		// date literal on the right-hand side. We expand the token and compare as
		// a date. The comparison runs against the stored sort_value with a
		// value_type="date" guard, so the left-hand key must be declared `date`
		// in meta.types — an undeclared key is stored with value_type="string"
		// and a string-normalized sort_value, which the guard (correctly) skips.
		typeInfo, _ := metaCfg.LookupType(key)
		if expanded, ok := expandRelativeDate(value, time.Now()); ok {
			typeInfo = MetaTypeInfo{Name: MetaTypeDate}
			value = expanded
		}

		// Normalize value using metaCfg.
		sortValue, warning := NormalizeSortValue(value, typeInfo)
		if warning != "" {
			return WhereCond{}, fmt.Errorf("where: %s (key=%s, value=%s)", warning, key, value)
		}
		return WhereCond{Key: key, Op: ot.op, Value: sortValue, valueType: string(typeInfo.Name)}, nil
	}

	// No operator found → EXISTS.
	key := expr
	if key == "" {
		return WhereCond{}, fmt.Errorf("where: empty key in %q", expr)
	}
	return WhereCond{Key: key, Op: WhereOpExists, Value: ""}, nil
}

func parseNotExistsWhere(expr string) (string, bool) {
	const suffix = " NOT EXISTS"
	if strings.EqualFold(expr, strings.TrimSpace(suffix)) {
		return "", true
	}
	if len(expr) < len(suffix) || !strings.EqualFold(expr[len(expr)-len(suffix):], suffix) {
		return "", false
	}
	key := strings.TrimSpace(expr[:len(expr)-len(suffix)])
	return key, true
}

// MetaFilterSQL generates a SQL fragment to filter nodes by meta conditions.
// alias is the node ID column (e.g. "n.id").
// Returns ("", nil) for nil receiver or empty conditions.
func (wc *WhereClause) MetaFilterSQL(alias string) (string, []any) {
	if wc == nil || (len(wc.Conditions) == 0 && len(wc.AndGroups) == 0) {
		return "", nil
	}

	var subqueries []string
	var allArgs []any

	// Process single conditions (same-key = OR, cross-key = INTERSECT).
	if len(wc.Conditions) > 0 {
		type keyGroup struct {
			key   string
			conds []WhereCond
		}
		orderMap := make(map[string]int)
		var groups []keyGroup
		for _, c := range wc.Conditions {
			idx, ok := orderMap[c.Key]
			if !ok {
				idx = len(groups)
				orderMap[c.Key] = idx
				groups = append(groups, keyGroup{key: c.Key})
			}
			groups[idx].conds = append(groups[idx].conds, c)
		}
		for _, g := range groups {
			sq, args := buildKeyGroupSQL(g.key, g.conds)
			subqueries = append(subqueries, sq)
			allArgs = append(allArgs, args...)
		}
	}

	// Process AND groups: each condition → own subquery, INTERSECT within group.
	for _, group := range wc.AndGroups {
		for _, c := range group {
			sq, args := buildKeyGroupSQL(c.Key, []WhereCond{c})
			subqueries = append(subqueries, sq)
			allArgs = append(allArgs, args...)
		}
	}

	if len(subqueries) == 1 {
		return fmt.Sprintf(" AND %s IN (%s)", alias, subqueries[0]), allArgs
	}
	return fmt.Sprintf(" AND %s IN (%s)", alias, strings.Join(subqueries, " INTERSECT ")), allArgs
}

func buildKeyGroupSQL(key string, conds []WhereCond) (string, []any) {
	// Check if EXISTS is present — it absorbs all other present-key conditions.
	hasExists := false
	hasNotExists := false
	var presentConds []WhereCond
	for _, c := range conds {
		switch c.Op {
		case WhereOpExists:
			hasExists = true
		case WhereOpNotExists:
			hasNotExists = true
		default:
			presentConds = append(presentConds, c)
		}
	}
	if hasExists && hasNotExists {
		return "SELECT n2.id FROM nodes n2 WHERE n2.type = 'note' AND n2.exists_flag = 1", nil
	}
	if hasExists {
		return "SELECT m.node_id FROM meta m WHERE m.key = ?", []any{key}
	}

	var unionSQL []string
	var unionArgs []any
	if hasNotExists {
		unionSQL = append(unionSQL, "SELECT n2.id FROM nodes n2 WHERE n2.type = 'note' AND n2.exists_flag = 1 AND NOT EXISTS (SELECT 1 FROM meta m WHERE m.node_id = n2.id AND m.key = ?)")
		unionArgs = append(unionArgs, key)
	}
	if len(presentConds) == 0 {
		return strings.Join(unionSQL, " UNION "), unionArgs
	}

	// Separate != from other conditions.
	var neqs []WhereCond
	var others []WhereCond
	for _, c := range presentConds {
		if c.Op == WhereOpNeq {
			neqs = append(neqs, c)
		} else {
			others = append(others, c)
		}
	}

	// If only != conditions: "key exists AND not matching any excluded value".
	if len(others) == 0 {
		sql, args := buildNeqSQL(key, neqs)
		unionSQL = append(unionSQL, sql)
		unionArgs = append(unionArgs, args...)
		return strings.Join(unionSQL, " UNION "), unionArgs
	}

	// Build OR conditions for positive matches.
	sql, args := buildPositiveSQL(key, others)

	// If there are also != conditions, INTERSECT with neq filter.
	if len(neqs) > 0 {
		neqSQL, neqArgs := buildNeqSQL(key, neqs)
		sql = sql + " INTERSECT " + neqSQL
		args = append(args, neqArgs...)
	}

	unionSQL = append(unionSQL, sql)
	unionArgs = append(unionArgs, args...)
	return strings.Join(unionSQL, " UNION "), unionArgs
}

func buildNeqSQL(key string, neqs []WhereCond) (string, []any) {
	// "key exists AND node not in (nodes with excluded values)"
	// SELECT m.node_id FROM meta m WHERE m.key = ?
	//   AND m.node_id NOT IN (SELECT m2.node_id FROM meta m2 WHERE m2.key = ? AND (m2.sort_value = ? OR ...))
	var excludeParts []string
	args := []any{key, key}
	for _, c := range neqs {
		excludeParts = append(excludeParts, "m2.sort_value = ?")
		args = append(args, c.Value)
	}
	return fmt.Sprintf(
		"SELECT m.node_id FROM meta m WHERE m.key = ? AND m.node_id NOT IN (SELECT m2.node_id FROM meta m2 WHERE m2.key = ? AND (%s))",
		strings.Join(excludeParts, " OR "),
	), args
}

// comparisonOpSQL maps comparison operators to their SQL representation.
var comparisonOpSQL = map[WhereOp]string{
	WhereOpGt:  ">",
	WhereOpLt:  "<",
	WhereOpGte: ">=",
	WhereOpLte: "<=",
}

func buildPositiveSQL(key string, conds []WhereCond) (string, []any) {
	// SELECT m.node_id FROM meta m WHERE m.key = ? AND (cond1 OR cond2 OR ...)
	args := []any{key}
	var parts []string
	for _, c := range conds {
		switch c.Op {
		case WhereOpEq:
			parts = append(parts, "m.sort_value = ?")
			args = append(args, c.Value)
		case WhereOpLike:
			parts = append(parts, "m.value LIKE ? ESCAPE '\\'")
			args = append(args, c.Value)
		case WhereOpGt, WhereOpLt, WhereOpGte, WhereOpLte:
			parts = append(parts, fmt.Sprintf("(m.sort_value %s ? AND m.value_type = ?)", comparisonOpSQL[c.Op]))
			args = append(args, c.Value, c.valueType)
		}
	}
	return fmt.Sprintf("SELECT m.node_id FROM meta m WHERE m.key = ? AND (%s)", strings.Join(parts, " OR ")), args
}

// expandRelativeDate expands a relative date token into an absolute date string
// (YYYY-MM-DD) evaluated against now's local date. Recognized forms:
//
//	today
//	today-90d / today+1d   (days)
//	today-2w / today+2w    (weeks)
//	today-3m / today+1m    (months)
//	today-1y / today+1y    (years)
//
// Returns (date, true) on a match. Any input that is not a relative date token
// returns ("", false), leaving the caller to treat it as an absolute literal.
func expandRelativeDate(token string, now time.Time) (string, bool) {
	const prefix = "today"
	if token == prefix {
		return now.Format("2006-01-02"), true
	}
	if !strings.HasPrefix(token, prefix) {
		return "", false
	}
	rest := token[len(prefix):]
	if len(rest) < 3 { // need at least sign, digit, unit
		return "", false
	}
	sign := rest[0]
	if sign != '+' && sign != '-' {
		return "", false
	}
	unit := rest[len(rest)-1]
	numStr := rest[1 : len(rest)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil || n < 0 {
		return "", false
	}
	if sign == '-' {
		n = -n
	}
	var t time.Time
	switch unit {
	case 'd':
		t = now.AddDate(0, 0, n)
	case 'w':
		t = now.AddDate(0, 0, n*7)
	case 'm':
		t = now.AddDate(0, n, 0)
	case 'y':
		t = now.AddDate(n, 0, 0)
	default:
		return "", false
	}
	return t.Format("2006-01-02"), true
}
