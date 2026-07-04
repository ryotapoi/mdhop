---
paths:
  - "internal/**"
  - "cmd/**"
---

# mdhop コーディング規約

## DB アクセスパターン

- 内部関数は `dbExecer` インターフェース（`db.go` 定義）を受け取る。公開関数は `*sql.DB` を受け取り、必要に応じてトランザクション内で内部関数を呼ぶ
- `dbExecer` は `Exec`, `QueryRow`, `Query` の3メソッドを持つ。`*sql.DB` と `*sql.Tx` の両方が満たす

## 共通ヘルパー

- `NormalizePath` (`util.go`): パス正規化。`filepath.ToSlash` + `filepath.Clean` + 先頭 `./` 除去
- `basenameKey` (`util.go`): `.md` を除いた小文字 basename を返す
- `isFieldActive` (`util.go`): format 文字列中にフィールドプレースホルダが含まれるかチェック。query/stats/diagnose 共通
- `rewriteRawLink`, `applyFileRewrites`, `isBasenameRawLink`, `replaceOutsideInlineCode` (`rewrite.go`): リンク書き換え共通ロジック

## フィールドバリデーション

- resolve/stats/diagnose/query の field validation は CLI 層（`cmd/mdhop/*.go`）で DB オープン前に実行する
- `parseFields`, `validateFormat`, `validateFields` (`format.go`) を使用
- query/stats/diagnose のフィールド名は `internal/core` の定数を参照し、core 分岐・cmd validation・format 出力キーの生文字列重複を避ける

## CLI テスト規約

- CLI テスト（`cmd/mdhop/cli_test.go`）はバイナリを起動せず、`runQuery` / `runSearch` 等の run 関数を直接呼び出す方式
- vault が必要なテストは `setupVaultForCLI(t, name)` で `testdata/vault_<name>` を `t.TempDir()` にコピーし、`core.Build` でインデックスを構築する
- stdout の検証は `os.Stdout` を `os.Pipe` に差し替えてキャプチャする（`captureStdout` ヘルパー）。この方式のため `t.Parallel()` は使わない（直列実行前提）
- 正常系: run 関数が nil error を返し、stdout の JSON / text 出力の中身を検証
- エラー系: run 関数の戻り値 error のメッセージを検証
- 出力フォーマッタ単体のテストは `format_test.go` で、結果構造体を直接作って `bytes.Buffer` に書き出して検証する

## アサーション規約

- 件数チェックだけでなく、値の中身（node_key, link_type, path 等）も検証する
- テーブル駆動テストでは `t.Run(name, ...)` でサブテスト化する
- エラーメッセージのアサーションは `strings.Contains` でキーフレーズのみ検証（完全一致にしない）
