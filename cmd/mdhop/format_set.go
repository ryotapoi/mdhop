package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type setJSONOutput struct {
	File    string `json:"file"`
	Key     string `json:"key"`
	Value   any    `json:"value"`
	Created bool   `json:"created"`
}

func printSetText(w io.Writer, r *core.SetResult) error {
	output := &textErrorWriter{w: w}
	w = output
	value := r.Value
	if r.List != nil {
		encoded, err := json.Marshal(r.List)
		if err != nil {
			return err
		}
		value = string(encoded)
	}
	fmt.Fprintf(w, "set: %s %s=%s\n", r.File, r.Key, value)
	return output.err
}

func printSetJSON(w io.Writer, r *core.SetResult) error {
	value := any(r.Value)
	if r.List != nil {
		value = r.List
	}
	out := setJSONOutput{
		File:    r.File,
		Key:     r.Key,
		Value:   value,
		Created: r.Created,
	}
	return encodeJSON(w, out)
}
