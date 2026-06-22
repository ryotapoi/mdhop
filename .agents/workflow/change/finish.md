# Finish Workflow

## ICAR

- **Intent**: review を通過した変更を、コミットまで含めて完了状態にする。
- **Constraints**:
  - コミットは `commit` スキルで作成する。
  - finish では tracked file の内容を追加・変更・削除しない。文書同期（`backlog/backlog.md` / `docs/decisions/` / `llm-wiki/` / `docs/specs/`）や ADR が不足していると分かった場合は、commit せず `change/implement.md` に戻り、verify と review をやり直す。
  - コミットメッセージ規約は `commit` スキル側が判断する。
  - ユーザーがコミット前確認を求めている場合は止まる。
  - Goal 実行中の場合、commit 後に Goal 全体が完了したか、次の 1 commit workflow に進むかを確認する。
  - Goal 完了報告では、`ユーザー判断が必要` と `レビュー上限超過` の有無を明示する。
- **Acceptance**:
  - コミット済みで、作業ツリーの残差分が意図したものだけ。
  - Goal が継続中の場合は、次の workflow に進む前に残タスクと次の 1 commit 単位を確認できる。
  - Goal が完了する場合は、ユーザー判断が必要な事項とレビュー上限超過の有無が明示されている。
- **Relevant**:
  - 変更差分
  - 検証結果
  - review 結果
  - `.agents/skills/commit/SKILL.md`
