package main

import (
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

func printSimplifyText(w io.Writer, r *core.SimplifyResult) {
	printRewrittenText(w, r.Rewritten)
	printSkippedText(w, r.Skipped)
}

func printSimplifyJSON(w io.Writer, r *core.SimplifyResult) error {
	return printRewriteResultJSON(w, r.Rewritten, r.Skipped)
}
