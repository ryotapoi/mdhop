# Delete Command: remove files from index with phantom conversion

## Touches
- `internal/core/delete.go` — core Delete logic (new)
- `internal/core/delete_test.go` — 10 test cases (new)
- `cmd/mdhop/delete.go` — CLI handler with multiString flag (new)
- `cmd/mdhop/main.go` — routing and usage update
- `cmd/mdhop/cli_test.go` — 2 CLI tests added
- `testdata/vault_delete/` — test fixture (A.md, B.md, C.md)

## Verification
- `go test ./internal/core/ -run TestDelete` — 10 tests pass
- `go test ./cmd/mdhop/ -run TestRunDelete` — 2 tests pass
- `go test ./...` — all 153 tests pass
- `go build -o bin/mdhop ./cmd/mdhop` — binary builds successfully

## Pitfalls
- Tag regex `[A-Za-z0-9_][A-Za-z0-9_/]*` does not match hyphens; test fixture initially used `#only-b` which was not recognized as a tag. Changed to `#only_b`.
- Existing phantom edge reassignment requires testing with manually inserted phantom nodes, since `Build` resolves phantoms to notes when the file exists.
