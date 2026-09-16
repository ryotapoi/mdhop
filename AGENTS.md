# mdhop

mdhop は Coding Agent 向けの CLI ツール。Obsidian Vault 相当の Markdown リポジトリ内のリンク関係を SQLite に事前解析し、grep に頼らず関連ノートへ辿れるようにする。プロダクトの正本は `docs/rules/01-concept.md`。

## タスク別の正本

タスクに必要な文書だけを読む。推測で済ませず、判断に影響する正本を確認する。

- 目的・要件: `docs/rules/01-concept.md`、`docs/rules/02-requirements.md`
- CLI の振る舞い: `docs/specs/`
- DB schema、query、link resolve、path: `docs/rules/03-data-model.md` と関連する `docs/specs/`
- 責務配置・依存方向: `docs/rules/architecture.md`
- `docs/`、`backlog/`、`llm-wiki/` の配置と正本性: `docs/rules/information-management.md`
- 検証: `docs/rules/verification.md`
- 過去の判断理由: `docs/decisions/`

stdout の JSON は agent 向けの安定インターフェースとして扱い、warnings などの付加情報は stderr に出す。

## Language

コード・コメント・commit message は英語。`AGENTS.md`、`.agents/`、`docs/`、`llm-wiki/`、`backlog/`、`README` 等の文書は日本語。
