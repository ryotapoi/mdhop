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
	Conditions []WhereCond   // from single-condition --where flags (each flag = AND)
	AndGroups  [][]WhereCond // from && --where flags (each group = all AND)
	OrGroups   [][]WhereCond // from || --where flags (each group = any OR)
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
// conditions must be satisfied (even for the same key). Expressions containing
// " || " are split into OR groups. Mixing both separators in one expression is
// rejected because --where has no precedence or parentheses support.
func ParseWhere(exprs []string, metaCfg MetaConfig) (*WhereClause, error) {
	if len(exprs) == 0 {
		return nil, nil
	}

	var conds []WhereCond
	var andGroups [][]WhereCond
	var orGroups [][]WhereCond
	for _, expr := range exprs {
		hasAnd := strings.Contains(expr, " && ")
		hasOr := strings.Contains(expr, " || ")
		if hasAnd && hasOr {
			return nil, fmt.Errorf("where: cannot mix && and || in one expression %q", expr)
		}

		switch {
		case hasAnd:
			// Multiple conditions joined by && — all AND.
			parts := strings.Split(expr, " && ")
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
		case hasOr:
			// Multiple conditions joined by || — any OR.
			parts := strings.Split(expr, " || ")
			var group []WhereCond
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					return nil, fmt.Errorf("where: empty condition in || expression %q", expr)
				}
				c, err := parseOneWhere(p, metaCfg)
				if err != nil {
					return nil, err
				}
				group = append(group, c)
			}
			orGroups = append(orGroups, group)
		default:
			// Single condition — append as its own ANDed flag.
			c, err := parseOneWhere(expr, metaCfg)
			if err != nil {
				return nil, err
			}
			conds = append(conds, c)
		}
	}
	if len(conds) == 0 && len(andGroups) == 0 && len(orGroups) == 0 {
		return nil, nil
	}
	return &WhereClause{Conditions: conds, AndGroups: andGroups, OrGroups: orGroups}, nil
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
