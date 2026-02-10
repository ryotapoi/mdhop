# stats コマンド: コアロジック + CLI 実装

## Touches
- `internal/core/stats.go` (新規) — `Stats()`, フィールドバリデーション, COUNT クエリ
- `internal/core/stats_test.go` (新規) — 7 テストケース
- `cmd/mdhop/stats.go` (新規) — CLI エントリポイント (`--vault`, `--format`, `--fields`)
- `cmd/mdhop/format.go` (編集) — `printStatsJSON`, `printStatsText` フォーマッタ追加
- `cmd/mdhop/main.go` (編集) — ルーティング + ヘルプに stats 追加
- `cmd/mdhop/cli_test.go` (編集) — 4 CLI テストケース追加

## Verification
- `go test ./... -count=1` — 全 131 テスト PASS (23 parse + 22 build + 15 resolve + 34 query + 7 stats + 26 CLI + 4 other)
- `/codex-impl-review` によるレビュー実施・指摘2件反映済み
  - `edges_total` の正確な値検証を追加（>= 10 → == 17）
  - CLI 出力テストに `edges_total` フィールドの検証を追加

## Pitfalls
- **フィールドバリデーションの実行順序**: `validateStatsFields()` を DB オープンの前に配置する必要がある。DB がない状態で unknown field エラーを返すため。初回テストで発見・修正
