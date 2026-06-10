---
name: goal-workflow
description: ユーザーが /goal を使った時、または goal-workflow を明示指定した時だけ使う。
---

# Goal Workflow

## Intent

mdhop の実装作業を開始し、`.claude/workflow/goal.md` を正本として Goal を完了まで進める。

## Constraints

- この skill は実行入口であり、Goal 手順の正本は `.claude/workflow/goal.md` とする。
- 最初に `.claude/workflow/goal.md` を Read し、Goal の完了条件・branch 運用・commit slicing・Goal Review・設計判断の扱いに従う。
- 各 commit では `.claude/workflow/default.md` を使い、必要な phase workflow だけ Read する。
- Goal 前提では都度のユーザー確認を避け、自動進行する。止まるのは `goal.md` の Stop Conditions に該当する場合だけ。
- Goal の途中状態、backlog の具体項目、レビュー結果、設計判断の記録は skill に書かず、適切な情報源（`backlog/backlog.md` / `decisions/` / `specs/`（あれば） / `rules/`）へ同期する。

## Acceptance

- `.claude/workflow/goal.md` に従って Goal が進行している。
- 各 commit が `.claude/workflow/default.md` を満たしている。
- Goal 完了時に Goal Review（Codex + `/code-review`）と必要な記録同期が済み、ブランチが `--ff-only` で main にマージされている。
- Goal 完了時に、設計判断がない場合も `設計判断: なし` と明示されている。

## Relevant

- `.claude/workflow/goal.md`
- `.claude/workflow/default.md`
- `codex-review` skill
- `backlog/backlog.md`

## Flow

1. `.claude/workflow/goal.md` を読む。
2. 専用ブランチを切り、Goal 開始時の base commit を記録する。
3. Goal を 1 commit 単位へ切り、各 commit で `.claude/workflow/default.md` を実行する。
4. Goal 完了前に、`.claude/workflow/goal.md` の Goal Review 条件（Codex + `/code-review`）を満たす。
5. `--ff-only` で main にマージし、Goal 全体の結果、残リスク、設計判断をまとめる。設計判断がない場合も `設計判断: なし` と書く。
