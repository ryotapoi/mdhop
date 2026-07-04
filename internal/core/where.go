package core

import (
	"fmt"
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
	Key          string
	CoalesceKeys []string
	Op           WhereOp
	Value        string // normalized sort_value for =, !=, >, <, >=, <=; raw pattern for ~; empty for EXISTS
	valueType    string // value_type for type-guarded comparisons (>, <, >=, <=)
	keyValues    map[string]whereValue
}

type whereValue struct {
	value     string
	valueType string
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
		key, coalesceKeys, err := parseWhereKey(key, expr)
		if err != nil {
			return WhereCond{}, err
		}
		return WhereCond{Key: key, CoalesceKeys: coalesceKeys, Op: WhereOpNotExists, Value: ""}, nil
	}

	// Try each operator (longest first).
	for _, ot := range operatorTable {
		idx := strings.Index(expr, ot.str)
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(expr[:idx])
		rawValue := expr[idx+len(ot.str):]

		key, coalesceKeys, err := parseWhereKey(key, expr)
		if err != nil {
			return WhereCond{}, err
		}
		if rawValue == "" {
			return WhereCond{}, fmt.Errorf("where: empty value in %q", expr)
		}

		if ot.op == WhereOpLike {
			// LIKE patterns may have meaningful leading/trailing whitespace —
			// preserve the raw value as-is (only the key gets trimmed, to
			// support "coalesce(a, b) ~pattern" spacing).
			return WhereCond{Key: key, CoalesceKeys: coalesceKeys, Op: ot.op, Value: rawValue}, nil
		}

		value := strings.TrimSpace(rawValue)
		if value == "" {
			return WhereCond{}, fmt.Errorf("where: empty value in %q", expr)
		}

		sortValue, valueType, keyValues, err := normalizeWhereValue(key, coalesceKeys, value, metaCfg)
		if err != nil {
			return WhereCond{}, err
		}
		return WhereCond{Key: key, CoalesceKeys: coalesceKeys, Op: ot.op, Value: sortValue, valueType: valueType, keyValues: keyValues}, nil
	}

	// No operator found → EXISTS.
	key, coalesceKeys, err := parseWhereKey(expr, expr)
	if err != nil {
		return WhereCond{}, err
	}
	return WhereCond{Key: key, CoalesceKeys: coalesceKeys, Op: WhereOpExists, Value: ""}, nil
}

func normalizeWhereValue(key string, coalesceKeys []string, value string, metaCfg MetaConfig) (string, string, map[string]whereValue, error) {
	// Relative date tokens (today, today-90d, ...) are sugar for an absolute
	// date literal on the right-hand side. Range comparisons also carry a
	// value_type="date" guard, so undeclared keys keep the existing single-key
	// behavior: their string-stored values are skipped rather than coerced.
	expandedValue := value
	forceDate := false
	if expanded, ok := ExpandRelativeDate(value, time.Now()); ok {
		expandedValue = expanded
		forceDate = true
	}

	if len(coalesceKeys) == 0 {
		typeInfo, _ := metaCfg.LookupType(key)
		if forceDate {
			typeInfo = MetaTypeInfo{Name: MetaTypeDate}
		}
		sortValue, warning := NormalizeSortValue(expandedValue, typeInfo)
		if warning != "" {
			return "", "", nil, fmt.Errorf("where: %s (key=%s, value=%s)", warning, key, expandedValue)
		}
		return sortValue, string(typeInfo.Name), nil, nil
	}

	keyValues := make(map[string]whereValue, len(coalesceKeys))
	normalizedByType := make(map[string]whereValue)
	for _, k := range coalesceKeys {
		typeInfo, _ := metaCfg.LookupType(k)
		if forceDate {
			typeInfo = MetaTypeInfo{Name: MetaTypeDate}
		}
		cacheKey := metaTypeCacheKey(typeInfo)
		kv, ok := normalizedByType[cacheKey]
		if !ok {
			sortValue, warning := NormalizeSortValue(expandedValue, typeInfo)
			if warning != "" {
				return "", "", nil, fmt.Errorf("where: %s (key=%s, value=%s)", warning, k, expandedValue)
			}
			kv = whereValue{value: sortValue, valueType: string(typeInfo.Name)}
			normalizedByType[cacheKey] = kv
		}
		keyValues[k] = kv
	}
	first := keyValues[coalesceKeys[0]]
	return first.value, first.valueType, keyValues, nil
}

