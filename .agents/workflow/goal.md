# Goal Workflow

この workflow はこのプロジェクトの Goal 手順の正本。実装作業の発火入口は `goal-workflow` skill とし、`goal-workflow` skill はこのファイルを読んで進める。

## ICAR

- **Intent**: `/goal` で指定された目的を、複数の 1 commit workflow に分割して完了まで進める。
- **Constraints**:
  - 実装作業は `goal-workflow` skill を入口にし、この workflow を正本として読む。
  - `/goal` の呼び出し文は、原則として skill への参照と完了対象だけでよい。例: `/goal goal-workflow skill に従い、backlog/backlog.md の「v0.x」を完了して。`
  - Goal 開始時の `HEAD` を base commit として記録する。`base` は Goal 終了まで動かさず、Goal 全体の差分 `<base>..HEAD` と最終報告の起点にする。分割レビューの進捗は `review_cursor`（初期値 `base`）で別に持つ。ブランチは切らず、range で対象を表す。
  - 1 回の実装 workflow は 1 commit 単位に限る。Goal main は実装を直接担当せず、Goal が 1 commit だけで完了する場合も、次の 1 Change を選んで fresh Change worker を 1 つずつ直列起動する。
  - 各 commit は、Goal 全体の途中でも、その commit 単位では review / revert / bisect できる完了状態にする。
  - Goal 全体を 1 plan / 1 commit に押し込まない。次に扱う 1 commit 分を毎回明確に切り出す。
  - 複数案があるだけでは止まらない。現在の要求、`docs/rules/` / `docs/specs/` / `docs/decisions/`、コード、調査・検証結果から最善案を選んで進める。
  - Goal 中に、ユーザーが違う選択をする可能性がある重要な仕様・UX・設計上の選択が発生したら、適切に進められる範囲では採用案を選んで実装し、Goal 完了報告で `ユーザー判断が必要` として選択肢、主な利点・欠点、採用結果を提示する。なければ `ユーザー判断が必要: なし` と明示する。
  - 報告対象になる判断の書き方は `.agents/workflow/design-decision-record.md` を参考にする。
  - 進捗・完了の報告は、このセッションのツール結果で裏取りできる事実だけを書く。テストが失敗していれば出力ごと報告し、未検証の項目は未検証と明示する。
  - 後から制約になる判断、仕様変更、未着手作業は、画面出力だけで終わらせず `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` の適切な情報源へ同期する。
  - workflow の review とは別に、commit 済み range への Cross-Agent Review を Goal 完了条件に含める。Goal range に通常の Self Review / `change-review` 相当を再実行しない。
- **Acceptance**:
  - Goal の目的が満たされている。
  - 必要な commit がすべて作成されている。
  - 各 commit が `change/workflow.md` の workflow を満たしている。
  - Goal 開始時 base 以降の commit 済み内容が Cross-Agent Review 済み。レビュー上限に到達した場合は、最終修正が未レビューであることを含めて `レビュー上限超過` として報告されている。
  - 必要な仕様・backlog・判断記録が同期されている。
  - ユーザー判断が必要な項目の有無が完了時に明示されている。
  - 作業ツリーの残差分がない、または残す理由が明確。
- **Relevant**:
  - `goal-workflow` skill
  - `.agents/workflow/change/workflow.md`
  - `.agents/workflow/design-decision-record.md`
  - `cross-agent-review` skill（Goal Review を別系統の Claude Code に依頼する）
  - `backlog/backlog.md`
  - 関連する `docs/rules/`, `docs/specs/`, `docs/decisions/`, `llm-wiki/`（作業地図）

## Flow

1. Goal の目的、制約、完了条件を確認し、開始時の base commit を記録する。ブランチは切らない。
2. Goal を 1 commit 単位の候補へ分割する。
3. 次に扱う 1 commit 分を選び、fresh Change worker に渡す。Goal が 1 commit だけの場合も同じ。
4. commit 後、Goal の残りと Cross-Agent Review の実施タイミングを確認する。
5. 残りがあれば次の 1 commit 分に戻る。
6. 必要な Cross-Agent Review と対応が済んでいなければ実施する。
7. 完了または停止する時は、Goal 全体の結果、残リスク、ユーザー判断が必要な項目、レビュー上限超過の有無をまとめる。停止時は停止理由と解決すべきことが分かるようにする。

## Commit Slicing

