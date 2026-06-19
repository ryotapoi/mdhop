# Goal Workflow

この workflow はこのプロジェクトの Goal 手順の正本。実装作業の発火入口は `goal-workflow` skill とし、`goal-workflow` skill はこのファイルを読んで進める。

## Intent

`/goal` で指定された目的を、複数の 1 commit workflow に分割して完了まで進める。

## Constraints

- 実装作業は `goal-workflow` skill を入口にし、この workflow を正本として読む。
- `/goal` の呼び出し文は、原則として skill への参照と完了対象だけでよい。例: `/goal goal-workflow skill に従い、backlog/backlog.md の「v0.x」を完了して。`
- ブランチは切らず、いるブランチ（通常 main）上にそのまま 1 commit ずつ積む。Goal 開始時の `HEAD` を base SHA として記録する（Goal Review の range 起点）。
- 1 回の実装 workflow は 1 commit 単位に限る。Goal が複数 commit を必要とする場合は、`default.md` の workflow を commit ごとに繰り返す。
- 各 commit は、Goal 全体の途中でも、その commit 単位では review / revert / bisect できる完了状態にする。
- Goal 全体を 1 plan / 1 commit に押し込まない。次に扱う 1 commit 分を毎回明確に切り出す。
- Goal 前提では都度のユーザー確認を避け、自動進行する。止まるのは Stop Conditions に該当する場合だけ。
- plan mode（`EnterPlanMode` / `ExitPlanMode`）は使わない。承認待ちが自動進行と噛み合わないため。計画が必要な場合は内部で立ててそのまま実装する。詳細は `plan.md`。
- Goal 中に設計判断が発生したら、`design-decision` で結論が出る範囲は自動判断し、Goal 完了時にまとめて提示する。ルールで決まらない仕様・UX 判断は Stop Conditions に従って止まる。
- Goal 完了報告では、設計判断がない場合も `設計判断: なし` と明示する。
- 進捗・完了の報告は、このセッションのツール結果で裏取りできる事実だけを書く。テストが失敗していれば出力ごと報告し、未検証の項目は未検証と明示する。
- 後から制約になる判断、仕様変更、未着手作業は、画面出力だけで終わらせず `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` の適切な情報源へ同期する。
- 各 commit 内のレビューとは別に、Goal の commit range に対する `/code-review` 観点ベースのレビューと Codex レビュー（`codex-review`）を Goal 完了条件に含める（Goal Review 参照）。

## Acceptance

- Goal の目的が満たされている。
- 必要な commit がすべて作成されている。
- 各 commit が `default.md` の workflow を満たしている。
- Goal の commit range（`<base>..HEAD`）が `/code-review` 観点ベースのレビューと Codex レビューを通過し、対応必須の指摘が残っていない（打ち切った場合は残った指摘が `レビュー上限超過` として報告されている）。
- 必要な仕様・backlog・判断記録が同期されている。
- Goal 中の設計判断が完了時にまとめて提示され、該当する判断がない場合もその旨が明示されている。
- 作業ツリーの残差分がない、または残す理由が明確。

## Relevant

- `goal-workflow` skill
- `.claude/workflow/default.md`
- `design-decision` skill
- `/code-review`（built-in、effort 引数あり）
- `codex-review` skill
- `backlog/backlog.md`

## Flow

1. Goal の目的、制約、完了条件を確認し、ブランチは切らず開始時の `HEAD` を base SHA として記録する。
2. Goal を 1 commit 単位の候補へ分割する（Commit Slicing 参照）。
3. 次に扱う 1 commit 分を選び、`default.md` の workflow を実行する。
4. commit 後、Goal の残りと Goal Review の実施タイミングを確認する。
5. 残りがあれば次の 1 commit 分に戻る。
6. 必要な Goal Review（`/code-review` 観点ベース → Codex、対象は `<review_cursor>..HEAD`）と対応が済んでいなければ実施する。レビュー済みまで `review_cursor` を進めてよい（`base` は動かさない）。
7. 完了していれば Goal 全体の結果、残リスク、設計判断をまとめる。設計判断がない場合も `設計判断: なし` と書く。

## Branch

- ブランチは切らない。いるブランチ（通常 main）上にそのまま 1 commit ずつ積む。
- Goal 開始時の `HEAD` を base SHA として記録する。`base` は Goal 終了まで動かさない。Goal 全体の差分は `<base>..HEAD` で表し、最終報告と全体俯瞰の起点になる。
- 分割レビューの進捗は `review_cursor` で持つ。初期値は `base`。レビューが済むたびにレビュー済みの commit まで `review_cursor` を進める。次の分割レビュー対象は `<review_cursor>..HEAD`。`base` と `review_cursor` を混同しない（全体差分は常に `base` 起点、未レビュー差分は `review_cursor` 起点）。
- merge 操作はない。Goal 完了後もそのままブランチ上に commit が残る。
- 履歴は線形に保ち、各 commit を単独で revert / bisect できる状態に残す。

## Commit Slicing

- 1 commit は、単独で説明できるユーザー価値、仕様同期、リファクタ、テスト追加のいずれかに寄せる。
- 仕様同期と実装は、同じ変更の理解に必要なら同じ commit に含めてよい。
- 広いリファクタと振る舞い変更は、レビューしづらくなるなら分ける。リファクタを先に commit してから振る舞い変更を別 commit にする。
- 途中で 1 commit に収まらないと分かったら、作業を広げず commit 単位を切り直す。元タスクの一部分だけを完了 commit にし、残りを派生タスクとして `backlog/backlog.md` に追記する。

## Goal Review

各 commit 内の `review.md` とは別に、Goal の commit range を対象に以下を Goal 完了条件として実施する。ブランチは切らないので、レビュー range は commit range で表す。分割レビューの未レビュー対象は `<review_cursor>..HEAD`、Goal 全体の差分は `<base>..HEAD`。

