# Goal State

status: running

## Current scope

v0.6.1 の残タスク（1件）:
- `internal/core/query.go`（797 行）をエントリ解決とデータフェッチに分離

## Last completed loop

2026-05-06: `cmd/mdhop/format.go`（838 行）をコマンド別ファイルに分割。
- 共通ヘルパーは `format.go`（196 行）に残す: `printWarnings` `parseFields` `validateFormat` `validateFields` `fieldSet` `printStringListText` `encodeJSON` + 共有型 `jsonNodeInfo`/`toJSONNodeInfo` + mutation 共通 `rewrittenJSON`/`toRewrittenJSON`/`printRewrittenText` + repair/simplify 共通 `skippedJSON`/`toSkippedJSON`/`printSkippedText`/`printRewriteResultJSON`
- コマンド別 14 ファイルへ分離: `format_resolve.go` `format_stats.go` `format_diagnose.go` `format_search.go` `format_query.go` `format_delete.go` `format_update.go` `format_add.go` `format_move.go` `format_movedir.go` `format_disambiguate.go` `format_convert.go` `format_repair.go` `format_simplify.go`
- 最大ファイル: `format_query.go` 186 行（writeNodeInfoText/nodeInfoOneLine/queryJSONOutput 等 query 専用ロジックを集約）
- レビュー: facts/design/go/mdhop の 4 観点すべて needs_action=NO で完了。go では既存問題の NIT（`validSearchFieldsCLI` 未使用）が指摘されたが本タスクのスコープ外
- 検証: `go vet ./...` `go build ./...` `go test ./...` 全グリーン

## Skipped tasks

なし

## Last verification

`go vet ./...` `go build ./...` `go test ./...` 全グリーン（2026-05-06）

## Next hint

次ループは `internal/core/query.go`（797 行）の分割。`query.go` には `Query` エントリポイント + 型定義のみを残し、エントリ解決系（`findEntryNode` `findEntryByKey` 等 6 関数）を `query_entry.go`、データフェッチ系（`queryBacklinks` `queryOutgoing` `queryTwoHop` `fetchNodeInfoBatch` + ファイル読み取り `readHead` `readSnippets` `readFileLines`）を `query_fetch.go` に分離する方針。
