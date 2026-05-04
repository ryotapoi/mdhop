# CLAUDE.md

## プロジェクト概要

mdhop は Coding Agent 向けの CLI ツール。Obsidian Vault 相当の Markdown リポジトリ内のリンク関係を事前解析して SQLite にインデックス化し、grep に頼らず関連ノートへ辿れるようにする。詳細: rules/01-concept.md

## ワークフロー入口

タスクを始める時は `.claude/workflow/default.md` を最初に Read する。
そこから Intake 分類（Small / Normal / High-risk / Exploratory）を判定し、必要な phase ファイルへ進む。

phase ファイル一覧（`.claude/workflow/`）:

- `default.md` — 入口、Intake、Routing
- `investigate.md` — Exploratory 用の事実集め
- `plan.md` — 計画作成（省略可条件含む）
- `implement.md` — 実装
- `verify.md` — 動作確認
- `review.md` — リスクベースの review depth 選択
- `finish.md` — コミット + 文書同期
- `maintenance.md` — L3、節目で呼ぶ構造棚卸し

各 phase ファイルは入る前に Read で読む（CLAUDE.md の要約で済ませない）。
不明点があれば止まってユーザーに確認。なければ自動進行。
コミットまで終えたら止まる。次のタスクはユーザー指示待ち。

## rules/

計画・実装時に必ず Read で参照すること。CLAUDE.md の要約で済ませず、実ファイルを読んで判断する。

- コア思想と設計根拠: `rules/01-concept.md`
- 要件定義: `rules/02-requirements.md`
- DB スキーマとクエリ設計: `rules/03-data-model.md`
- ユーザー視点のコマンド仕様: `rules/overview.md`
- 情報管理の原則（フォルダ構成・情報分類・SSoT）: `rules/information-management.md`
- モジュール構成と依存方向: `rules/architecture.md`

## ビルド・テストコマンド

```bash
go test ./...                          # 全テスト実行
go test ./internal/core/               # 特定パッケージのテスト実行
go test ./internal/core/ -run TestBuild # 特定テストのみ実行
go build ./...                         # 全パッケージのビルド
go vet ./...                           # 静的チェック
go build -o bin/mdhop ./cmd/mdhop      # CLI 動作確認用バイナリ
```

## 開発スタイル

### サブエージェント活用

メインコンテキストを汚さないために、skill 以外の場面でもサブエージェントを積極的に使う。

- 調査・比較・コード探索は Explore サブエージェントに委譲する
- 独立した作業は並列でサブエージェントを起動する
- 1 サブエージェント = 1 タスクに絞り、焦点を明確にする

## ドキュメント管理

- 同じ情報を複数のドキュメントに書かない。各情報の置き場所は1箇所に限定する（DRY / SSoT は `rules/information-management.md` 参照）
- 新しいスキルやファイルを作成したら、同じステップで `.claude/settings.json` 等への登録も行う
- 技術的な知見・ハマりどころは `references/knowledge.md` に集約する

## 言語

コミットメッセージは英語（Conventional Commits）。ドキュメントは日本語の場合がある。コード（変数名、コメント）は英語で書く。
