# Goal Workflow

この workflow はこのプロジェクトの Goal 手順の正本。実装作業の発火入口は `goal-workflow` skill とし、`goal-workflow` skill はこのファイルを読んで進める。

## Intent

`/goal` で指定された目的を、複数の 1 commit workflow に分割して完了まで進める。

## Constraints

- 実装作業は `goal-workflow` skill を入口にし、この workflow を正本として読む。
- `/goal` の呼び出し文は、原則として skill への参照と完了対象だけでよい。例: `/goal goal-workflow skill に従い、backlog/backlog.md の「v0.x」を完了して。`
- ブランチは切らず、いるブランチ（通常 main）上にそのまま 1 commit ずつ積む。Goal 開始時の `HEAD` を base SHA として記録する（Goal Review の range 起点）。
- 1 回の実装 workflow は 1 commit 単位に限る。Goal が複数 commit を必要とする場合は、Goal main が次の 1 Change を選び、fresh subagent を Change worker として 1 つずつ直列起動する。
- 各 commit は、Goal 全体の途中でも、その commit 単位では review / revert / bisect できる完了状態にする。
- Goal 全体を 1 plan / 1 commit に押し込まない。次に扱う 1 commit 分を毎回明確に切り出す。
- Goal 前提では都度のユーザー確認を避け、自動進行する。止まるのは Stop Conditions に該当する場合だけ。
- plan mode（`EnterPlanMode` / `ExitPlanMode`）は使わない。承認待ちが自動進行と噛み合わないため。計画が必要な場合は内部で立ててそのまま実装する。詳細は `change/plan.md`。
- 複数案があるだけでは止まらない。現在の要求、`docs/rules/` / `docs/specs/` / `docs/decisions/`、コード、調査・検証結果から最善案を選んで進める。
- Goal 中に、ユーザーが違う選択をする可能性がある重要な仕様・UX・設計上の選択が発生したら、適切に進められる範囲では採用案を選んで実装し、Goal 完了報告で `ユーザー判断が必要` として選択肢、主な利点・欠点、採用結果を提示する。なければ `ユーザー判断が必要: なし` と明示する。
- 進捗・完了の報告は、このセッションのツール結果で裏取りできる事実だけを書く。テストが失敗していれば出力ごと報告し、未検証の項目は未検証と明示する。
- 後から制約になる判断、仕様変更、未着手作業は、画面出力だけで終わらせず `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` の適切な情報源へ同期する。
- 各 commit 内の Self Review とは別に、Goal の commit range に対する Cross-Agent Review（Codex レビュー / `cross-agent-review`）を Goal 完了条件に含める（Goal Review 参照）。Goal range に `/code-review` 観点ベースの Self Review を再実行しない。

## Acceptance

- Goal の目的が満たされている。
- 必要な commit がすべて作成されている。
- 各 commit が `change/workflow.md` の workflow を満たしている。
- Goal の commit range（`<base>..HEAD`）が Cross-Agent Review（Codex レビュー）を通過している。レビュー上限に到達した場合は、最終修正が未レビューであることを含めて `レビュー上限超過` として報告されている。
- 必要な仕様・backlog・判断記録が同期されている。
- ユーザー判断が必要な項目の有無が完了時に明示されている。
- 作業ツリーの残差分がない、または残す理由が明確。

## Relevant

- `goal-workflow` skill
- `.claude/workflow/change/workflow.md`
- `design-decision` skill
- `cross-agent-review` skill
- `backlog/backlog.md`

## Flow

1. Goal の目的、制約、完了条件を確認し、ブランチは切らず開始時の `HEAD` を base SHA として記録する。
2. Goal を 1 commit 単位の候補へ分割する（Commit Slicing 参照）。
3. 次に扱う 1 commit 分を選び、Change worker に渡す。単発 Change は現在の agent が直接 `change/workflow.md` を実行してよい。
4. commit 後、Goal の残りと Goal Review の実施タイミングを確認する。
5. 残りがあれば次の 1 commit 分に戻る。
6. 必要な Goal Review（Cross-Agent Review: Codex）と対応が済んでいなければ実施する。実行直前の `HEAD` を `review_end` として固定し、PASS 相当なら `review_cursor` を `review_end` まで進めてよい（`base` は動かさない）。
7. 完了または停止する時は、Goal 全体の結果、残リスク、ユーザー判断が必要な項目、レビュー上限超過の有無をまとめる。停止時は停止理由と解決すべきことが分かるようにする。

## Branch

- ブランチは切らない。いるブランチ（通常 main）上にそのまま 1 commit ずつ積む。
- Goal 開始時の `HEAD` を base SHA として記録する。`base` は Goal 終了まで動かさない。Goal 全体の差分は `<base>..HEAD` で表し、最終報告と全体俯瞰の起点になる。
- 分割レビューの進捗は `review_cursor` で持つ。初期値は `base`。レビューが済むたびにレビュー済みの commit まで `review_cursor` を進める。次の分割レビューでは実行直前の `HEAD` を `review_end` として固定し、対象を `<review_cursor>..<review_end>` にする。`base` と `review_cursor` を混同しない（全体差分は常に `base` 起点、未レビュー差分は `review_cursor` 起点）。
- merge 操作はない。Goal 完了後もそのままブランチ上に commit が残る。
- 履歴は線形に保ち、各 commit を単独で revert / bisect できる状態に残す。

