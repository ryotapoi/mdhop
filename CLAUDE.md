# CLAUDE.md

## プロジェクト概要

mdhop は Coding Agent 向けの CLI ツール。Obsidian Vault 相当の Markdown リポジトリ内のリンク関係を事前解析して SQLite にインデックス化し、grep に頼らず関連ノートへ辿れるようにする。詳細: rules/01-concept.md

## ビルド・テストコマンド

```bash
go test ./...                          # 全テスト実行
go test ./internal/core/               # 特定パッケージのテスト実行
go test ./internal/core/ -run TestBuild # 特定テストのみ実行
go build -o bin/mdhop ./cmd/mdhop      # バイナリビルド
```

## rules/

計画・実装時に必ず Read で参照すること。CLAUDE.md の要約で済ませず、実ファイルを読んで判断する。

- コア思想と設計根拠: rules/01-concept.md
- 要件定義: rules/02-requirements.md
- DB スキーマとクエリ設計: rules/03-data-model.md
- ユーザー視点のコマンド仕様: rules/overview.md
- 開発ワークフロー: rules/workflow.md
- 情報管理の原則（フォルダ構成・情報分類・SSoT）: rules/information-management.md

## 開発ワークフロー

IMPORTANT: 各ステップの詳細は rules/workflow.md に定義。ステップに入る前に該当セクションを Read で読むこと。

1. **計画** — rules/workflow.md「Step 1: 計画」を読んでから着手
2. **プランレビュー** — rules/workflow.md「Step 2: プランレビュー」に従う
3. **実装** — rules/workflow.md「Step 3: 実装」を読んでから着手
4. **実装レビュー** — rules/workflow.md「Step 4: 実装レビュー」に従う
5. **コミット** — rules/workflow.md「Step 5: コミット」に従う

## 開発スタイル

### サブエージェント活用

メインコンテキストを汚さないために、skill 以外の場面でもサブエージェントを積極的に使う。

- 調査・比較・コード探索は Explore サブエージェントに委譲する
- 独立した作業は並列でサブエージェントを起動する
- 1 サブエージェント = 1 タスクに絞り、焦点を明確にする

## ドキュメント管理

- 同じ情報を複数のドキュメントに書かない。各情報の置き場所は1箇所に限定する
- 新しいスキルやファイルを作成したら、同じステップで settings.json 等への登録も行う
- 技術的な知見・ハマりどころは references/knowledge.md に集約する

## 言語

コミットメッセージは英語（Conventional Commits）。ドキュメントは日本語の場合がある。コード（変数名、コメント）は英語で書く。

## デバッグ

バグ修正・デバッグ時は `/debug` スキルを使う。
