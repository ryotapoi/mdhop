package core

import (
	"fmt"
	"strings"
)

// WhereOp represents a comparison operator for --where conditions.
type WhereOp int

const (
	WhereOpEq     WhereOp = iota // =
	WhereOpNeq                   // !=
	WhereOpLike                  // ~
	WhereOpGt                    // >
	WhereOpLt                    // <
	WhereOpGte                   // >=
	WhereOpLte                   // <=
	WhereOpExists                // key only (no operator)
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
	Conditions []WhereCond
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
func ParseWhere(exprs []string, metaCfg MetaConfig) (*WhereClause, error) {
	if len(exprs) == 0 {
		return nil, nil
	}

	var conds []WhereCond
	for _, expr := range exprs {
		c, err := parseOneWhere(expr, metaCfg)
		if err != nil {
			return nil, err
		}
		conds = append(conds, c)
	}
	return &WhereClause{Conditions: conds}, nil
}

func parseOneWhere(expr string, metaCfg MetaConfig) (WhereCond, error) {
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

		// Normalize value using metaCfg.
		typeInfo, _ := metaCfg.LookupType(key)
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

// MetaFilterSQL generates a SQL fragment to filter nodes by meta conditions.
// alias is the node ID column (e.g. "n.id").
// Returns ("", nil) for nil receiver or empty conditions.
func (wc *WhereClause) MetaFilterSQL(alias string) (string, []any) {
	if wc == nil || len(wc.Conditions) == 0 {
		return "", nil
	}

	// Group conditions by key (preserving order of first occurrence).
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

	var subqueries []string
	var allArgs []any

	for _, g := range groups {
		sq, args := buildKeyGroupSQL(g.key, g.conds)
		subqueries = append(subqueries, sq)
		allArgs = append(allArgs, args...)
	}

	if len(subqueries) == 1 {
		return fmt.Sprintf(" AND %s IN (%s)", alias, subqueries[0]), allArgs
	}
	return fmt.Sprintf(" AND %s IN (%s)", alias, strings.Join(subqueries, " INTERSECT ")), allArgs
}

func buildKeyGroupSQL(key string, conds []WhereCond) (string, []any) {
	// Check if EXISTS is present — it absorbs all other conditions.
	for _, c := range conds {
		if c.Op == WhereOpExists {
			return "SELECT m.node_id FROM meta m WHERE m.key = ?", []any{key}
		}
	}

	// Separate != from other conditions.
	var neqs []WhereCond
	var others []WhereCond
	for _, c := range conds {
		if c.Op == WhereOpNeq {
			neqs = append(neqs, c)
		} else {
			others = append(others, c)
		}
	}

	// If only != conditions: "key exists AND not matching any excluded value".
	if len(others) == 0 {
		return buildNeqSQL(key, neqs)
	}

	// Build OR conditions for positive matches.
	sql, args := buildPositiveSQL(key, others)

	// If there are also != conditions, INTERSECT with neq filter.
	if len(neqs) > 0 {
		neqSQL, neqArgs := buildNeqSQL(key, neqs)
		sql = sql + " INTERSECT " + neqSQL
		args = append(args, neqArgs...)
	}

	return sql, args
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
