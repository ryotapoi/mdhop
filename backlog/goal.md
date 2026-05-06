# Goal

## Intent

`backlog/backlog.md` の v0.6.1 の全タスクを完了する。

v0.6.1 は内部リファクタ（ノード型定数化、format.go 分割、query.go 分割）が中心で、外部仕様への影響はない。

## Acceptance

`backlog/backlog.md` の v0.6.1 セクション内に未完了タスク（`- [ ]`）がなくなったら達成。達成したら `goal-done.md` を作成する。

## Procedure

最終目標の達成までは、**1セッションで `backlog/goal-loop.md` を1回だけ実行する**。runner が複数セッションを繰り返すことで全体目標へ収束する。

1セッションの中で複数ループを回さない（runner との接続が崩れる）。

## Relevant

- タスク管理: `backlog/backlog.md` の v0.6.1 セクション（各タスクに該当ファイルが書かれている）
- 1ループの手順: `backlog/goal-loop.md`

## Constraints

- v0.6.1 の範囲外の unrelated refactor はやらない
- 外部仕様（CLI コマンド・出力フォーマット）に影響する変更はこのゴールでは避ける