## Commit Slicing

- 1 commit に独立した複数作業を混ぜない。
- 1 commit は、単独で説明できるユーザー価値、仕様同期、リファクタ、テスト追加のいずれかに寄せる。
- 仕様同期と実装は、同じ変更の理解に必要なら同じ commit に含めてよい。
- 広いリファクタと振る舞い変更は、レビューしづらくなるなら分ける。リファクタを先に commit してから振る舞い変更を別 commit にする。
- 途中で 1 commit として不自然になったら、作業を広げず commit 単位を切り直す。
- Goal に必要な残作業は、次の Change として続けるか、別タスクが適切なら `backlog/backlog.md` に残す。どちらの場合も漏らさない。

## Change Worker

- Goal main は次の 1 Change を選び、fresh subagent を Change worker として 1 つずつ直列起動する。同じ worktree で複数の Change worker を並行実行しない。
- Change worker は渡された Change だけを担当し、Goal 全体を再計画・再分割しない。
- 通常は `change/workflow.md` に従い、調査から commit まで完了して戻る。
- 1 commit として不自然だと分かった場合は、作業を広げず事実を Goal main に返す。Goal main が commit 単位を切り直す。
- 戻りの固定 schema は作らない。commit、検証、残作業、停止理由が理解できればよい。
- 単発 Change は現在の agent が直接実行してよい。

## Goal Review

各 commit 内の Self Review とは別に、Goal の commit range を対象に Cross-Agent Review を Goal 完了条件として実施する。ブランチは切らないので、レビュー range は commit range で表す。分割レビューの未レビュー対象は実行直前に固定する `<review_cursor>..<review_end>`、Goal 全体の差分は `<base>..HEAD`。Goal range に対して `/code-review` 観点ベースの Self Review は再実行しない。

- **Codex レビュー（必須）**: `cross-agent-review` skill を未レビュー range 対象で実行する。実行直前に `review_start = review_cursor`、`review_end = 現在の HEAD の実 SHA` を確定し、1 回の review 中は `<review_start>..<review_end>` を動かさない。別系統モデル（Codex）に未レビュー差分を見せる。
- **分割レビュー**: 一気に全部ではなく、適当なコミットのまとまりごとにレビューしてよい（毎回でなくてよい）。PASS 相当なら `review_cursor` を `review_end` まで進める（`base` は動かさない）。次のレビューは新しい `review_cursor` から、実行直前に新しい終点 SHA を取り直す。
- 1 commit ごとではなく、関連する数 commit をまとめてレビューする。
- 差分が大きい、または永続化 / 同期 / 外部 API / 広い UI 挙動に触れる場合は、数 commit を待たずにその時点までの commit range で早めにレビューする。
- 指摘対応は別 commit として作成し、対応 commit を含む range で再レビューする。follow-up review でも実行直前に新しい `review_end` を取り直す。
- 各レビュー単位につき reviewer を呼ぶ回数は、初回を含めて合計最大 3 回。`Review 1 -> Fix 1 -> Review 2 -> Fix 2 -> Review 3 -> Fix 3` まで行ったら Review 4 は行わない。Review 3 後の Fix 3 は未レビューの最終修正になるため、同じ review 単位を上限到達として打ち切り、Goal 作業は続ける。

## Final Report

- 完了時も停止時も、報告形式は状況に合わせて分かりやすく整える。固定テンプレートに無理に合わせない。
- `ユーザー判断が必要: なし` または必要な判断内容を必ず明示する。
- ユーザー判断が必要な項目は、複数の仕様・UX・設計選択肢があり、ユーザーが違う選択をする可能性があるものに限る。
- 既存 `docs/rules/` / `docs/specs/` / `backlog/` から自然に決まること、要求どおり実装しただけの内容、単なる実装上の判断、未実装 TODO は毎回の報告対象にしない。
- `レビュー上限超過: なし` または対象単位・回数・最後の指摘・行った修正・最終修正が未レビューであること・残リスクを状況に合わせて明示する。収束した review も、どの review が通ったかを状況に合わせて報告する。
- 停止時は、停止理由と解決すべきことが分かるようにする。

## Stop Conditions

- Goal の完了条件が曖昧で、1 commit 単位へ切れない。
- 次の commit が、その時点の情報では適切に決められない重要な仕様・UX・データ保持・削除方針に依存しており、ユーザー判断や不足情報なしに進めること自体が不適切。
- Goal の途中で、現在の目的と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している。
- 必須の検証を代替手段でも裏付けられず、完了扱いにできない。
- Cross-Agent Review を完全に実施できない。
