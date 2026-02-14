# move command: implement file move with link rewriting

## Touches
- `internal/core/rewrite.go` — 新規: add.go からリライト共通関数を抽出
- `internal/core/add.go` — リライト関数群を削除（rewrite.go に委譲）
- `internal/core/move.go` — 新規: `Move()` コア実装
- `internal/core/move_test.go` — 新規: 15テスト
- `testdata/vault_move_basic/` — 新規: テストフィクスチャ
- `testdata/vault_move_phantom/` — 新規: テストフィクスチャ
- `testdata/vault_move_error/` — 新規: テストフィクスチャ
- `cmd/mdhop/move.go` — 新規: CLI
- `cmd/mdhop/main.go` — "move" サブコマンド接続
- `docs/external/overview.md` — move 仕様に追記

## Verification
- `go test ./... -count=1` — 全テスト PASS（既存184 + move 15 = 199テスト）
- `go build ./...` — ビルド成功
- `/codex-impl-review` による Codex レビュー実施、指摘反映済み

## Pitfalls
- `buildRewritePath` はサブディレクトリターゲットに vault-relative パスを返す設計。着リンクリライトにはこれを使うが、発リンクの相対パスリライトには `filepath.Rel` ベースの `rewriteOutgoingRelativeLink` を別途実装した
- 移動ファイルが着リンクの書き換え対象でもある場合（自身への参照がある場合）、着リンク収集時に除外して発リンクフェーズでまとめて処理する必要があった
- 既に移動済みケースでの stale チェック: `os.Rename` は mtime を保持するので、to の mtime と DB の from の mtime を比較する
