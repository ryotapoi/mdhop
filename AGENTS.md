# AGENTS.md

## Project

mdhop は Coding Agent 向けの CLI ツール。Obsidian Vault 相当の Markdown リポジトリ内のリンク関係を SQLite に事前解析し、grep に頼らず関連ノートへ辿れるようにする。詳細は `docs/rules/01-concept.md`。

複数タスク後の構造・負債の棚卸しは、明示的に依頼された場合だけ global `maintenance-audit` skill を使う。

Claude Code 由来の `.claude/` は参考資料であり、Codex の入口ではない。

## Information Sources

- `docs/rules/`: プロダクト目的、アーキテクチャ、情報管理の正本
- `docs/specs/`: ユーザー視点の振る舞い仕様（`overview.md` / `stories.md` / `test-plan.md`）
- `docs/decisions/`: 後から理由を問われる判断
- `backlog/backlog.md`: 未着手・進行中の作業項目
- `llm-wiki/`: AI が編纂・保守する作業の入口（正本ではない。ソース/正本と矛盾したらそちらが勝つ）

`docs/` が正本、`llm-wiki/` は正本に負ける作業の入口（各ファイルの `regen` で再生成可否を宣言）。権威による配置の詳細は `docs/rules/information-management.md`。必要な情報だけ読む。判断に影響する可能性がある情報源は、推測で済ませず実物を確認する。

変更時の検証方法と必須gateは `docs/rules/verification.md` を正本とする。

## Core Policies

- workflow / skill は ICAR（Intent / Constraints / Acceptance / Relevant）を基本形にする。
- 小さい変更に重い手続きを載せない。作業の大きさとリスクで plan / verify / review の深さを選ぶ。
- 原則 1 plan = 1 commit。独立した成果が混ざるなら plan を分ける。
- 仕様・CLI 挙動・データ保持・削除方針に複数の妥当な選択肢がある場合はユーザーに確認する。
- 技術的知見は特定ソースに紐づくものはそのコードのコメントへ、横断的な挙動は `llm-wiki/` の該当地図へ。単一の集約ファイルは作らない。workflow / skill 本体を肥大化させない。
- 後から制約になる判断は `docs/decisions/` に残す。
- 広い構造改善は必要に応じて `backlog/backlog.md` へ切り出すか、`maintenance-audit` skill の棚卸し対象とする。

## Skills

Codex 用のプロジェクトスキルは `.agents/skills/` に置く。グローバルスキルは `~/.agents/skills/` に置く。commit は project local skill ではなく global `commit` skill を使う。

主に使うスキル:

- `design-decision`: 設計判断の価値基準を当てる
- `maintenance-audit`: 明示的に依頼された場合に、既存コード全体からリファクタリング候補を棚卸しする
- `commit`: global skill を使い、review 済み差分だけを stage して Conventional Commits 形式でコミットする

独立した調査・レビュー・実装は subagent で並列化してよい。1 subagent = 1 タスクに絞る。

## mdhop Constraints

- `cmd/mdhop` は CLI 入出力とフラグ解析を担い、DB 操作・リンクパース・パス解決は `internal/core` に置く。
- `internal/core` は `cmd/mdhop` に依存しない。
- `internal/core` の外部ライブラリは `modernc.org/sqlite`、`gopkg.in/yaml.v3`、`golang.org/x/text/unicode/norm` を基本とする（NFC path 正規化は ADR 0020）。
- DB に Markdown 本文 TEXT を保存しない。スニペットは query 時にファイルから切り出す。
- 曖昧解決は静かに誤解決しない。厳密モードとルート優先ルールを守る。
- stdout JSON は agent 向け安定インターフェースとして扱う。warnings 等の付加情報は stderr に出す。

## Language

- コード・コメント・コミットメッセージ: 英語
- ドキュメント（`docs/`, `backlog/`, `llm-wiki/`, `.agents/`）: 日本語
- `AGENTS.md`: 日本語
