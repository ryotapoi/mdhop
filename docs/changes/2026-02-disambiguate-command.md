# disambiguate: rewrite basename links to full paths

## Touches
- `internal/core/disambiguate.go` — new: core `Disambiguate()` function
- `internal/core/disambiguate_test.go` — new: 14 test cases
- `cmd/mdhop/disambiguate.go` — new: CLI entry point
- `cmd/mdhop/main.go` — replaced stub with implementation, added to usage
- `testdata/vault_disambiguate/` — new: test fixture
- `testdata/vault_disambiguate_file_scope/` — new: `--file` scope test fixture
- `docs/test-plan.md` — corrected path link behavior description

## Verification
- `go test ./internal/core/ -run TestDisambiguate -v` — 14/14 PASS
- `go test ./...` — all tests PASS (no regressions)
- `go build -o bin/mdhop ./cmd/mdhop` — builds successfully

## Pitfalls
- `TestDisambiguateMultipleCandidatesNoTarget`: build rejects ambiguous basename links, so the fixture must use path-only links when testing multiple candidates with same basename. Overwrite B.md to use `[[sub/A]]` before adding root `A.md`.
