# AGENTS.md

## Project

mdhop は Coding Agent 向けの CLI ツール。Obsidian Vault 相当の Markdown リポジトリ内のリンク関係を SQLite に事前解析し、grep に頼らず関連ノートへ辿れるようにする。詳細は `rules/01-concept.md`。

## Entry Point

入口は依頼の形で 2 通り。

- **Goal（`/goal` または `goal-workflow` を明示指定）**: グローバルの `goal-workflow` skill（`~/.agents/skills/goal-workflow/`）を入口にする。Goal は作業全体を 1 commit 単位へ分割し、各 commit で `.agents/workflow/default.md` 以下の phase workflow を回す。Goal 手順の正本は `.agents/workflow/goal.md`。
- **単発依頼**: 最初に `.agents/workflow/default.md` を読み、Intake から必要な phase ファイルへ進む。

各 phase に入るときだけ、対応する workflow ファイルを読む。`AGENTS.md` の要約だけで進めない。

```text
goal-workflow skill（Goal の入口）
└── .agents/workflow/goal.md（正本: commit slicing / Claude review / 完了条件）
    └── default.md（各 commit / 単発依頼の Intake・Routing）
        ├── investigate.md
        ├── plan.md
        ├── implement.md
        ├── verify.md
        ├── review.md
        ├── finish.md
        └── maintenance.md
```

Claude Code 由来の `.claude/` は参考資料であり、Codex の入口ではない。

## Information Sources

- `rules/`: プロダクト目的、外部仕様、データモデル、アーキテクチャ、情報管理の正本
- `decisions/`: 後から理由を問われる判断
- `backlog/backlog.md`: 未着手・進行中の作業項目
- `references/knowledge.md`: 技術的な知見・ハマりどころ
- `derived/`: 正本からの派生ビュー。矛盾時は `rules/` などの正本を優先する
- `specs/`: 現状は未配置。`rules/` とテストの間に明文化すべき振る舞い仕様が増えた場合の置き場

必要な情報だけ読む。判断に影響する可能性がある情報源は、推測で済ませず実物を確認する。

## Core Policies

- workflow / skill は ICAR（Intent / Constraints / Acceptance / Relevant）を基本形にする。
- 小さい変更に重い手続きを載せない。作業の大きさとリスクで plan / verify / review の深さを選ぶ。
- 原則 1 plan = 1 commit。独立した成果が混ざるなら plan を分ける。
- 仕様・CLI 挙動・データ保持・削除方針に複数の妥当な選択肢がある場合はユーザーに確認する。
- 技術的知見は `references/knowledge.md` に集約する。workflow / skill 本体を肥大化させない。
- 後から制約になる判断は `decisions/` に残す。
- 広い構造改善は必要に応じて `backlog/backlog.md` または `maintenance.md` の対象へ切り出す。
- workflow は 1 つの commit 単位で回す。Goal が複数 commit に分かれる場合は `goal-workflow` skill に従って commit 単位へ分けて繰り返す。
- 単発依頼はコミットまで終えたら止まる（次のタスクはユーザー指示待ち）。Goal は完了したら止まる。

## Skills

Codex 用のプロジェクトスキルは `.agents/skills/` に置く。グローバルスキルは `~/.agents/skills/` に置く。

主に使うスキル:

- `goal-workflow`: `/goal` または明示指定時だけグローバル skill（`~/.agents/skills/goal-workflow/`）を使う。Goal を 1 commit 単位へ分割して完了まで進める
- `investigate`: 計画前の不明点を調査する
- `design-decision`: 設計判断の価値基準を当てる
- `mdhop-risk-check`: mdhop 固有の制約に照らして plan / 実装を確認する
- `maintenance-audit`: 複数タスク後の構造・負債を棚卸しする（light / deep を scope で指定）
- `commit`: Conventional Commits 形式でコミットする

独立した調査・レビュー・実装は subagent で並列化してよい。1 subagent = 1 タスクに絞る。

## Synchronization

- `.claude/`・`CLAUDE.md`（Claude 側）と `.agents/`・`AGENTS.md`（Codex 側）は、方針・ルールの内容を一致させる。形式は各側の流儀（`.agents/` は ICAR）に合わせてよいが、`skills/mdhop-risk-check/SKILL.md` は同一内容を保つ。片方を変更したら、同じコミットで他方にも反映する。

## mdhop Constraints

- `cmd/mdhop` は CLI 入出力とフラグ解析を担い、DB 操作・リンクパース・パス解決は `internal/core` に置く。
- `internal/core` は `cmd/mdhop` に依存しない。
- `internal/core` の外部ライブラリは `modernc.org/sqlite` と `gopkg.in/yaml.v3` を基本とする。
- DB に Markdown 本文 TEXT を保存しない。スニペットは query 時にファイルから切り出す。
- 曖昧解決は静かに誤解決しない。厳密モードとルート優先ルールを守る。
- stdout JSON は agent 向け安定インターフェースとして扱う。warnings 等の付加情報は stderr に出す。
- `delete --rm`, `move` などの破壊的操作は testdata 等の一時 vault で確認する。

## Tooling

```bash
go test ./...
go test ./internal/core/
go test ./internal/core/ -run TestBuild
go build ./...
go vet ./...
go build -o bin/mdhop ./cmd/mdhop
```

変更内容に応じて `bin/mdhop <args>` で stdout / stderr / 終了コード / DB 副作用を確認する。

## Language

- コード・コメント・コミットメッセージ: 英語
- ドキュメント（`rules/`, `backlog/`, `decisions/`, `references/`, `.agents/`）: 日本語
- `AGENTS.md`: 日本語
