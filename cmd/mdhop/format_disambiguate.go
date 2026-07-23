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
		Rewritten: emptyIfNil(toRewrittenJSON(r.Rewritten)),
	}
	return encodeJSON(w, out)
}
