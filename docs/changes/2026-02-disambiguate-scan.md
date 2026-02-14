# disambiguate --scan: DB なし走査モード

## Touches
- `internal/core/disambiguate.go` — `DisambiguateScan()` 関数を新設
- `cmd/mdhop/disambiguate.go` — `--scan` フラグの stub 解除、`DisambiguateScan()` 呼び分け
- `internal/core/disambiguate_test.go` — scan モードのテスト 10 件追加
- `docs/test-plan.md` — `--scan` テスト一覧を追記

## Verification
- `go test ./internal/core/ -run TestDisambiguateScan -v` — 10 テスト全 PASS
- `go test ./...` — 全テスト通過（既存テストの退行なし）
- `/codex-impl-review` — LGTM（指摘なし）

## Pitfalls
- `applyFileRewrites()` の `sourceID=0` 固定は `newMtimes` を無視する前提でのみ安全。scan 版では DB 操作しないので問題ないが、将来 mtime を使う場合は要注意
