package main

import (
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

func printRepairText(w io.Writer, r *core.RepairResult) {
	printRewrittenText(w, r.Rewritten)
	printSkippedText(w, r.Skipped)
}

func printRepairJSON(w io.Writer, r *core.RepairResult) error {
	return printRewriteResultJSON(w, r.Rewritten, r.Skipped)
}
