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
		Rewritten: emptyIfNil(toRewrittenJSON(r.Rewritten)),
	}
	return encodeJSON(w, out)
}
