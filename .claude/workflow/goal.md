# Goal Workflow

この workflow は mdhop の Goal 手順の正本。実装作業の発火入口は `goal-workflow` skill とし、`goal-workflow` skill はこのファイルを読んで進める。

## Intent

`/goal` で指定された目的を、複数の 1 commit workflow に分割して完了まで進める。

## Constraints

- 実装作業は `goal-workflow` skill を入口にし、この workflow を正本として読む。
- `/goal` の呼び出し文は、原則として skill への参照と完了対象だけでよい。例: `/goal goal-workflow skill に従い、backlog/backlog.md の「v0.x」を完了して。`
- Goal 開始時に専用ブランチを切る（Branch 参照）。Goal 開始時の `HEAD` を base commit として記録する。
- 1 回の実装 workflow は 1 commit 単位に限る。Goal が複数 commit を必要とする場合は、`default.md` の workflow を commit ごとに繰り返す。
- 各 commit は、Goal 全体の途中でも、その commit 単位では review / revert / bisect できる完了状態にする。
- Goal 全体を 1 plan / 1 commit に押し込まない。次に扱う 1 commit 分を毎回明確に切り出す。
- Goal 前提では都度のユーザー確認を避け、自動進行する。止まるのは Stop Conditions に該当する場合だけ。
- plan mode（`EnterPlanMode` / `ExitPlanMode`）は使わない。承認待ちが自動進行と噛み合わないため。計画が必要な場合は内部で立ててそのまま実装する。詳細は `plan.md`。
- Goal 中に設計判断が発生したら、`design-decision` で結論が出る範囲は自動判断し、Goal 完了時にまとめて提示する。ルールで決まらない仕様・CLI 挙動の判断は Stop Conditions に従って止まる。
- Goal 完了報告では、設計判断がない場合も `設計判断: なし` と明示する。
- 進捗・完了の報告は、このセッションのツール結果で裏取りできる事実だけを書く。テストが失敗していれば出力ごと報告し、未検証の項目は未検証と明示する。
- 後から制約になる判断、仕様変更、未着手作業は、画面出力だけで終わらせず `rules/` / `specs/`（あれば） / `decisions/` / `backlog/backlog.md` の適切な情報源へ同期する。
- 各 commit 内のレビューとは別に、Goal の commit range に対する Codex レビュー（`codex-review`）と `/code-review` を Goal 完了条件に含める（Goal Review 参照）。

## Acceptance

- Goal の目的が満たされている。
- 必要な commit がすべて作成されている。
- 各 commit が `default.md` の workflow を満たしている。
- Goal の commit range が Codex レビューと `/code-review` を通過し、対応必須の指摘が残っていない。
- 必要な仕様・backlog・判断記録が同期されている。
- Goal 中の設計判断が完了時にまとめて提示され、該当する判断がない場合もその旨が明示されている。
- レビュー通過後、ブランチが `--ff-only` で main にマージされている。
- 作業ツリーの残差分がない、または残す理由が明確。

## Relevant

- `goal-workflow` skill
- `.claude/workflow/default.md`
- `design-decision` skill
- `codex-review` skill
- `/code-review`（built-in、effort 引数あり）
- `backlog/backlog.md`

## Flow

1. Goal の目的、制約、完了条件を確認し、専用ブランチを切って開始時の base commit を記録する。
2. Goal を 1 commit 単位の候補へ分割する（Commit Slicing 参照）。
3. 次に扱う 1 commit 分を選び、`default.md` の workflow を実行する。
4. commit 後、Goal の残りと Goal Review の実施タイミングを確認する。
5. 残りがあれば次の 1 commit 分に戻る。
6. 必要な Goal Review（Codex + `/code-review`）と対応が済んでいなければ実施する。
7. 完了していれば `--ff-only` で main にマージし、Goal 全体の結果、残リスク、設計判断をまとめる。設計判断がない場合も `設計判断: なし` と書く。

## Branch

- Goal ごとに main から専用ブランチを切る。例: `git switch -c goal/<topic>`。
- ブランチ上に 1 commit ずつ積む。ブランチ差分（`main..HEAD`）が Goal 全体の差分になる。
- Goal Review 通過後、`git switch main && git merge --ff-only <branch>` で main へ戻す。ff できない場合は、未 push の Goal ブランチを main 上に rebase してから ff する（merge commit を作らない）。push 済みブランチは履歴を書き換えないこと。
- main は線形に保ち、各 commit を単独で revert / bisect できる状態に残す。

## Commit Slicing

- 1 commit は、単独で説明できるユーザー価値、仕様同期、リファクタ、テスト追加のいずれかに寄せる。
- 仕様同期と実装は、同じ変更の理解に必要なら同じ commit に含めてよい。
- 広いリファクタと振る舞い変更は、レビューしづらくなるなら分ける。リファクタを先に commit してから振る舞い変更を別 commit にする。
- 途中で 1 commit に収まらないと分かったら、作業を広げず commit 単位を切り直す。元タスクの一部分だけを完了 commit にし、残りを派生タスクとして `backlog/backlog.md` に追記する。

## Goal Review

各 commit 内の `review.md` とは別に、Goal の commit range（`main..HEAD`）を対象に以下を Goal 完了条件として実施する。

- **Codex レビュー（必須）**: `codex-review` skill を commit range 対象で実行する。別系統モデル（Codex）に Goal 差分全体を見せる。
- **`/code-review`（必須）**: `/code-review xhigh` をローカル実行する。effort は `xhigh`（最深ローカル。`ultra` はクラウド・billed・ユーザー手動起動なので Goal 自動進行では使わない）。`--fix` は付けず結果を受け取り、採否判断して直す。
- **二重実行の省略**: Goal が 1 commit で完結し、その commit の `review.md` で既に `/code-review xhigh` を Goal 差分全体に対して通している場合、Goal Review の `/code-review` は省略してよい（Codex レビューは別系統なので省略しない）。複数 commit の Goal では各 commit の review.md は局所差分、Goal Review は commit range 全体を対象とするため両方実施する。
- 1 commit ごとではなく、関連する数 commit をまとめてレビューする。
- 差分が大きい、または SQLite スキーマ / マイグレーション / リンク解決 / CLI 破壊的変更 / vault escape / 破壊的処理（`delete --rm`, `move`）に触れる場合は、数 commit を待たずにその時点までの commit range で早めにレビューする。
- 指摘対応は別 commit として作成し、対応 commit を含む range で再レビューする。
- 各レビュー単位につき Codex レビュー実行は最大 3 回までとする。対応必須の指摘が残る場合は、残リスクと次の判断をユーザーに確認する。

## Design Decisions

- Goal 完了時に、後から制約になる判断、仕様・CLI 挙動の採否、既存方針から外れる判断だけをまとめて提示する。
- 該当する設計判断がない場合も、Goal 完了時に `設計判断: なし（Goal 報告対象の判断なし）` と明示する。
- ユーザー確認が必要な仕様・CLI 挙動の判断がなかった場合も、Goal 完了時に `ユーザー確認が必要な判断: なし` と明示する。

## Stop Conditions

- Goal の完了条件が曖昧で、1 commit 単位へ切れない。
- 次の commit が仕様・CLI 挙動・データ保持・削除方針の未確定判断に依存している（`design-decision` で結論が出る範囲なら止まらず採否を決める）。
- Goal の途中で、現在の目的と `rules/` / `specs/` / `decisions/` が矛盾している。
- 必須の検証手段がなく、代替検証やユーザー確認でも完了扱いにできない。
- Codex レビューまたは `/code-review` で対応必須の指摘が残り、上限回数内で解消できない。
