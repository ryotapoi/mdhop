# ADR 0013: Goal Runner — Headless Loop Architecture

## Status

Accepted

## Context

複数タスクからなる「全体目標」（例: backlog v0.6.1 の全タスク完了）を、`claude -p` で1ループずつ無人実行したい需要が出た。

既存の `.claude/workflow/` 系は人間相手の対話フロー前提で、`EnterPlanMode` でユーザー承認を取る、`AskUserQuestion` で迷ったら聞く、`git status` clean 後はユーザー指示待ち、といった「対話を前提とした停止点」が随所にある。これを `claude -p` の非対話セッションでそのまま動かすと、停止点で詰まる。

無人ループには次の特性が必要:

- 各セッションは独立（1セッション = 1ループぶんの進捗）
- ユーザー確認なしで自動判断する範囲を明確化
- ユーザー意図と違う可能性がある判断は記録（後でまとめてレビュー）
- 確認が絶対に必要な案件は明示的に停止（runner が検知できる形で）
- レビュー周回が無限にならない（最大ループ数）

## Considered Options

- **選択肢A: `.claude/workflow/` を改修して auto mode 分岐を入れる**
  - 1系統で済む
  - default.md / plan.md / review.md などの条件分岐が増えて複雑化
  - 対話モード時の挙動への影響リスク
- **選択肢B: 完全派生の goal 系統を別ディレクトリに用意する**
  - 対話モードと無人モードを完全分離
  - workflow ファイルを2系統メンテする必要がある
  - 各 phase の Plan Mode 廃止やユーザー確認削除を独立して書ける
- **選択肢C: `/loop` skill で代替**
  - 同一セッション内でのプロンプト再投入のみで、コンテキスト分離ができない
  - 1ループごとに会話履歴が積み上がり、自動 compaction で過去の判断が曖昧になる
  - 「無人ループ」の本質要件（独立セッション）を満たさない

## Decision

We will adopt option B: 無人ループ専用の派生 workflow 系統を `.claude/goal/` に新設する。

具体構成:

- `.claude/goal/runner.sh` — bash でループ実行、stop/blocked/done フラグ判定のみ
- `.claude/goal/workflow.md` — 共通メカニクス（読むファイル順、No Plan Mode、State update、Blocking、Done、Forbidden）
- `.claude/goal/workflow/*.md` — 個別 phase（`.claude/workflow/` から派生）
- `backlog/goal.md` — 全体目標の ICAR（Intent / Acceptance / Relevant / Constraints）
- `backlog/goal-loop.md` — 1ループの手順（A: タスク実装 / B: タスク整理 / C: スキップ）
- `backlog/goal-state.md` — ループ間の引き継ぎ状態（コミット対象）
- `backlog/goal-decisions.md` — ユーザー意図と違う可能性がある自動判断のログ（コミット対象）
- ランタイムフラグ（`goal-blocked.md` / `goal-done.md` / `goal-stop.md` / `goal-plan.md`）は `.gitignore` 対象

主要な設計判断:

- **Plan Mode 廃止**: `EnterPlanMode` / `ExitPlanMode` は使わず、コンテキスト上で計画を立ててそのまま実装に進む
- **最大2周レビュー**: 周回が無限にならないよう、レビューループは最大2周で打ち切り、重大指摘が残れば `goal-blocked.md` 作成
- **Codex 系 skill は呼ばない**: 無人ループでの安定性を優先（Codex の応答揺れ回避）
- **ユーザー確認案件は `goal-blocked.md` 作成で停止**: runner が次ループに入らない停止点として機能
- **state / decisions はコミット対象**: 履歴として残し、ゴール達成後にユーザーがまとめて確認できる
- **ランタイムフラグは `.gitignore` 対象**: 一時的な停止フラグなのでリポジトリに含めない

## Consequences

容易になること:

- 複数タスクからなる全体目標を `claude -p` で無人実行できる
- 各ループのコンテキストが独立するため、自動 compaction による判断履歴の劣化を回避できる
- ユーザー意図と違う可能性がある判断が `goal-decisions.md` に集約され、後でまとめてレビューできる
- Plan Mode を使わない無人実行と、Plan Mode を使う対話実行が共存できる

困難になること:

- workflow ファイル群（default.md / plan.md / implement.md / verify.md / review.md / finish.md / investigate.md）を2系統メンテする必要がある
- `.claude/workflow/` を更新したら `.claude/goal/workflow/` への反映を検討する手間が発生
- 無人ループで動かすには、呼ばれる skill 群が `~/.claude/settings.json` および `mdhop/.claude/settings.local.json` で許可されている必要がある（goal 用の追加許可は別途実施済み）

中立的な影響:

- `.claude/settings.local.json` は `.gitignore` 対象のため、`Skill(commit)` 等の project 側許可は他環境に配布されない（個人マシン前提の運用）
- runner は `claude -p` をそのまま叩くため、permission mode は `~/.claude/settings.json` の `defaultMode` に従う（現在は `auto`）
