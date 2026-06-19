# Finish Workflow

## ICAR

- **Intent**: review を通過した変更を、コミットまで含めて完了状態にする。
- **Constraints**:
  - コミットは `commit` スキルで作成する。
  - 文書同期（backlog / decisions / references / specs）、ADR 作成、コミットメッセージ規約は `commit` スキル側が判断する。
  - ユーザーがコミット前確認を求めている場合は止まる。
  - Goal 実行中の場合、commit 後に Goal 全体が完了したか、次の 1 commit workflow に進むかを確認する。
  - Goal 完了報告では、設計判断がない場合も `設計判断: なし` と明示する。
- **Acceptance**:
  - コミット済みで、作業ツリーの残差分が意図したものだけ。
  - Goal が継続中の場合は、次の workflow に進む前に残タスクと次の 1 commit 単位を確認できる。
  - Goal が完了する場合は、設計判断の有無が明示されている。
- **Relevant**:
  - 変更差分
  - 検証結果
  - review 結果
  - `agents/skills/commit/SKILL.md`