- **実施順**: `/code-review` 観点ベース → Codex の順で実施する。`/code-review` 起点のレビュー指摘対応が済んでから Codex レビューを実行する。別系統モデル（Codex）は Claude 側の漏れを最後に拾う網として使う。
- **`/code-review` の使い方（観点取得 → 自前 subagent 実レビュー）**: `/code-review` は観点を返すだけに使い、実際のレビューは返ってきた観点で自分が subagent を起動して行う（実行手順は Code Review Delegation 参照）。`/code-review` の Phase 0 が出す差分指定（`@{upstream}...HEAD` / `main...HEAD` / `HEAD~1`）はそのまま使わず、現在のレビュー対象の差分に置き換える。Goal レビューでは未レビュー範囲 `git diff <review_cursor>..HEAD` をレビュー対象とする（分割しない場合は `review_cursor == base`）。`ultra` はクラウド・billed・ユーザー手動起動なので Goal 自動進行では使わない。
- **effort 選択基準**（観点取得時の effort）:
  - **xhigh**: 永続化 / 同期 / 外部連携 / 広い UI 挙動に触れる、または差分が大きい Goal。
  - **high**: docs 中心、または上記に触れない小さな差分の Goal。
- **Codex レビュー（必須）**: `codex-review` skill を未レビュー range（`<review_cursor>..HEAD`）対象で実行する。別系統モデル（Codex）に未レビュー差分を見せる。
- **分割レビュー**: 一気に全部ではなく、適当なコミットのまとまりごとにレビューしてよい（毎回でなくてよい）。レビューが済んだ範囲まで `review_cursor` を進める（`base` は動かさない）。次のレビューは進めた `review_cursor` からの `<review_cursor>..HEAD` を対象にする。
- **二重実行の省略**: Goal が 1 commit で完結し、その commit の `review.md` で既に `/code-review xhigh` 観点ベースのレビューを Goal 差分全体に対して通している場合、Goal Review の `/code-review` 起点レビューは省略してよい（Codex レビューは別系統なので省略しない）。複数 commit の Goal では各 commit の review.md は局所差分、Goal Review は commit range 全体を対象とするため両方実施する。
- 1 commit ごとではなく、関連する数 commit をまとめてレビューする。
- 差分が大きい、または永続化 / 同期 / 外部 API / 広い UI 挙動に触れる場合は、数 commit を待たずにその時点までの commit range で早めにレビューする。
- 指摘対応は別 commit として作成し、対応 commit を含む range で再レビューする。
- 各レビュー単位につき再レビュー実行（`/code-review` 起点・Codex とも）は最大 3 回。3 回で収束しなければそれ以上回さず打ち切り、残った指摘と実行回数を記録して Goal 完了報告の `レビュー上限超過` で通知する。

## Code Review Delegation

`/code-review` は観点を返すだけに使い、実際のレビューは返ってきた観点で自分が subagent を起動して行う。main の context を汚さないため実レビューは subagent に隔離し、main は最終採否と修正のみ行う。

- **観点の取得**: `/code-review`（`high` / `xhigh`、effort 選択基準で決めた値）を観点取得に使う。Phase 0 が出す差分指定（`@{upstream}...HEAD` / `main...HEAD` / `HEAD~1`）はそのまま使わず、現在のレビュー対象（Goal レビューでは未レビュー範囲 `git diff <review_cursor>..HEAD`）に置き換える。
- **実レビュー subagent の起動**: main は `Agent` ツールで取得した観点ごとに finder subagent を起動する。`model` を必ず明示する（基本 `sonnet`。判断の重い観点のみ `opus`）。複数観点は 1 メッセージで並列起動してよい。各 subagent には次を渡す:
  - 対象 commit range（未レビュー範囲 `<review_cursor>..HEAD`）または diff ファイルのパス
  - 担当する観点（`/code-review` が返した観点の 1 つ）
  - 直前の実装意図メモ（3 行以内、あれば）
- **subagent の責務**: 各 subagent は割り当てられた観点で対象差分をレビューし、修正は一切行わず、次の 2 つを返す:
  - **採用候補リスト**: 各項目に file:line / 問題 / failure scenario / 推奨対応（一行）。
  - **却下リスト**: 指摘と却下理由。
- **main の責務**: main が全 subagent の戻りを統合し、最終採否を行う。採用分の修正・テスト・コミットはすべて main で行う。
- **再レビュー**: 再レビューは finder subagent をもう一周起動する。再レビュー上限（最大 3 回）は維持する。

## Design Decisions

- Goal 完了時に、後から制約になる判断、仕様・UX の採否、既存方針から外れる判断だけをまとめて提示する。
- 該当する設計判断がない場合も、Goal 完了時に `設計判断: なし（Goal 報告対象の判断なし）` と明示する。
- ユーザー確認が必要な仕様・UX 判断がなかった場合も、Goal 完了時に `ユーザー確認が必要な判断: なし` と明示する。
- レビューループ（commit 内のレビュー周回、Goal Review の再レビュー回数）を上限で打ち切った場合は、Goal 完了時に `レビュー上限超過` として対象単位・回数・残った指摘を提示する。打ち切りがない場合も `レビュー上限超過: なし` と明示する。

## Stop Conditions

- Goal の完了条件が曖昧で、1 commit 単位へ切れない。
- 次の commit が仕様・UX・データ保持・削除方針の未確定判断に依存している（`design-decision` で結論が出る範囲なら止まらず採否を決める）。
- Goal の途中で、現在の目的と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している。
- 必須の検証手段がなく、代替検証やユーザー確認でも完了扱いにできない。
