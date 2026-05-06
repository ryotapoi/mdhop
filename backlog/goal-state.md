# Goal State

status: running

## Current scope

`backlog/backlog.md` の v0.7.0 の全タスク完了。

## Last completed loop

2026-05-06: ノード型を `type NodeType string` の named type に昇格。
- `internal/core/db.go` に `type NodeType string` を定義し、`NodeTypeNote/Asset/Phantom/Tag` を昇格
- `upsertNode` / `listDirNodesByType` / `queryBasenameMatches` / `queryTwoHop` / `queryCollateralRewrites` のシグネチャを `NodeType` に変更
- `core.NodeInfo.Type` / `core.ResolveResult.Type` / `cmd/mdhop.jsonNodeInfo.Type` を `NodeType` に
- DB scan は `var typ NodeType` で直接受ける（database/sql の reflect で動作）
- テストヘルパー `edgeRow.targetType` / `nodeRow.nodeType` / `queryNodes` を `NodeType` 化、関連リテラル比較を定数化（一部スコープ外残存あり、decisions 参照）
- 全テスト 388+ 通過、`go vet` クリーン

## Skipped tasks

なし

## Last verification

`go build ./...` / `go vet ./...` / `go test ./...` 全部グリーン。

## Next hint

次タスクは backlog v0.7.0 の上から: `internal/core` の sentinel error 化。
