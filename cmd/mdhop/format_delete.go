package main

import (
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type deleteJSONOutput struct {
	Deleted   []string `json:"deleted"`
	Phantomed []string `json:"phantomed"`
}

func printDeleteText(w io.Writer, r *core.DeleteResult) error {
	output := &textErrorWriter{w: w}
	w = output
	printStringListText(w, "deleted", r.Deleted)
	printStringListText(w, "phantomed", r.Phantomed)
	return output.err
}

func printDeleteJSON(w io.Writer, r *core.DeleteResult) error {
	out := deleteJSONOutput{
		Deleted:   emptyIfNil(r.Deleted),
		Phantomed: emptyIfNil(r.Phantomed),
	}
	return encodeJSON(w, out)
}
