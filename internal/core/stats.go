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
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type=?`, NodeTypeNote).Scan(&result.NotesTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive(FieldStatsNotesExists, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type=? AND exists_flag=1`, NodeTypeNote).Scan(&result.NotesExists); err != nil {
			return nil, err
		}
	}

	if isFieldActive(FieldStatsEdgesTotal, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM edges`).Scan(&result.EdgesTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive(FieldStatsTagsTotal, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type=?`, NodeTypeTag).Scan(&result.TagsTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive(FieldStatsPhantomsTotal, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type=?`, NodeTypePhantom).Scan(&result.PhantomsTotal); err != nil {
			return nil, err
		}
	}

	if isFieldActive(FieldStatsAssetsTotal, opts.Fields) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE type=?`, NodeTypeAsset).Scan(&result.AssetsTotal); err != nil {
			return nil, err
		}
	}

	return result, nil
}
