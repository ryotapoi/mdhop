# query コマンド: コアロジック実装

## Touches
- `internal/core/query.go` (新規) — `Query()`, エントリ解決, backlinks/outgoing/tags/twohop/head/snippet
- `internal/core/query_test.go` (新規) — 34 テストケース
- `testdata/vault_query_ambiguous_name/` (新規) — basename 衝突あり・曖昧リンクなしのフィクスチャ

## Verification
- `go test ./... -count=1` — 全 98 テスト PASS (23 parse + 22 build + 15 resolve + 34 query + 4 other)
- `/codex-impl-review` によるレビュー実施・指摘確認済み
  - stale 時エラーは overview.md の仕様通り
  - `via_max_degree` はプランで Out of Scope と明示

## Pitfalls
- **outgoing の self-link 除外**: `[[#Index]]` のような self-link は outgoing に含めない。`e.target_id != e.source_id` 条件をクエリに追加して対処。プランに明示していなかったがテスト実行時に発見
- **ambiguous name テスト**: `vault_build_conflict` は `[[A]]` を含むため build がエラーになる。basename 衝突のみ（曖昧リンクなし）のフィクスチャ `vault_query_ambiguous_name` を別途作成
