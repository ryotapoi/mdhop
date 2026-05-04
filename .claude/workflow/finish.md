# Finish

## Intent

review を通過した変更を、コミットまで含めて完了状態にする。

## Inputs

- 変更差分
- 検証結果
- review 結果

## Decision Criteria

- コミットは `commit` スキルで作成する。文書同期（backlog / decisions / references / specs）、ADR 作成、コミットメッセージ規約は `commit` スキル側が判断する
- このファイルでは commit スキルを呼ぶこと自体を担保する

## Acceptance

- コミット済みで、作業ツリーの残差分が意図したものだけ
- コミット完了後は次のタスクに進まない（ユーザー指示待ち）

## Stop Conditions

- ユーザーがコミット前確認を求めている
