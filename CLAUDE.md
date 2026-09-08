# CLAUDE.md

プロジェクト指示の正本として、作業開始時に `AGENTS.md` を読む。このファイルと重複する説明より `AGENTS.md` を優先する。

## プロジェクト概要

mdhop は Coding Agent 向けの CLI ツール。Obsidian Vault 相当の Markdown リポジトリ内のリンク関係を事前解析して SQLite にインデックス化し、grep に頼らず関連ノートへ辿れるようにする。詳細: docs/rules/01-concept.md

## docs/

計画・実装時に必ず Read で参照すること。CLAUDE.md の要約で済ませず、実ファイルを読んで判断する。

- コア思想と設計根拠: `docs/rules/01-concept.md`
- 要件定義: `docs/rules/02-requirements.md`
- DB スキーマとクエリ設計: `docs/rules/03-data-model.md`
- ユーザー視点のコマンド仕様: `docs/specs/overview.md`
- 情報管理の原則（フォルダ構成・情報分類・SSoT）: `docs/rules/information-management.md`
- モジュール構成と依存方向: `docs/rules/architecture.md`

## 開発スタイル

複数タスク後の構造・負債の棚卸しに `maintenance-audit` skill を使うのは、明示的に依頼された場合だけとする。

### サブエージェント活用

メインコンテキストを汚さないために、skill 以外の場面でもサブエージェントを積極的に使う。

- 調査・比較・コード探索は Explore サブエージェントに委譲する
- 独立した作業は並列でサブエージェントを起動する
- 1 サブエージェント = 1 タスクに絞り、焦点を明確にする

## ドキュメント管理

- 同じ情報を複数のドキュメントに書かない。各情報の置き場所は1箇所に限定する（DRY / SSoT は `docs/rules/information-management.md` 参照）
- 新しいスキルやファイルを作成したら、同じステップで `.claude/settings.json` 等への登録も行う
- `docs/` が正本、`llm-wiki/` は正本に負ける AI 編纂の作業入口（各ファイルの `regen` で再生成可否を宣言）。権威による配置は `docs/rules/information-management.md`
- 技術的な知見・ハマりどころは単一ファイルに集約しない。特定の関数/ファイルだけに効く罠はそのソースのコメントへ、複数箇所にまたがる挙動・設計理解は `llm-wiki/` の該当する地図へ統合する。どちらにも収まらない外部由来の知見が溜まったらテーマ別の `regen: none` ページを立てる

## 言語

コミットメッセージは英語（Conventional Commits）。ドキュメントは日本語の場合がある。コード（変数名、コメント）は英語で書く。
