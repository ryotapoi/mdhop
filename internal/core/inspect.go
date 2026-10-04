package core

import "fmt"

// InspectOptions selects indexed attributes and an optional body preview.
// Nil Fields selects tags and meta; nil IncludeHead skips reading the file.
type InspectOptions struct {
	Fields      []string
	IncludeHead *int
}

// InspectResult keeps unselected fields nil and selected empty fields non-nil.
type InspectResult struct {
	Entry NodeInfo
	Tags  []string
	Meta  map[string][]string
	Head  []string
}

// Inspect returns information for one indexed note without relation traversal.
func Inspect(vaultPath, file string, opts InspectOptions) (*InspectResult, error) {
	if file == "" {
		return nil, fmt.Errorf("--file is required")
	}
	fields := opts.Fields
	if fields == nil {
		fields = []string{"tags", "meta"}
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("fields must not be empty")
	}
	seen := map[string]bool{}
	for _, field := range fields {
		if field != "tags" && field != "meta" {
			return nil, fmt.Errorf("invalid inspect field: %q", field)
		}
		if seen[field] {
			return nil, fmt.Errorf("duplicate inspect field: %q", field)
		}
		seen[field] = true
	}
	if opts.IncludeHead != nil && *opts.IncludeHead <= 0 {
		return nil, fmt.Errorf("include-head must be positive")
	}
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	id, info, err := findEntryByFile(db, file)
	if err != nil {
		return nil, err
	}
	if info.Type != NodeTypeNote {
		return nil, fmt.Errorf("inspect requires an indexed note: %s", file)
	}
	result := &InspectResult{Entry: info}
	if seen["tags"] {
		result.Tags, err = queryTags(db, id, nil)
		if err != nil {
			return nil, err
		}
		if result.Tags == nil {
			result.Tags = []string{}
		}
	}
	if seen["meta"] {
		rows, err := queryMetaByNode(db, id)
		if err != nil {
			return nil, err
		}
		result.Meta = map[string][]string{}
		for _, row := range rows {
			result.Meta[row.Key] = append(result.Meta[row.Key], row.Value)
		}
	}
	if opts.IncludeHead != nil {
		source, err := queryHeadSource(db, id)
		if err != nil {
			return nil, err
		}
		result.Head, err = readHead(vaultPath, source, *opts.IncludeHead)
		if err != nil {
			return nil, err
		}
		if result.Head == nil {
			result.Head = []string{}
		}
	}
	return result, nil
}
