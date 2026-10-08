---
status: current
---

# ADR 0028: move の共通 batch 処理と directory delete

## 背景

directory move を逐次的な単一ファイル move に分けると、中間状態ごとに basename の曖昧性が変わり、最終状態に対して不正な書き換えが生じる。ADR 0009 の最終状態で処理する判断は維持するが、Move と MoveDir の独立した制御フローと CLI だけの directory delete 展開は現構造に合わせて置き換える。

## 判断と理由

- move の集合を先に確定し、全ファイルが移動した後の解決 map を使って incoming / collateral / outgoing の書き換えを準備する。逐次 Move 呼び出しで中間状態に基づく書き換えを行わない。
- 単一 Move と MoveDir は入口の検証と対象収集を分け、書き換え・disk 操作・DB transaction は共通の batch 実行処理で扱う。単一 Move も一件の batch とする。最終状態の意味は共通であり、書き換えと失敗処理の修正を別々の制御フローへ同期する負担を避ける。
- 外部 note と移動 note の書き換え候補を disk 副作用の前に準備し、batch 全体で disk move と DB 更新を行う。失敗時は rename と内容を戻し、rollback 自体の失敗も報告する。disk と SQLite の操作を一つの transaction とみなす保証はしない。
- directory delete の対象展開は core の Delete に置く。登録 note / asset の集合を core 内で展開して削除対象を検証し、`--rm` では登録ファイルの disk 削除と DB 更新を行う。その後の directory cleanup で残る non-Markdown file と空 directory を処理する。CLI だけで展開する別経路を持たず、core の呼び出しにも同じ対象処理を適用するためである。

## 帰結と範囲

batch 内で両端が移動するリンクも最終状態に対して扱える。書き換えと rollback は共通処理に集まり、独立アルゴリズムの同期を不要にする。directory delete の登録ファイル・DB 更新後に cleanup が失敗した場合、既に完了した更新と区別してエラーを報告する。asset の登録と disk-only file の扱いは ADR 0011 の判断を維持する。
