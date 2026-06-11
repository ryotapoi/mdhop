package main

import (
	"fmt"
	"io"

	"github.com/ryotapoi/mdhop/internal/core"
)

type metaIssueJSON struct {
	SourcePath string `json:"source_path"`
	Key        string `json:"key"`
	Value      string `json:"value"`
	Reason     string `json:"reason"`
}

type metaCheckJSONOutput struct {
	Issues []metaIssueJSON `json:"issues"`
}

func printMetaCheckJSON(w io.Writer, r *core.MetaCheckResult) error {
	issues := make([]metaIssueJSON, len(r.Issues))
	for i, is := range r.Issues {
		issues[i] = metaIssueJSON{
			SourcePath: is.SourcePath,
			Key:        is.Key,
			Value:      is.Value,
			Reason:     string(is.Reason),
		}
	}
	return encodeJSON(w, metaCheckJSONOutput{Issues: issues})
}

func printMetaCheckText(w io.Writer, r *core.MetaCheckResult) error {
	if len(r.Issues) == 0 {
		return nil
	}
	fmt.Fprintln(w, "issues:")
	for _, is := range r.Issues {
		fmt.Fprintf(w, "- source_path: %s\n", is.SourcePath)
		fmt.Fprintf(w, "  key: %s\n", is.Key)
		fmt.Fprintf(w, "  value: %s\n", is.Value)
		fmt.Fprintf(w, "  reason: %s\n", is.Reason)
	}
	return nil
}
