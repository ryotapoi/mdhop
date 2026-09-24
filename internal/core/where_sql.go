package core

import (
	"fmt"
	"strings"
)

// MetaFilterSQL generates a SQL fragment to filter nodes by meta conditions.
// alias is the node ID column (e.g. "n.id").
// Returns ("", nil) for nil receiver or empty conditions.
func (wc *WhereClause) MetaFilterSQL(alias string) (string, []any) {
	if wc == nil || (len(wc.Conditions) == 0 && len(wc.OrGroups) == 0) {
		return "", nil
	}

	var subqueries []string
	var allArgs []any
	for _, c := range wc.Conditions {
		sq, args := buildConditionSQL(c)
		subqueries = append(subqueries, sq)
		allArgs = append(allArgs, args...)
	}
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
		sql, sqlArgs := buildConditionSQL(c)
		unionSQL = append(unionSQL, sql)
		args = append(args, sqlArgs...)
	}
	return strings.Join(unionSQL, " UNION "), args
}

func buildConditionSQL(c WhereCond) (string, []any) {
	keys := []string{c.Key}
	if len(c.CoalesceKeys) > 0 {
		keys = c.CoalesceKeys
	}

	switch c.Op {
	case WhereOpExists:
		return buildExistsSQL(keys)
	case WhereOpNotExists:
		return buildNotExistsSQL(keys)
	case WhereOpNeq:
		return buildNeqSQL(keys, c)
	default:
		return buildPositiveSQL(keys, c)
	}
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

func buildNeqSQL(keys []string, cond WhereCond) (string, []any) {
	var unionSQL []string
	var args []any
	for i, key := range keys {
		branchArgs := []any{key, key, cond.whereValueForKey(key).value}
		sql := "SELECT m.node_id FROM meta m WHERE m.key = ? AND m.node_id NOT IN (SELECT m2.node_id FROM meta m2 WHERE m2.key = ? AND (m2.sort_value = ?))"
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

func buildPositiveSQL(keys []string, cond WhereCond) (string, []any) {
	var unionSQL []string
	var args []any
	for i, key := range keys {
		branchArgs := []any{key}
		value := cond.whereValueForKey(key)
		var part string
		switch cond.Op {
		case WhereOpEq:
			part = "m.sort_value = ?"
			branchArgs = append(branchArgs, value.value)
		case WhereOpLike:
			part = "m.value LIKE ? ESCAPE '\\'"
			branchArgs = append(branchArgs, cond.Value)
		case WhereOpGt, WhereOpLt, WhereOpGte, WhereOpLte:
			part = fmt.Sprintf("(m.sort_value %s ? AND m.value_type = ?)", comparisonOpSQL[cond.Op])
			branchArgs = append(branchArgs, value.value, value.valueType)
		}
		sql := fmt.Sprintf("SELECT m.node_id FROM meta m WHERE m.key = ? AND (%s)", part)
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
