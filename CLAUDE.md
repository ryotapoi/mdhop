# CLAUDE.md

## プロジェクト概要

mdhop は Coding Agent 向けの CLI ツール。Obsidian Vault 相当の Markdown リポジトリ内のリンク関係を事前解析して SQLite にインデックス化し、grep に頼らず関連ノートへ辿れるようにする。詳細: docs/rules/01-concept.md

## ワークフロー入口

入口は依頼の形で 2 通り。

- **Goal（`/goal` または `goal-workflow` を明示指定）**: `goal-workflow` skill を入口にする。Goal は作業全体を 1 commit 単位へ分割し、各 commit で `.claude/workflow/default.md` 以下の phase workflow を回す。Goal 手順の正本は `.claude/workflow/goal.md`。`goal-workflow` skill はそのファイルを読んで進める。Goal 前提では都度確認を避けて自動進行し、止まるのは各 workflow の Stop Conditions だけ。
- **単発依頼**: `.claude/workflow/default.md` を最初に Read し、Intake 分類（Small / Normal / High-risk / Exploratory）から必要な phase ファイルへ進む。

```text
goal-workflow skill（Goal の入口）
└── goal.md（正本: commit slicing / Goal Review / branch / ff-merge）
    └── default.md（各 commit / 単発依頼の Intake・Routing）
        ├── investigate.md — Exploratory 用の事実集め
        ├── plan.md — 計画作成（省略可条件含む。plan mode は使わない）
        ├── implement.md — 実装
        ├── verify.md — 動作確認
        ├── review.md — リスクベースの review depth 選択
        ├── finish.md — コミット + 文書同期
        └── maintenance.md — L3、節目で呼ぶ構造棚卸し
```

各 phase ファイルは入る前に Read で読む（CLAUDE.md の要約で済ませない）。
plan mode（`EnterPlanMode` / `ExitPlanMode`）は使わない。計画は内部で立ててそのまま実装する。
不明点があれば止まってユーザーに確認。なければ自動進行。
単発依頼はコミットまで終えたら止まる（次のタスクはユーザー指示待ち）。Goal は完了したら止まる。

## docs/

計画・実装時に必ず Read で参照すること。CLAUDE.md の要約で済ませず、実ファイルを読んで判断する。

- コア思想と設計根拠: `docs/rules/01-concept.md`
- 要件定義: `docs/rules/02-requirements.md`
- DB スキーマとクエリ設計: `docs/rules/03-data-model.md`
- ユーザー視点のコマンド仕様: `docs/specs/overview.md`
- 情報管理の原則（フォルダ構成・情報分類・SSoT）: `docs/rules/information-management.md`
- モジュール構成と依存方向: `docs/rules/architecture.md`

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

### ユーザーへの質問

ユーザーに質問することになった場合は `~/.claude/resources/rules/asking-user.md` を Read してから質問を組み立てる。

## ドキュメント管理

- 同じ情報を複数のドキュメントに書かない。各情報の置き場所は1箇所に限定する（DRY / SSoT は `docs/rules/information-management.md` 参照）
- `.claude/`・`CLAUDE.md`（Claude 側）と `.agents/`・`AGENTS.md`（Codex 側）は、目的・制約・判断基準の方向性を揃える。subagent、review delegation、tool 呼び出し、skill / workflow の実行手順は各エージェントの仕組みに合わせてよい。共有方針を片方で変更したら、同じコミットで他方にも反映する
- 新しいスキルやファイルを作成したら、同じステップで `.claude/settings.json` 等への登録も行う
- `docs/` が正本、`llm-wiki/` は正本に負ける AI 編纂の作業入口（各ファイルの `regen` で再生成可否を宣言）。権威による配置は `docs/rules/information-management.md`
- 技術的な知見・ハマりどころは `llm-wiki/knowledge.md`（`regen: none`）に集約する

## 言語

コミットメッセージは英語（Conventional Commits）。ドキュメントは日本語の場合がある。コード（変数名、コメント）は英語で書く。
