# Finish Workflow

## ICAR

- **Intent**: Gatekeeper（Small では Conductor）を通過した変更を、コミットまで含めて完了状態にする。
- **Constraints**:
  - コミットは `commit` スキルで作成する。
  - Goal では Conductor だけが commit を実行する。Implementer と Gatekeeper は git 書き込みをしない。Conductor は Gatekeeper の baseline SHA、status、diff stat / hash、test exit code を機械照合し、test 後に status / hash を再照合する。Small の直接 diff 照合は限定例外とする。
  - finish では tracked file の内容を追加・変更・削除しない。文書同期（`backlog/backlog.md` / `docs/decisions/` / `llm-wiki/` / `docs/specs/`）や ADR が不足していると分かった場合は、commit せず Conductor 経由で Implementer に戻し、verify と Gatekeeper review をやり直す。
  - commit 前に差分、review 結果、Product Decision Ledger、同期済み docs を照合する。product decision（UX・データ意味・cross-surface 等。カテゴリ一覧は同ファイル）について `.agents/workflow/design-decision-record.md` の基準で採用案・別案・理由を追えない場合は commit せず `change/implement.md` に戻る。
  - コミットメッセージ規約は `commit` スキル側が判断する。
  - ユーザーがコミット前確認を求めている場合は止まる。
  - Goal 実行中の場合、commit 後に Goal 全体が完了したか、次の 1 commit workflow に進むかを確認する。
  - Goal 完了報告では、`ユーザー判断が必要` と Goal Review の `レビュー上限超過` の有無（reviewer ごと）を明示する。Goal Review MUST は `goal.md` の 7 分類で、その Gatekeeper 手続きのすり抜けを記録する。
- **Acceptance**:
  - コミット済みで、作業ツリーの残差分が意図したものだけ。
  - Goal が継続中の場合は、次の workflow に進む前に残タスクと次の 1 commit 単位を確認できる。
  - Goal が完了する場合は、記憶ではなく Product Decision Ledger / review 結果 / 同期済み docs から、ユーザー判断が必要な事項と Goal Review のレビュー上限超過の有無が明示されている。
- **Relevant**:
  - 変更差分
  - 検証結果
  - review 結果
  - global `commit` skill
