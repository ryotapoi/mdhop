package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type setJSONOutput struct {
	File    string `json:"file"`
	Key     string `json:"key"`
	Value   string `json:"value"`
	Created bool   `json:"created"`
}

func printSetText(w io.Writer, r *core.SetResult) {
	fmt.Fprintf(w, "set: %s %s=%s\n", r.File, r.Key, r.Value)
}

func printSetJSON(w io.Writer, r *core.SetResult) error {
	out := setJSONOutput{
		File:    r.File,
		Key:     r.Key,
		Value:   r.Value,
		Created: r.Created,
	}
	return encodeJSON(w, out)
}
