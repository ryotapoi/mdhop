# auto-disambiguate: rewrite existing links on basename collision

## Touches
- `internal/core/add.go` — auto-disambiguate logic, rewrite helpers, rollback support
- `internal/core/add_test.go` — 12 new tests, 1 deleted (NotImplemented)
- `testdata/vault_add_disambiguate/` — new fixture (A.md with 5 link types, sub/B.md)
- `testdata/vault_add_disambiguate_root/` — new fixture (root target, multiple source dirs)

## Verification
- `go test ./internal/core/ -run TestAddAutoDisambiguate -v` — 12 tests pass
- `go test ./internal/core/ -run TestAdd -v` — all existing add tests pass
- `go test ./...` — full suite passes
- Codex impl-review x2: all MUST resolved, remaining SHOULD (0o644 perm) deferred

## Pitfalls
- `filepath.Rel(dir, ".")` returns `"../."` not `".."` — need `filepath.Clean` before use
- mtime stale test: `os.WriteFile` within same second produces same Unix mtime — tamper DB mtime directly instead
- `applyFileRewrites` must return backups for caller to restore on post-rewrite failure
