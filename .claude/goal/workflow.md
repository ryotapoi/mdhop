# Workflow: execute-goal

Goal runner（`.claude/goal/runner.sh`）から `claude -p` で呼ばれる時の **共通メカニクス**。

このファイルにはゴール固有の内容は書かない。全体目標は `backlog/goal.md` に、1ループの手順は `backlog/goal-loop.md` に書く。

このワークフローは **無人ループ専用** で、`.claude/workflow/default.md` 系列とは別系統。runner から呼ばれた時はこのファイルと `backlog/goal.md` を読み、`.claude/workflow/` 配下は読まない（goal 用の派生は `.claude/goal/workflow/` 配下にコピーされている）。`backlog/goal.md` の指示に従って `backlog/goal-loop.md` 等の関連ファイルへ進む。

## Files

- `backlog/goal.md` — 全体目標。Intent / Acceptance / Procedure / Relevant / Constraints / Halt。**ゴールごとに書き直す**
- `backlog/goal-loop.md` — `goal.md` から呼ばれる1ループぶんの手順。**ゴールごとに書き直す**（流用前提）
- `backlog/goal-state.md` — 途中状態。各ループで更新する
- `backlog/goal-decisions.md` — ユーザー意図と違う可能性がある自動判断のログ
- `backlog/goal-blocked.md` — ユーザー確認必須で停止する時に作る（通常は存在しない）
- `backlog/goal-stop.md` — 強制中断用。ユーザーが手動で作る（通常は存在しない）
- `backlog/goal-done.md` — `goal.md` の Acceptance を満たした時に作る（通常は存在しない）

## Entry

runner から呼ばれたら、`backlog/goal.md` を読んで指示に従う。`goal.md` から `goal-loop.md` 等の関連ファイルへ辿る。

`backlog/goal-state.md` と `backlog/goal-decisions.md` は前回からの引き継ぎ情報として読む。

## No Plan Mode

このワークフローでは **`EnterPlanMode` を使わない**。

理由: Plan Mode は人間確認 UI を出す仕組みで、`claude -p` の非対話セッションと相性が悪い。承認待ちで止まる。

代わりに、計画が必要な場合は次のようにする。

- 短い実行計画を内部で立てて、そのまま実装する
- ユーザー承認が絶対に必要な場合のみ `goal-blocked.md` を作成して停止する

## State update

各ループの最後に `goal-state.md` を更新する。書く内容は次のとおり。

- status（idle / running / blocked / done）
- 今回のループで何をしたか
- 次のループへの引き継ぎヒント
- スキップしたタスクがあれば、その内容と再開条件

## Task SSoT

タスクは `backlog/backlog.md` にしか存在しない。`goal-decisions.md` に「別タスクとして切る」と書いただけでは、そのタスクは存在しないものとして扱われる。

派生タスク（実装中に発見した未完了の作業、設計レビューで「今回スコープ外」と判断した別案、横断対応に持ち越す指摘など）が出たら、必ず `backlog/backlog.md` の該当バージョンセクションへ追記する。追記は派生タスクを生んだループのコミットに含める（次ループに先送りしない）。

`goal-decisions.md` はあくまで「判断の記録」で、タスク台帳ではない。判断と派生タスクは両方書く。

## Decision logging

ユーザー意図と違う可能性がある自動判断は `goal-decisions.md` に追記する。

書く対象は次のものだけ。

- ユーザーが明示していない仕様解釈
- 複数案があり得る設計選択
- タスクの分解
- タスクの順序変更
- タスクのスキップ
- 将来レビューで「その判断は違う」と言われる可能性があるもの

通常の実装メモや作業ログは書かない。

## Blocking

次の場合だけ `goal-blocked.md` を作成して停止する。

- ユーザー確認なしに進めると破壊的・不可逆な変更になる
- 外部サービス、課金、公開、deploy、push が必要
- `goal.md` または `goal-loop.md` の指示と矛盾する可能性が高い
- 自動判断すると取り返しがつかない
- `goal.md` の Intent や `goal-loop.md` の A/B/C が空・不明瞭で、解釈が複数通り成り立つ

迷うだけなら停止しない。自動判断し、`goal-decisions.md` に記録する。

## Done

`goal.md` の Acceptance を満たしたら `goal-done.md` を作成する。runner はこのファイルを見て終了する。

何をもって達成とするかは `goal.md` の Acceptance セクションに書く。

## Forbidden

ゴール非依存の禁止事項。

- `git push`
- deploy / 公開
- 外部サービスへの破壊的操作
- 完了していないタスクを `- [x]` にすること
- `EnterPlanMode` の使用
- ユーザーへの質問

## Reuse from existing rules

実装の詳細ルールは既存ファイルを参照してよい（読むのは必要な時だけ）。

- 実装規約: `.claude/rules/conventions.md`
- テスト方針: `.claude/rules/testing.md`
- 調査ルール: `.claude/rules/investigation.md`
- ドキュメント方針: `.claude/rules/docs.md`
- アーキテクチャ: `rules/architecture.md`
- データモデル: `rules/03-data-model.md`

ただし `.claude/workflow/default.md` および `.claude/workflow/*.md` の各 phase ファイルは **読まない**。それらは人間相手の対話フロー用。goal 用の派生は `.claude/goal/workflow/` 配下にコピーされており、A パスではこちらを使う。
