# Goal State

status: running

## Current scope

`backlog/backlog.md` の v0.7.0 の全タスク完了。

## Last completed loop

2026-05-06: `internal/core` の sentinel error 化を完了。
- `internal/core/errors.go` を新規作成し 14 個の sentinel を定義（ErrIndexNotFound / ErrFileNotRegistered / ErrFileNotFound / ErrSourceFileMissing / ErrFileAlreadyRegistered / ErrAlreadyRegistered / ErrAlreadyExistsOnDisk / ErrAmbiguousLink / ErrAmbiguousName / ErrLinkNotFound / ErrLinkEscapesVault / ErrSourceStale / ErrMovedFileStale / ErrAddingMakesAmbiguous）
- `sql.ErrNoRows` の `==` / `!=` 比較を 26 箇所すべて `errors.Is` に置換（test 1 件含む）
- `fmt.Errorf` を `%w` ラップに置換し、各 sentinel のメッセージ文字列を既存エラー先頭フレーズと完全一致させて strings.Contains テスト互換性を維持
- 対象ファイル: db.go, query_entry.go, disambiguate.go, move.go, move_dir.go, move_helpers.go, add.go, update.go, resolve.go, build.go, delete.go, simplify.go, asset_test.go
- `go vet ./...` / `go test ./...` / `go build ./...` 全部グリーン（388+ tests）

## Skipped tasks

なし

## Last verification

`go build ./...` / `go vet ./...` / `go test ./...` 全部グリーン。

## Next hint

次タスクは backlog v0.7.0 の上から: `internal/core/init_meta.go` の責務分離（YAML 生成 / 型推論 / マージ の3ファイル分割）。
