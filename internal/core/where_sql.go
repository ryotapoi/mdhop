package core

import (
	"fmt"
	"strings"
)

// MetaFilterSQL generates a SQL fragment to filter nodes by meta conditions.
// alias is the node ID column (e.g. "n.id").
// Returns ("", nil) for nil receiver or empty conditions.
func (wc *WhereClause) MetaFilterSQL(alias string) (string, []any) {
	if wc == nil || (len(wc.Conditions) == 0 && len(wc.AndGroups) == 0 && len(wc.OrGroups) == 0) {
		return "", nil
	}

	var subqueries []string
	var allArgs []any

	// Process single-condition flags: every repeated --where flag is ANDed,
	// regardless of whether keys match. Explicit OR stays available via " || ".
	for _, c := range wc.Conditions {
		sq, args := buildKeyGroupSQL(c.Key, []WhereCond{c})
		subqueries = append(subqueries, sq)
		allArgs = append(allArgs, args...)
	}

	// Process AND groups: each condition → own subquery, INTERSECT within group.
	for _, group := range wc.AndGroups {
		for _, c := range group {
			sq, args := buildKeyGroupSQL(c.Key, []WhereCond{c})
			subqueries = append(subqueries, sq)
			allArgs = append(allArgs, args...)
		}
	}

	// Process OR groups: each condition → own subquery, UNION within group.
	for _, group := range wc.OrGroups {
		sq, args := buildOrGroupSQL(group)
		subqueries = append(subqueries, sq)
		allArgs = append(allArgs, args...)
	}

	if len(subqueries) == 1 {
		return fmt.Sprintf(" AND %s IN (%s)", alias, subqueries[0]), allArgs
	}
	for i, sq := range subqueries {
		subqueries[i] = "SELECT * FROM (" + sq + ")"
	}
	return fmt.Sprintf(" AND %s IN (%s)", alias, strings.Join(subqueries, " INTERSECT ")), allArgs
}

func buildOrGroupSQL(conds []WhereCond) (string, []any) {
	var unionSQL []string
	var args []any
	for _, c := range conds {
		sql, sqlArgs := buildKeyGroupSQL(c.Key, []WhereCond{c})
		unionSQL = append(unionSQL, sql)
		args = append(args, sqlArgs...)
	}
	return strings.Join(unionSQL, " UNION "), args
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
