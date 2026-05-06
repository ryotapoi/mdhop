package main

import (
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type deleteJSONOutput struct {
	Deleted   []string `json:"deleted"`
	Phantomed []string `json:"phantomed"`
}

func printDeleteText(w io.Writer, r *core.DeleteResult) {
	printStringListText(w, "deleted", r.Deleted)
	printStringListText(w, "phantomed", r.Phantomed)
}

func printDeleteJSON(w io.Writer, r *core.DeleteResult) error {
	out := deleteJSONOutput{
		Deleted:   r.Deleted,
		Phantomed: r.Phantomed,
	}
	if out.Deleted == nil {
		out.Deleted = []string{}
	}
	if out.Phantomed == nil {
		out.Phantomed = []string{}
	}
	return encodeJSON(w, out)
}
