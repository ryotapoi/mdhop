package main

import (
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type disambiguateJSONOutput struct {
	Rewritten []rewrittenJSON `json:"rewritten"`
}

func printDisambiguateText(w io.Writer, r *core.DisambiguateResult) {
	printRewrittenText(w, r.Rewritten)
}

func printDisambiguateJSON(w io.Writer, r *core.DisambiguateResult) error {
	out := disambiguateJSONOutput{
		Rewritten: toRewrittenJSON(r.Rewritten),
	}
	if out.Rewritten == nil {
		out.Rewritten = []rewrittenJSON{}
	}
	return encodeJSON(w, out)
}
