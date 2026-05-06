# Goal State

status: running

## Current scope

v0.6.1 の残タスク（2件）:
- `cmd/mdhop/format.go`（838 行）をコマンド別に分割
- `internal/core/query.go`（797 行）をエントリ解決とデータフェッチに分離

## Last completed loop

2026-05-06: ノード型文字列リテラル定数化を完了。
- `internal/core/db.go` に exported untyped string const（`NodeTypeNote`/`NodeTypeAsset`/`NodeTypePhantom`/`NodeTypeTag`）を追加
- 9 ファイル 26 箇所の Go リテラル参照を const に置換
- SQL 内シングルクォート、test ファイル、linkType 用 `"tag"`/`"frontmatter"` は対象外（`goal-decisions.md` 2026-05-06 参照）
- レビュー: facts/design/go/mdhop の 4 観点すべて needs_action=NO で完了
- 検証: `go vet ./...` `go build ./...` `go test ./...` 全グリーン

## Skipped tasks

なし

## Last verification

`go vet ./...` `go build ./...` `go test ./...` 全グリーン（2026-05-06）

## Next hint

次ループは `cmd/mdhop/format.go`（838 行）のコマンド別分割を上から順に取り組む。format.go は共通ヘルパー（encodeJSON, printStringListText, parseFields, validateFormat, validateFields, fieldSet）を残し、コマンド別の `printXText`/`printXJSON` ペアを `format_<command>.go` に移す方針。
