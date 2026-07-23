package main

import (
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type addJSONOutput struct {
	Added     []string        `json:"added"`
	Promoted  []string        `json:"promoted"`
	Rewritten []rewrittenJSON `json:"rewritten"`
}

func printAddText(w io.Writer, r *core.AddResult) {
	printStringListText(w, "added", r.Added)
	printStringListText(w, "promoted", r.Promoted)
	printRewrittenText(w, r.Rewritten)
}

func printAddJSON(w io.Writer, r *core.AddResult) error {
	out := addJSONOutput{
		Added:     emptyIfNil(r.Added),
		Promoted:  emptyIfNil(r.Promoted),
		Rewritten: emptyIfNil(toRewrittenJSON(r.Rewritten)),
	}
	return encodeJSON(w, out)
}
