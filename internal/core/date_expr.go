package core

import (
	"strconv"
	"strings"
	"time"
)

// ExpandRelativeDate expands a relative date token into an absolute date string
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
func ExpandRelativeDate(token string, now time.Time) (string, bool) {
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
