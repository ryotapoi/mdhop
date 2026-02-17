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

- `normalizePath` (`util.go`): パス正規化。`filepath.ToSlash` + `filepath.Clean` + 先頭 `./` 除去
- `basenameKey` (`util.go`): `.md` を除いた小文字 basename を返す
- `isFieldActive` (`util.go`): format 文字列中にフィールドプレースホルダが含まれるかチェック。query/stats/diagnose 共通
- `rewriteRawLink`, `applyFileRewrites`, `isBasenameRawLink`, `replaceOutsideInlineCode` (`rewrite.go`): リンク書き換え共通ロジック

## フィールドバリデーション

- resolve/stats/diagnose/query の field validation は CLI 層（`cmd/mdhop/*.go`）で DB オープン前に実行する
- `parseFields`, `validateFormat`, `validateFields` (`format.go`) を使用

## CLI テスト規約

- CLI テスト（`cmd/mdhop/*_test.go`）は `exec.Command` でバイナリを起動する方式
- テスト前に `go build` でバイナリをビルドし、`t.TempDir()` に配置する
- 正常系: stdout の内容と exit code 0 を検証
- エラー系: stderr のメッセージと非ゼロ exit code を検証

## アサーション規約

- 件数チェックだけでなく、値の中身（node_key, link_type, path 等）も検証する
- テーブル駆動テストでは `t.Run(name, ...)` でサブテスト化する
- エラーメッセージのアサーションは `strings.Contains` でキーフレーズのみ検証（完全一致にしない）
