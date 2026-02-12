# add command: add new files to existing index

## Touches
- `internal/core/add.go` — `Add()` core function, `isBasenameRawLink()` helper
- `internal/core/add_test.go` — 14 test cases
- `cmd/mdhop/add.go` — CLI entry point `runAdd()`
- `cmd/mdhop/main.go` — routing and usage update
- `testdata/vault_add/` — test fixture (A.md, B.md)

## Verification
- `go test ./...` — all 184 tests pass
- `go build -o bin/mdhop ./cmd/mdhop` — builds successfully
- Codex implementation review executed; self-link false positive bug found and fixed

## Pitfalls
- `isBasenameRawLink` must return false for self-links (`[[#Heading]]`, `[text](#heading)`). Initially missed; caught by Codex review. Empty target/url after stripping fragment means self-link, not basename link.
- Phantom promotion (step 13) must run after note insertion (step 12) so that `pathToID` contains the new note ID for edge reassignment.
- Step 8 (existing ambiguity check) needs both pattern A (oldCount==1 → newCount>1) and pattern B (oldCount==0, adding 2+ same-basename files with existing phantom links).
