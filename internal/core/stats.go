package core

// StatsOptions controls which fields to return.
type StatsOptions struct {
	Fields []string // nil/empty = all
}

// Stats field names accepted by StatsOptions.Fields and the stats --fields
// CLI flag.
const (
	FieldStatsNotesTotal    = "notes_total"
	FieldStatsNotesExists   = "notes_exists"
	FieldStatsEdgesTotal    = "edges_total"
	FieldStatsTagsTotal     = "tags_total"
	FieldStatsPhantomsTotal = "phantoms_total"
	FieldStatsAssetsTotal   = "assets_total"
)

// StatsResult contains vault statistics.
type StatsResult struct {
	NotesTotal    int
	NotesExists   int
	EdgesTotal    int
	TagsTotal     int
	PhantomsTotal int
	AssetsTotal   int
}

// Stats returns aggregate statistics for the indexed vault.
func Stats(vaultPath string, opts StatsOptions) (*StatsResult, error) {
	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result := &StatsResult{}

	if isFieldActive(FieldStatsNotesTotal, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type='note'`).Scan(&result.NotesTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive(FieldStatsNotesExists, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type='note' AND exists_flag=1`).Scan(&result.NotesExists); err != nil {
			return nil, err
		}
	}

	if isFieldActive(FieldStatsEdgesTotal, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM edges`).Scan(&result.EdgesTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive(FieldStatsTagsTotal, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type='tag'`).Scan(&result.TagsTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive(FieldStatsPhantomsTotal, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type='phantom'`).Scan(&result.PhantomsTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive(FieldStatsAssetsTotal, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type='asset'`).Scan(&result.AssetsTotal); err != nil {
			return nil, err
		}
	}

	return result, nil
}
