# Goal State

status: running

## Current scope

`backlog/backlog.md` の v0.7.0 の全タスク完了。

## Last completed loop

2026-05-06: `internal/core/move_dir.go` の `MoveDir` ロールバックパス向けテストを 2 件追加。
- `TestMoveDir_Rollback_RenameFails`: destination dir を read-only にして Phase 4.3 の `os.Rename` を失敗させ、defer 内 rename rollback (607) と external rewrite restore (619) を発火。`Other.md` の `[[sub/B]]` が元に戻ること、DB の path / edge raw_link が元のままであることを検証
- `TestMoveDir_Rollback_MovedFileRestore`: `MoveDir(sub → newdir/inner)` で各 sub/*.md の `../external.md` 相対リンクを `../../external.md` に書き換え必須にして Phase 4.2 で `movedFileBackups` を埋め、`newdir/inner` を read-only にして Phase 4.3 を失敗させ、moved file の content rollback (583-590 / 611-617) を発火。
- root ユーザー実行時は permission チェックがバイパスされるため `os.Geteuid()==0` でスキップ
- 既存コードロジックは未変更。`go vet ./...` / `go test ./...` 全部グリーン
- レビュー: 非 Small だがテスト追加のみ・既存テスト全 pass・コードロジック変更なし → self-check で完了

## Skipped tasks

なし

## Last verification

`go vet ./...` / `go test ./...` 全部グリーン。

## Next hint

次タスクは backlog v0.7.0 の上から: `internal/core/move_dir.go` の `MoveDir`（758 行 1 関数）の分解。今回追加したロールバックテストが回帰検出網。move_helpers.go 側に段階別ヘルパーを追加し、MoveDir はオーケストレーションのみに縮小。ロールバックは defer + 状態フラグで集約する方針。
