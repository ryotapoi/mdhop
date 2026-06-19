# Goal Workflow

この workflow はこのプロジェクトの Goal 手順の正本。実装作業の発火入口は `goal-workflow` skill とし、`goal-workflow` skill はこのファイルを読んで進める。

## ICAR

- **Intent**: `/goal` で指定された目的を、複数の 1 commit workflow に分割して完了まで進める。
- **Constraints**:
  - 実装作業は `goal-workflow` skill を入口にし、この workflow を正本として読む。
  - `/goal` の呼び出し文は、原則として skill への参照と完了対象だけでよい。例: `/goal goal-workflow skill に従い、backlog/backlog.md の「v0.x」を完了して。`
  - Goal 開始時の `HEAD` を base commit として記録する。`base` は Goal 終了まで動かさず、Goal 全体の差分 `<base>..HEAD` と最終報告の起点にする。分割レビューの進捗は `review_cursor`（初期値 `base`）で別に持つ。ブランチは切らず、range で対象を表す。
  - 1 回の実装 workflow は 1 commit 単位に限る。Goal が複数 commit を必要とする場合は、`default.md` の workflow を commit ごとに繰り返す。
  - 各 commit は、Goal 全体の途中でも、その commit 単位では review / revert / bisect できる完了状態にする。
  - Goal 全体を 1 plan / 1 commit に押し込まない。次に扱う 1 commit 分を毎回明確に切り出す。
  - Goal 中に設計判断が発生したら、指定がない限り画面出力で残す。ユーザーがファイル出力を指定した場合だけ、指定先へ書く。
  - 設計判断の書き方は `agents/workflow/design-decision-record.md` に従う。
  - Goal 完了報告では、設計判断がない場合も `設計判断: なし` と明示する。
  - 進捗・完了の報告は、このセッションのツール結果で裏取りできる事実だけを書く。テストが失敗していれば出力ごと報告し、未検証の項目は未検証と明示する。
  - 後から制約になる判断、仕様変更、未着手作業は、画面出力だけで終わらせず `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` の適切な情報源へ同期する。
  - workflow の review とは別に、commit 済み内容へのコードレビューを Goal 完了条件に含める。
- **Acceptance**:
  - Goal の目的が満たされている。
  - 必要な commit がすべて作成されている。
  - 各 commit が `default.md` の workflow を満たしている。
  - Goal 開始時 base 以降の commit 済み内容がコードレビュー済みで、対応必須の指摘が残っていない（打ち切った場合は残った指摘が `レビュー上限超過` として報告されている）。
  - 必要な仕様・backlog・判断記録が同期されている。
  - Goal 中の設計判断が完了時にまとめて提示され、該当する判断がない場合もその旨が明示されている。
  - 作業ツリーの残差分がない、または残す理由が明確。
- **Relevant**:
  - `goal-workflow` skill
  - `agents/workflow/default.md`
  - `agents/workflow/design-decision-record.md`
  - `claude-review-request` skill（Goal Review を別系統の Claude Code に依頼する）
  - `backlog/backlog.md`
  - 関連する `docs/rules/`, `docs/specs/`, `docs/decisions/`, `llm-wiki/`（作業地図）

## Flow

1. Goal の目的、制約、完了条件を確認し、開始時の base commit を記録する。ブランチは切らない。
2. Goal を 1 commit 単位の候補へ分割する。
3. 次に扱う 1 commit 分を選び、`default.md` の workflow を実行する。
4. commit 後、Goal の残りとコードレビューの実施タイミングを確認する。
5. 残りがあれば次の 1 commit 分に戻る。
6. 必要なコードレビューと対応が済んでいなければ実施する。
7. 完了していれば Goal 全体の結果、残リスク、設計判断をまとめる。設計判断がない場合も `設計判断: なし` と書く。

## Commit Slicing

- 1 commit は、単独で説明できるユーザー価値、仕様同期、リファクタ、テスト追加のいずれかに寄せる。
- 仕様同期と実装は、同じ変更の理解に必要なら同じ commit に含めてよい。
- 広いリファクタと振る舞い変更は、レビューしづらくなるなら分ける。
- 途中で 1 commit に収まらないと分かったら、作業を広げず commit 単位を切り直す。

## Design Decisions

- Goal 中の設計判断は、未指定なら画面出力する。
- Goal 完了時に、`agents/workflow/design-decision-record.md` の粒度に該当する設計判断だけをまとめて提示する。
- 該当する設計判断がない場合も、Goal 完了時に `設計判断: なし（design-decision-record.md の粒度に該当する判断なし）` と明示する。
- ユーザー確認が必要な仕様・UX 判断がなかった場合も、Goal 完了時に `ユーザー確認が必要な判断: なし` と明示する。
- レビューループ（commit 内のレビュー周回、Goal Review の再レビュー回数）を上限で打ち切った場合は、Goal 完了時に `レビュー上限超過` として対象単位・回数・残った指摘を提示する。打ち切りがない場合も `レビュー上限超過: なし` と明示する。
- ファイル出力が指定された場合も、判断内容は `agents/workflow/design-decision-record.md` の基準で書く。
- 要求どおり実装しただけの内容、既存 specs / backlog に書かれている内容、単なる未実装 TODO は設計判断として扱わない。

## Goal Review

- Goal Review は、通常の `review.md` とは別に Goal 完了条件として扱う。
- レビュー対象は未コミット差分ではなく、未レビュー範囲の commit range `<review_cursor>..HEAD` とする（分割しない場合は `review_cursor == base`）。ブランチは切らないので range で対象を表す。
- 1 commit ごとではなく、関連する数 commit をまとめてレビューする。毎回でなくてよい。レビューが済んだ範囲まで `review_cursor` を進める（`base` は動かさない）。
- 差分が大きい、または永続化 / 同期 / 外部 API / 広い UI 挙動に触れる場合は、数 commit を待たずにその時点までの range で早めにレビューする。
- 指摘対応は別 commit として作成し、対応 commit を含む range で再レビューする。
- 各レビュー単位につきコードレビュー実行は最大 3 回。3 回で収束しなければそれ以上回さず打ち切り、残った指摘と実行回数を記録して Goal 完了報告の `レビュー上限超過` で通知する。
- `claude-review-request` skill にレビュー対象の commit range `<review_cursor>..HEAD` を渡して実行する。修正は Codex 側が行い、Claude は外部レビュアーとして指摘を返す。

## Stop Conditions

- Goal の完了条件が曖昧で、1 commit 単位へ切れない。
- 次の commit が仕様・UX・データ保持・削除方針の未確定判断に依存している。
- Goal の途中で、現在の目的と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している。
- 必須の検証手段がなく、代替検証やユーザー確認でも完了扱いにできない。
