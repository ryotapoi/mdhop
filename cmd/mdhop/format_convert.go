package main

import (
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type convertJSONOutput struct {
	Rewritten []rewrittenJSON `json:"rewritten"`
}

func printConvertJSON(w io.Writer, r *core.ConvertResult) error {
	out := convertJSONOutput{
		Rewritten: toRewrittenJSON(r.Rewritten),
	}
	if out.Rewritten == nil {
		out.Rewritten = []rewrittenJSON{}
	}
	return encodeJSON(w, out)
}
