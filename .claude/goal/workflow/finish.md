# Finish (goal)

## Intent

review を通過した変更を、コミットまで含めて完了状態にする。

## Inputs

- 変更差分
- 検証結果
- review 結果

## State files first

`commit` スキルは `git status` クリーンを Acceptance にしているため、コミット前に goal-loop の状態ファイルを更新しておく：

- `backlog/goal-state.md` を今ループの結果で更新する
- 必要なら `backlog/goal-decisions.md` に追記する

これら state ファイルは backlog/backlog.md の `- [x]` 更新と同じコミットに入る。

`backlog/goal-blocked.md` `goal-done.md` `goal-stop.md` `goal-plan.md` は `.gitignore` 対象なのでコミットされない。

## Decision Criteria

- コミットは `commit` スキルで作成する。文書同期（backlog / decisions / references / specs）、ADR 作成、コミットメッセージ規約は `commit` スキル側が判断する
- このファイルでは「state 更新 → commit スキル呼び出し」の順を担保する

## Acceptance

- コミット済みで、作業ツリーの残差分が `.gitignore` 対象（goal-blocked.md 等）のみ

## Stop Conditions

次の場合は `goal-blocked.md` を作成して停止する。

- コミット時に commit スキルが失敗した（手動コミットで代替しない。原因究明が必要）
- 残差分の判断が自動でつかない
