# update command: incremental index update for specified files

## Touches
- `internal/core/db.go` — added `dbExecer` interface, changed upsert/query function signatures from `*sql.DB` to `dbExecer`
- `internal/core/build.go` — changed `resolveLink`, `resolvePathTarget` signatures to `dbExecer`, removed unused `database/sql` import
- `internal/core/update.go` — NEW: `Update()`, `UpdateOptions`, `UpdateResult`, `buildMapsFromDB()`
- `internal/core/update_test.go` — NEW: 17 test cases
- `testdata/vault_update/` — NEW: A.md, B.md, C.md fixture
- `cmd/mdhop/update.go` — NEW: CLI wrapper `runUpdate()`
- `cmd/mdhop/main.go` — added "update" routing and usage line
- `docs/architecture/02-requirements.md` — updated 2.2 delete behavior spec
- `docs/external/overview.md` — added update delete behavior section
- `docs/test-plan.md` — updated update test section

## Verification
- `go test ./...` — 170 tests pass (153 → 170, +17 update tests)
- `go build -o bin/mdhop ./cmd/mdhop` — binary builds successfully
- Codex implementation review: 0 MUST, 1 SHOULD (fixed), 1 NIT (fixed)

## Pitfalls
- `dbExecer` interface change: `build.go` no longer directly imports `database/sql` since `resolveLink`/`resolvePathTarget` use the interface type. The unused import must be removed or build fails.
- Simultaneous update+delete: when file A references file B, and both are updated with B deleted from disk, Phase A's `resolveLink` creates a phantom B (since B is removed from maps). Phase B then finds note B has 0 incoming edges (edges point to phantom B, not note B) → note B is completely deleted, not phantom-converted. This is correct behavior but non-obvious.
- `basenameCounts` adjustment must be guarded by `pathToID` existence check to avoid decrementing for files not in the maps (e.g., `exists_flag=0` notes from inconsistent DB state).
