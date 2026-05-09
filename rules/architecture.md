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

分割は YAGNI と責務境界を分けて判断する。

- 未来の可能性だけで空に近い package や汎用 abstraction を作らない。
- 一方で、概念として独立しており、依存方向をコンパイラで固定したい境界は、小さくても分割してよい。
- 「小さいから同居させる」は、責務名・公開 API・依存方向が明確な場合に限る。小さいことは雑に置いてよい理由にはしない。
- 分割を見送る場合も、package 境界・公開 interface・テストで責務を明確に保つ。

分割を検討する目安:

- 責務名が安定しており、呼び出し元から見た公開 API を説明できる。
- 依存方向が一方向にでき、循環回避のための不自然な workaround が不要。
- SQLite / YAML / filesystem / CLI 表示の置き場所が明確。
- 近い backlog で同じ境界を繰り返し触る、または誤った参照を package 境界で防ぐ価値がある。
