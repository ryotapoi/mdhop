# diagnose command: basename conflicts and phantom detection

## Touches
- `internal/core/diagnose.go` — new: Diagnose() function, DiagnoseOptions/DiagnoseResult/BasenameConflict structs
- `internal/core/diagnose_test.go` — new: 7 tests (conflicts, phantoms, full, empty, NoDB, fields filter, unknown field)
- `cmd/mdhop/diagnose.go` — new: runDiagnose() CLI handler
- `cmd/mdhop/format.go` — added: diagnose formatters (JSON via map[string]any, YAML-like text)
- `cmd/mdhop/main.go` — updated: routing and usage for diagnose subcommand
- `cmd/mdhop/cli_test.go` — added: 4 CLI tests (format/field validation, text/JSON output)

## Verification
- `go test ./internal/core/ -run TestDiagnose` — 7 tests pass
- `go test ./cmd/mdhop/ -run TestRunDiagnose` — 4 tests pass
- `go test ./...` — all 142 tests pass, no regressions

## Pitfalls
- JSON output uses `map[string]any` instead of struct with `omitempty` to ensure requested fields always appear (even as empty arrays), matching consumer expectations
- basename conflict detection queries `exists_flag=1` to exclude deleted notes; grouping is done in Go (not SQL GROUP BY) to preserve per-group path lists
- Codex review flagged "excluded/parse-failure counts" from requirements doc, but overview.md (source of truth) specifies only basename conflicts + phantoms for diagnose