func metaTypeCacheKey(typeInfo MetaTypeInfo) string {
	return string(typeInfo.Name) + "\x00" + strings.Join(typeInfo.OrderedValues, "\x00")
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

func parseWhereKey(key, expr string) (string, []string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", nil, fmt.Errorf("where: empty key in %q", expr)
	}
	lowerKey := strings.ToLower(key)
	if !strings.HasPrefix(lowerKey, "coalesce(") {
		return key, nil, nil
	}
	if !strings.HasSuffix(key, ")") {
		return "", nil, fmt.Errorf("where: unclosed coalesce in %q", expr)
	}
	inner := strings.TrimSpace(key[len("coalesce(") : len(key)-1])
	if inner == "" {
		return "", nil, fmt.Errorf("where: empty coalesce key list in %q", expr)
	}
	rawKeys := strings.Split(inner, ",")
	keys := make([]string, 0, len(rawKeys))
	seen := make(map[string]struct{}, len(rawKeys))
	for _, raw := range rawKeys {
		k := strings.TrimSpace(raw)
		if k == "" {
			return "", nil, fmt.Errorf("where: empty coalesce key in %q", expr)
		}
		if _, ok := seen[k]; ok {
			return "", nil, fmt.Errorf("where: coalesce keys must be unique in %q", expr)
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	if len(keys) < 2 {
		return "", nil, fmt.Errorf("where: coalesce requires at least 2 keys in %q", expr)
	}
	return "coalesce(" + strings.Join(keys, ", ") + ")", keys, nil
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
	keys := []string{key}
	if len(conds) > 0 && len(conds[0].CoalesceKeys) > 0 {
		keys = conds[0].CoalesceKeys
	}

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
		return buildExistsSQL(keys)
	}

	var unionSQL []string
	var unionArgs []any
	if hasNotExists {
		sql, args := buildNotExistsSQL(keys)
		unionSQL = append(unionSQL, sql)
		unionArgs = append(unionArgs, args...)
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
		sql, args := buildNeqSQL(keys, neqs)
		unionSQL = append(unionSQL, sql)
		unionArgs = append(unionArgs, args...)
		return strings.Join(unionSQL, " UNION "), unionArgs
	}

	// Build OR conditions for positive matches.
	sql, args := buildPositiveSQL(keys, others)

	// If there are also != conditions, INTERSECT with neq filter.
	if len(neqs) > 0 {
		neqSQL, neqArgs := buildNeqSQL(keys, neqs)
		sql = "SELECT * FROM (" + sql + ") INTERSECT SELECT * FROM (" + neqSQL + ")"
		args = append(args, neqArgs...)
	}

	unionSQL = append(unionSQL, sql)
	unionArgs = append(unionArgs, args...)
	return strings.Join(unionSQL, " UNION "), unionArgs
}

func buildExistsSQL(keys []string) (string, []any) {
	if len(keys) == 1 {
		return "SELECT m.node_id FROM meta m WHERE m.key = ?", []any{keys[0]}
	}
	return fmt.Sprintf("SELECT m.node_id FROM meta m WHERE m.key IN (%s)", placeholders(len(keys))), anySlice(keys)
}

func buildNotExistsSQL(keys []string) (string, []any) {
	if len(keys) == 1 {
		return "SELECT n2.id FROM nodes n2 WHERE n2.type = 'note' AND n2.exists_flag = 1 AND NOT EXISTS (SELECT 1 FROM meta m WHERE m.node_id = n2.id AND m.key = ?)", []any{keys[0]}
	}
	return fmt.Sprintf(
		"SELECT n2.id FROM nodes n2 WHERE n2.type = 'note' AND n2.exists_flag = 1 AND NOT EXISTS (SELECT 1 FROM meta m WHERE m.node_id = n2.id AND m.key IN (%s))",
		placeholders(len(keys)),
	), anySlice(keys)
}

func buildNeqSQL(keys []string, neqs []WhereCond) (string, []any) {
	if len(keys) > 1 {
		return buildCoalesceNeqSQL(keys, neqs)
	}
	key := keys[0]
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

func buildCoalesceNeqSQL(keys []string, neqs []WhereCond) (string, []any) {
	var unionSQL []string
	var args []any
	for i, key := range keys {
		var excludeParts []string
		branchArgs := []any{key, key}
		for _, c := range neqs {
			kv := c.whereValueForKey(key)
			excludeParts = append(excludeParts, "m2.sort_value = ?")
			branchArgs = append(branchArgs, kv.value)
		}
		sql := fmt.Sprintf(
			"SELECT m.node_id FROM meta m WHERE m.key = ? AND m.node_id NOT IN (SELECT m2.node_id FROM meta m2 WHERE m2.key = ? AND (%s))",
			strings.Join(excludeParts, " OR "),
		)
		if i > 0 {
			sql += fmt.Sprintf(" AND NOT EXISTS (SELECT 1 FROM meta mh WHERE mh.node_id = m.node_id AND mh.key IN (%s))", placeholders(i))
			branchArgs = append(branchArgs, anySlice(keys[:i])...)
		}
		unionSQL = append(unionSQL, sql)
		args = append(args, branchArgs...)
	}
	return strings.Join(unionSQL, " UNION "), args
}

// comparisonOpSQL maps comparison operators to their SQL representation.
var comparisonOpSQL = map[WhereOp]string{
	WhereOpGt:  ">",
	WhereOpLt:  "<",
	WhereOpGte: ">=",
	WhereOpLte: "<=",
}

func buildPositiveSQL(keys []string, conds []WhereCond) (string, []any) {
	if len(keys) > 1 {
		return buildCoalescePositiveSQL(keys, conds)
	}
	key := keys[0]
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

func buildCoalescePositiveSQL(keys []string, conds []WhereCond) (string, []any) {
	var unionSQL []string
	var args []any
	for i, key := range keys {
		branchArgs := []any{key}
		var parts []string
		for _, c := range conds {
			kv := c.whereValueForKey(key)
			switch c.Op {
			case WhereOpEq:
				parts = append(parts, "m.sort_value = ?")
				branchArgs = append(branchArgs, kv.value)
			case WhereOpLike:
				parts = append(parts, "m.value LIKE ? ESCAPE '\\'")
				branchArgs = append(branchArgs, c.Value)
			case WhereOpGt, WhereOpLt, WhereOpGte, WhereOpLte:
				parts = append(parts, fmt.Sprintf("(m.sort_value %s ? AND m.value_type = ?)", comparisonOpSQL[c.Op]))
				branchArgs = append(branchArgs, kv.value, kv.valueType)
			}
		}
		sql := fmt.Sprintf("SELECT m.node_id FROM meta m WHERE m.key = ? AND (%s)", strings.Join(parts, " OR "))
		if i > 0 {
			sql += fmt.Sprintf(" AND NOT EXISTS (SELECT 1 FROM meta mh WHERE mh.node_id = m.node_id AND mh.key IN (%s))", placeholders(i))
			branchArgs = append(branchArgs, anySlice(keys[:i])...)
		}
		unionSQL = append(unionSQL, sql)
		args = append(args, branchArgs...)
	}
	return strings.Join(unionSQL, " UNION "), args
}

func (c WhereCond) whereValueForKey(key string) whereValue {
	if c.keyValues != nil {
		if kv, ok := c.keyValues[key]; ok {
			return kv
		}
	}
	return whereValue{value: c.Value, valueType: c.valueType}
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func anySlice(values []string) []any {
	args := make([]any, len(values))
	for i, v := range values {
		args[i] = v
	}
	return args
}
