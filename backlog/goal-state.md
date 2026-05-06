# Goal State

status: done

## Current scope

v0.6.1 の全タスク完了。

## Last completed loop

2026-05-06: `internal/core/query.go`（797 行）をエントリ解決とデータフェッチに分離。
- `query.go`（156 行）: 型定義（`EntrySpec` `QueryOptions` `NodeInfo` `TwoHopEntry` `SnippetEntry` `QueryResult`）と `Query` エントリポイントのみ
- `query_entry.go`（200 行）: `findEntryNode` `findEntryByKey` `findEntryByFile` `findEntryByTag` `findEntryByPhantom` `findEntryByName` `fetchNodeInfo`
- `query_fetch.go`（438 行）: `queryBacklinks` `queryOutgoing` `queryTags` `filterLeafTags` `fetchNodeInfoBatch` `queryTwoHop` `readHead` `readSnippets` `checkStale` `readFileLines`
- ロジック変更なし・公開 API 変更なし・SQL 変更なしの純粋な物理分割
- レビュー: facts / design / mdhop は LGTM。go で `sql.ErrNoRows` の `==` 比較が指摘されたが、これは元コードからの持ち越しで、backlog v0.7.0「`internal/core` の sentinel error 化」で横断対応する範囲のため今回は対処せず（再レビューで妥当と判定）
- 検証: `go vet ./...` `go build ./...` `go test ./...` 全グリーン

## Skipped tasks

なし

## Last verification

`go vet ./...` `go build ./...` `go test ./...` 全グリーン（2026-05-06）

## Next hint

v0.6.1 の Acceptance（v0.6.1 セクションに `- [ ]` がない）を満たしたため `goal-done.md` を作成して終了。
