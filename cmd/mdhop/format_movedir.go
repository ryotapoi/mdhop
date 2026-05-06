package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type movedFileJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type moveDirJSONOutput struct {
	Moved     []movedFileJSON `json:"moved"`
	Rewritten []rewrittenJSON `json:"rewritten"`
}

func printMoveDirText(w io.Writer, r *core.MoveDirResult) {
	if len(r.Moved) > 0 {
		fmt.Fprintln(w, "moved:")
		for _, m := range r.Moved {
			fmt.Fprintf(w, "- from: %s\n", m.From)
			fmt.Fprintf(w, "  to: %s\n", m.To)
		}
	}
	printRewrittenText(w, r.Rewritten)
}

func printMoveDirJSON(w io.Writer, r *core.MoveDirResult) error {
	moved := make([]movedFileJSON, len(r.Moved))
	for i, m := range r.Moved {
		moved[i] = movedFileJSON{From: m.From, To: m.To}
	}
	out := moveDirJSONOutput{
		Moved:     moved,
		Rewritten: toRewrittenJSON(r.Rewritten),
	}
	if out.Moved == nil {
		out.Moved = []movedFileJSON{}
	}
	if out.Rewritten == nil {
		out.Rewritten = []rewrittenJSON{}
	}
	return encodeJSON(w, out)
}
