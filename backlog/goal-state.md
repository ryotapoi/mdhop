# Goal State

status: running

## Current scope

`backlog/backlog.md` の v0.7.0 の全タスク完了。

## Last completed loop

2026-05-07: `internal/core/move_dir.go` の `MoveDir`（758 行 1 関数）を段階別ヘルパーに分割。
- move_helpers.go に段階別ヘルパーを追加: validateMoveDirOptions / loadMovesFromDB / checkDestinationsFree / collectDiskOnlyFiles / classifyDiskState / checkMovedFilesNotStale / adjustMapsForDirMove / collectIncomingRewritesForDir / collectCollateralRewritesForDir / buildMovedFileRewrites / lookupEdgeTargetPath
- move_dir.go: 758 → 297 行に縮小、本体は薄いオーケストレーション
- 型を昇格: moveInfo / movedFileRewrite / dirMoveMaps / diskOnlyMove を package scope に
- Phase 4.3 (rename + completedRenames) と Phase 5 (DB tx) は rollback 状態管理が密に絡むため本体に残置
- 挙動変更なし、テスト追加なし（前ループのロールバックテスト + 既存 388+ テストが回帰検出網）
- レビュー: review-code-all 並列レビュー 2 周。1 周目 facts LGTM / mdhop LGTM / design SHOULD x1 / go SHOULD x1。doc コメント不正確 + 無名 struct を named type 化で対処。2 周目 design LGTM / go SHOULD x1（classifyDiskState 空スライス時の暗黙挙動）→ doc コメント追記で対処
- `go vet ./...` / `go test ./...` 全部グリーン

## Skipped tasks

なし

## Last verification

`go vet ./...` / `go test ./...` 全部グリーン。

## Next hint

次タスクは backlog v0.7.0 の上から: frontmatter 内 wikilink 対応。backlog 該当エントリに「現状」「YAML パース調査結果」「設計論点」が事細かに書かれているため、まずそれらを読み込み、設計判断（新 linkType 追加 vs 既存 wikilink 流用、bare `[[note]]` 検出戦略、書き換え戦略、meta テーブル両立、alias/subpath 対応）を進める。横断対応（add/update/move/disambiguate/convert）が必須なため、複数コミット分割の検討が必要になる可能性が高い → goal-loop B パスに切り替えてタスク細分化を先に行う判断もありうる。
