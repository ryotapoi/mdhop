package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

func printStatusJSON(w io.Writer, r *core.StatusResult) error {
	return encodeJSON(w, struct {
		Untracked []string `json:"untracked"`
		Modified  []string `json:"modified"`
		Deleted   []string `json:"deleted"`
	}{
		Untracked: emptyIfNil(r.Untracked),
		Modified:  emptyIfNil(r.Modified),
		Deleted:   emptyIfNil(r.Deleted),
	})
}

func printStatusText(w io.Writer, r *core.StatusResult) error {
	output := &textErrorWriter{w: w}
	for _, group := range []struct {
		name  string
		paths []string
	}{
		{"untracked", r.Untracked},
		{"modified", r.Modified},
		{"deleted", r.Deleted},
	} {
		if len(group.paths) == 0 {
			continue
		}
		fmt.Fprintf(output, "%s:\n", group.name)
		for _, path := range group.paths {
			fmt.Fprintf(output, "- %s\n", path)
		}
	}
	return output.err
}
