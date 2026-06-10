package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ryotapoi/mdhop/internal/core"
)

var validReachableFieldsCLI = map[string]bool{
	"reachable":   true,
	"unreachable": true,
}

func printReachableJSON(w io.Writer, r *core.ReachableResult, fields []string) error {
	show := fieldSet(fields, validReachableFieldsCLI)
	m := make(map[string]any)
	m["from"] = r.From
	if show["reachable"] {
		if r.Reachable != nil {
			m["reachable"] = r.Reachable
		} else {
			m["reachable"] = []string{}
		}
	}
	if show["unreachable"] {
		if r.Unreachable != nil {
			m["unreachable"] = r.Unreachable
		} else {
			m["unreachable"] = []string{}
		}
	}
	if r.Routes != nil {
		m["routes"] = r.Routes
	}
	return encodeJSON(w, m)
}

func printReachableText(w io.Writer, r *core.ReachableResult, fields []string) error {
	show := fieldSet(fields, validReachableFieldsCLI)
	if show["reachable"] && len(r.Reachable) > 0 {
		fmt.Fprintln(w, "reachable:")
		for _, p := range r.Reachable {
			fmt.Fprintf(w, "- %s\n", p)
		}
	}
	if show["unreachable"] && len(r.Unreachable) > 0 {
		fmt.Fprintln(w, "unreachable:")
		for _, p := range r.Unreachable {
			fmt.Fprintf(w, "- %s\n", p)
		}
	}
	if len(r.Routes) > 0 {
		fmt.Fprintln(w, "routes:")
		targets := make([]string, 0, len(r.Routes))
		for t := range r.Routes {
			targets = append(targets, t)
		}
		sort.Strings(targets)
		for _, t := range targets {
			fmt.Fprintf(w, "- %s: %s\n", t, strings.Join(r.Routes[t], " -> "))
		}
	}
	return nil
}
