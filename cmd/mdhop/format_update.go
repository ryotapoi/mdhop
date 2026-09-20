package main

import (
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type updateJSONOutput struct {
	Updated   []string `json:"updated"`
	Deleted   []string `json:"deleted"`
	Phantomed []string `json:"phantomed"`
}

func printUpdateText(w io.Writer, r *core.UpdateResult) error {
	output := &textErrorWriter{w: w}
	w = output
	printStringListText(w, "updated", r.Updated)
	printStringListText(w, "deleted", r.Deleted)
	printStringListText(w, "phantomed", r.Phantomed)
	return output.err
}

func printUpdateJSON(w io.Writer, r *core.UpdateResult) error {
	out := updateJSONOutput{
		Updated:   emptyIfNil(r.Updated),
		Deleted:   emptyIfNil(r.Deleted),
		Phantomed: emptyIfNil(r.Phantomed),
	}
	return encodeJSON(w, out)
}
