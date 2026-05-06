# Goal Loop

`backlog/goal.md` から呼ばれる1ループぶんの手順。1ループ = 次のいずれか1つ。

- **A**: 全体ゴール内の1タスクを実装して完了させる
- **B**: タスクを整理する（追加・分解・統合）
- **C**: タスクをスキップする

## Acceptance

次のどれかで1ループ完了とする。

- A: タスクの実装・検証・`- [x]` 更新と goal-state.md / goal-decisions.md 更新を 1 コミットで作成した
- B: backlog の整理と goal-state.md / goal-decisions.md 更新を 1 コミットで作成した
- C: goal-state.md のスキップ記録と goal-decisions.md 追記を 1 コミットで作成した
- ユーザー確認が絶対に必要なため `goal-blocked.md` を作成した
- `backlog/goal.md` の Acceptance を満たしたため `goal-done.md` を作成した

## Relevant

- 全体ゴール: `backlog/goal.md`
- タスク台帳: `backlog/backlog.md`
- 状態（前回ループの結果）: `backlog/goal-state.md`
- 判断ログ（過去の自動判断）: `backlog/goal-decisions.md`
- 設計ルール: `rules/architecture.md` `rules/03-data-model.md` `.claude/rules/conventions.md` `.claude/rules/testing.md`
- 検証コマンド: `go test ./...` `go vet ./...` `go build ./...`

## Constraints

### Task selection

タスクを選ぶ前に `goal-state.md` のスキップ記録を確認する。再開条件を満たしていないスキップ済みタスクは選ばない。

原則は全体ゴールの範囲内の未完了タスクを上から順に選ぶ。

ただし、次の場合は順序を変えてよい。

- 依存関係上、別タスクを先にやる方が自然
- 今やると大きくなりすぎる
- 今はやらない方がよい（→ Skip へ）
- 先に分解した方が安全（→ Task rewrite へ）

順序変更した場合は `goal-decisions.md` に理由を書く。

### Task rewrite

タスクは必要なら分解・追加・統合してよい（B パス）。

- 元タスクの意図を失わない
- 新しい未完了タスクは全体ゴールの範囲内に残す
- 分解した元タスクは「分解完了」として `- [x]` にしてよい
- 整理した内容は `backlog/backlog.md` に反映してコミットする
- 分解・追加・統合の理由を `goal-decisions.md` に書く

### Skip

今やらない方がよいタスクはスキップしてよい（C パス）。

- `goal-state.md` にスキップしたタスク・スキップ理由・再開条件を書く
- スキップだけで1ループを終えてよい
- 範囲内の全タスクがスキップ対象になり進められない場合は `goal-blocked.md` を作成して停止する

### No Plan Mode

`EnterPlanMode` を使わない。計画が必要な場合は内部で短く立ててそのまま実装する。ユーザー承認が絶対に必要な場合のみ `goal-blocked.md` を作成して停止する。

## A. タスクを進める

1. 全体ゴール内の未完了タスクを1つ選ぶ（原則は上から順）
2. 選んだタスクを「ユーザー依頼」として `.claude/goal/workflow/default.md` に従って実装・検証・レビューまで進める
3. `finish.md`（コミット）に入る**前**に `goal-state.md` を更新し、必要なら `goal-decisions.md` に追記する
4. `finish.md` でコミットする（`backlog/backlog.md` の `- [x]` 更新と goal-state.md / goal-decisions.md の更新が同じコミットに入る）

`.claude/goal/workflow/default.md` 系列の workflow（`plan.md` `implement.md` `verify.md` `review.md` `finish.md` 等）が、タスク完了までの調査・計画・実装・検証・コミット・`backlog/backlog.md` の `- [x]` 更新を担う。

整理と実装は同じループでやらない。実装中にタスクの不整合に気づいた場合は、そのループは元タスクを完了させるか中断し、次ループで B パスに入る。

## B. タスクを整理する

実装に入る前にタスクの追加・分解・統合が必要だと判断した場合のみ。

1. `backlog/backlog.md` を書き換える
2. `goal-state.md` を更新する
3. 整理理由を `goal-decisions.md` に書く
4. 整理内容を `commit` スキルでコミットする（backlog.md, goal-state.md, goal-decisions.md が同じコミットに入る）

次ループで実装（A パス）に入る。

## C. タスクをスキップする

今やらない方がよいタスクが先頭にある場合のみ。

1. `goal-state.md` にスキップしたタスク・理由・再開条件を書く
2. スキップ理由を `goal-decisions.md` に追記する
3. `commit` スキルでコミットする（goal-state.md と goal-decisions.md の更新を残すため）

次ループで別のタスクを選ぶ。