- 1 commit に独立した複数作業を混ぜない。
- 1 commit は、単独で説明できるユーザー価値、仕様同期、リファクタ、テスト追加のいずれかに寄せる。
- 仕様同期と実装は、同じ変更の理解に必要なら同じ commit に含めてよい。
- 広いリファクタと振る舞い変更は、レビューしづらくなるなら分ける。
- 途中で 1 commit として不自然になったら、作業を広げず commit 単位を切り直す。
- Goal に必要な残作業は、次の Change として続けるか、別タスクが適切なら `backlog/backlog.md` に残す。どちらの場合も漏らさない。

## Change Worker

- Goal 経由の Change は、commit 数に関わらず、原則 fresh Change worker に渡す。
- Goal main は実装を直接担当しない。Goal main の責務は、base / review_cursor 管理、commit slicing、次の Change 選定、Cross-Agent Review、最終報告に限る。
- Goal main は次の 1 Change を選び、fresh Change worker を 1 つずつ直列起動する。同じ worktree で複数の Change worker を並行実行しない。Goal が 1 commit だけで完了する場合も Change worker を 1 つ起動する。
- Change worker は渡された Change だけを active scope とし、Goal 全体を再計画・再分割しない。
- 通常は `change/workflow.md` に従い、調査から commit まで完了して戻る。
- 1 commit として不自然だと分かった場合は、作業を広げず事実を Goal main に返す。Goal main が commit 単位を切り直す。
- 戻りの固定 schema は作らない。commit、検証、残作業、停止理由が理解できればよい。
- 直接実行の例外は Goal 経由の作業には適用しない。Goal を経由しない単発 Change だけは、現在の agent が直接実行してよい。
- Change worker は、独立委任が効率または品質を高める調査・実装補助・検証を必要に応じて下位 subagent に任せてよい。

## Final Report

- 完了時も停止時も、報告形式は状況に合わせて分かりやすく整える。固定テンプレートに無理に合わせない。
- `ユーザー判断が必要: なし` または必要な判断内容を必ず明示する。
- ユーザー判断が必要な項目は、複数の仕様・UX・設計選択肢があり、ユーザーが違う選択をする可能性があるものに限る。
- 既存 `docs/rules/` / `docs/specs/` / `backlog/` から自然に決まること、要求どおり実装しただけの内容、単なる実装上の判断、未実装 TODO は毎回の報告対象にしない。
- `レビュー上限超過: なし` または対象単位・回数・最後の指摘・行った修正・最終修正が未レビューであること・残リスクを状況に合わせて明示する。収束した review も、どの review が通ったかを状況に合わせて報告する。
- 停止時は、停止理由と解決すべきことが分かるようにする。

## Goal Review

- Goal Review は、通常の `change/review.md` とは別に Goal 完了条件として扱う。
- Goal Review は Cross-Agent Review だけを行う。各 commit の Self Review は `change/review.md` で完了済みとして扱い、Goal range に対して通常の Self Review / `change-review` 相当は再実行しない。
- レビュー対象は未コミット差分ではなく、未レビュー範囲の commit range とする（分割しない場合は `review_cursor == base`）。Cross-Agent Review の実行直前に `review_start = review_cursor`、`review_end = 現在の HEAD の実 SHA` を確定し、1 回の review 中は `<review_start>..<review_end>` を動かさない。ブランチは切らないので range で対象を表す。
- 1 commit ごとではなく、関連する数 commit をまとめてレビューする。毎回でなくてよい。PASS 相当なら `review_cursor` を `review_end` まで進める（`base` は動かさない）。
- 差分が大きい、または永続化 / 同期 / 外部 API / 広い UI 挙動に触れる場合は、数 commit を待たずにその時点までの range で早めにレビューする。
- 指摘対応は別 commit として作成し、対応 commit を含む range で再レビューする。follow-up review でも実行直前に新しい `review_end` を取り直す。
- 各レビュー単位につき reviewer を呼ぶ回数は、初回を含めて合計最大 3 回。`Review 1 -> Fix 1 -> Review 2 -> Fix 2 -> Review 3 -> Fix 3` まで行ったら Review 4 は行わない。Review 3 後の Fix 3 は未レビューの最終修正になるため、同じ review 単位を上限到達として打ち切り、Goal 作業は続ける。
- `cross-agent-review` skill にレビュー対象の commit range `<review_start>..<review_end>` を渡して実行する。修正は Codex 側が行い、Claude は外部レビュアーとして指摘を返す。

## Stop Conditions

- Goal の完了条件が曖昧で、1 commit 単位へ切れない。
- 次の commit が、その時点の情報では適切に決められない重要な仕様・UX・データ保持・削除方針に依存しており、ユーザー判断や不足情報なしに進めること自体が不適切。
- Goal の途中で、現在の目的と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している。
- 必須の検証を代替手段でも裏付けられず、完了扱いにできない。
- Cross-Agent Review を完全に実施できない。
