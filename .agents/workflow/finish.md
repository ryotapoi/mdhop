# Finish Workflow

## ICAR

- **Intent**: review を通過した変更を、コミットまで含めて完了状態にする。
- **Constraints**:
  - コミットは `commit` スキルで作成する。
  - 文書同期（backlog / decisions / references / specs）、ADR 作成、コミットメッセージ規約は `commit` スキル側が判断する。
  - ユーザーがコミット前確認を求めている場合は止まる。
- **Acceptance**:
  - コミット済みで、作業ツリーの残差分が意図したものだけ。
  - コミット完了後は次のタスクに進まない。
- **Relevant**:
  - 変更差分
  - 検証結果
  - review 結果
  - `.agents/skills/commit/SKILL.md`
