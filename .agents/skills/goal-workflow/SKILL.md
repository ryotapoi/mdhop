---
name: goal-workflow
description: Use only when the user invokes /goal or explicitly names goal-workflow.
---

# Goal Workflow

## ICAR

- **Intent**: mdhop の実装作業を開始し、`.agents/workflow/goal.md` を正本として Goal を完了まで進める。
- **Constraints**:
  - この skill は実行入口であり、Goal 手順の正本は `.agents/workflow/goal.md` とする。
  - 最初に `.agents/workflow/goal.md` を読み、Goal の完了条件・commit slicing・Claude review・設計判断の扱いに従う。
  - 各 commit では `.agents/workflow/default.md` を使い、必要な phase workflow だけ読む。
  - Goal の途中状態、backlog の具体項目、review 結果、設計判断の記録は skill に書かず、適切な md（`backlog/backlog.md` / `decisions/` / `specs/`（あれば） / `rules/`）に同期する。
- **Acceptance**:
  - `.agents/workflow/goal.md` に従って Goal が進行している。
  - 各 commit が `.agents/workflow/default.md` を満たしている。
  - Goal 完了時に Claude review と必要な記録同期が済んでいる。
  - Goal 完了時に、設計判断がない場合も `設計判断: なし` と明示されている。
- **Relevant**:
  - `.agents/workflow/goal.md`
  - `.agents/workflow/default.md`
  - `claude-review-request` skill
  - `backlog/backlog.md`

## Flow

1. `.agents/workflow/goal.md` を読む。
2. Goal 開始時の base commit を記録する。
3. Goal を 1 commit 単位へ切り、各 commit で `.agents/workflow/default.md` を実行する。
4. Goal 完了前に、`.agents/workflow/goal.md` の Claude review 条件を満たす。
5. Goal 全体の結果、残リスク、設計判断をまとめる。設計判断がない場合も `設計判断: なし` と書く。
