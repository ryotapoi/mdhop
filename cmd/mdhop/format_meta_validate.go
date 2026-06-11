package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type metaViolationJSON struct {
	SourcePath string `json:"source_path"`
	Key        string `json:"key"`
	Value      string `json:"value"`
	Reason     string `json:"reason"`
}

type metaValidateJSONOutput struct {
	Violations []metaViolationJSON `json:"violations"`
}

func printMetaValidateJSON(w io.Writer, r *core.MetaValidateResult) error {
	violations := make([]metaViolationJSON, len(r.Violations))
	for i, v := range r.Violations {
		violations[i] = metaViolationJSON{
			SourcePath: v.SourcePath,
			Key:        v.Key,
			Value:      v.Value,
			Reason:     string(v.Reason),
		}
	}
	return encodeJSON(w, metaValidateJSONOutput{Violations: violations})
}

func printMetaValidateText(w io.Writer, r *core.MetaValidateResult) error {
	if len(r.Violations) == 0 {
		return nil
	}
	fmt.Fprintln(w, "violations:")
	for _, v := range r.Violations {
		fmt.Fprintf(w, "- source_path: %s\n", v.SourcePath)
		fmt.Fprintf(w, "  key: %s\n", v.Key)
		fmt.Fprintf(w, "  value: %s\n", v.Value)
		fmt.Fprintf(w, "  reason: %s\n", v.Reason)
	}
	return nil
}
