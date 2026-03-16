# アーキテクチャ

## モジュール構成

```
cmd/mdhop/       CLI エントリーポイント。フラグ解析・出力フォーマット・サブコマンドルーティング
internal/core/   ビジネスロジック。パース・インデックス構築・リンク解決・クエリ・DB 操作
internal/testutil/ テストヘルパー（CopyDir 等）
```

## 依存方向

```
cmd/mdhop → internal/core
cmd/mdhop → internal/testutil (テストのみ)
```

- `internal/core` は `cmd/mdhop` に依存しない
- `internal/core` は外部ライブラリとして `modernc.org/sqlite` と `gopkg.in/yaml.v3` のみ使用
- `cmd/mdhop` は stdlib `flag` のみ使用（外部 CLI フレームワーク不使用）

## 責務の境界

| モジュール | 責務 | やらないこと |
|-----------|------|-------------|
| `cmd/mdhop` | CLI 入出力、フラグ解析、出力フォーマット（text/JSON）、エラーメッセージ表示 | DB 操作、リンクパース、パス解決 |
| `internal/core` | ファイルパース、DB スキーマ管理、リンク解決、インデックス構築・更新・クエリ | CLI フラグ解析、出力フォーマット、`os.Exit` |
| `internal/testutil` | テスト用ファイルシステムセットアップ | プロダクションコードからの利用 |

## 新モジュールを切る判断基準

現時点では `internal/core` が全ビジネスロジックを担う。以下の場合に分割を検討する:

- `internal/core` のファイル数が 20 を超え、責務の異なるグループが明確になった場合
- 外部から `internal/core` の一部だけを使いたいケースが発生した場合
