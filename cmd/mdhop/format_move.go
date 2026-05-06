package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type moveJSONOutput struct {
	From      string          `json:"from"`
	To        string          `json:"to"`
	Rewritten []rewrittenJSON `json:"rewritten"`
}

func printMoveText(w io.Writer, from, to string, r *core.MoveResult) {
	fmt.Fprintf(w, "from: %s\n", from)
	fmt.Fprintf(w, "to: %s\n", to)
	printRewrittenText(w, r.Rewritten)
}

func printMoveJSON(w io.Writer, from, to string, r *core.MoveResult) error {
	out := moveJSONOutput{
		From:      from,
		To:        to,
		Rewritten: toRewrittenJSON(r.Rewritten),
	}
	if out.Rewritten == nil {
		out.Rewritten = []rewrittenJSON{}
	}
	return encodeJSON(w, out)
}
