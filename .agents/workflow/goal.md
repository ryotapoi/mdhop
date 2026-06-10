# Goal Workflow

この workflow は mdhop の Goal 手順の正本。実装作業の発火入口は `goal-workflow` skill とし、`goal-workflow` skill はこのファイルを読んで進める。

## ICAR

- **Intent**: `/goal` で指定された目的を、複数の 1 commit workflow に分割して完了まで進める。
- **Constraints**:
  - 実装作業は `goal-workflow` skill を入口にし、この workflow を正本として読む。
  - `/goal` の呼び出し文は、原則として skill への参照と完了対象だけでよい。例: `/goal goal-workflow skill に従い、backlog/backlog.md の「v0.x」を完了して。`
  - Goal 開始時の `HEAD` を base commit として記録する。
  - 1 回の実装 workflow は 1 commit 単位に限る。Goal が複数 commit を必要とする場合は、`default.md` の workflow を commit ごとに繰り返す。
  - 各 commit は、Goal 全体の途中でも、その commit 単位では review / revert / bisect できる完了状態にする。
  - Goal 全体を 1 plan / 1 commit に押し込まない。次に扱う 1 commit 分を毎回明確に切り出す。
  - Goal 中に設計判断が発生したら、指定がない限り画面出力で残す。ユーザーがファイル出力を指定した場合だけ、指定先へ書く。後から制約になる判断は `decisions/` に ADR として残す（基準は `commit` skill の ADR 判断基準に従う）。
  - Goal 完了報告では、設計判断がない場合も `設計判断: なし` と明示する。
  - 進捗・完了の報告は、このセッションのツール結果で裏取りできる事実だけを書く。テストが失敗していれば出力ごと報告し、未検証の項目は未検証と明示する。
  - 後から制約になる判断、仕様変更、未着手作業は、画面出力だけで終わらせず `rules/` / `specs/`（あれば） / `decisions/` / `backlog/backlog.md` の適切な情報源へ同期する。
  - workflow の review とは別に、commit 済み内容への Claude review を Goal 完了条件に含める。
- **Acceptance**:
  - Goal の目的が満たされている。
  - 必要な commit がすべて作成されている。
  - 各 commit が `default.md` の workflow を満たしている。
  - Goal 開始時 base 以降の commit 済み内容が Claude review 済みで、対応必須の指摘が残っていない。
  - 必要な仕様・backlog・判断記録が同期されている。
  - Goal 中の設計判断が完了時にまとめて提示され、該当する判断がない場合もその旨が明示されている。
  - 作業ツリーの残差分がない、または残す理由が明確。
- **Relevant**:
  - `goal-workflow` skill
  - `.agents/workflow/default.md`
  - `claude-review-request` skill
  - `backlog/backlog.md`
  - 関連する `rules/`, `specs/`（あれば）, `decisions/`, `references/knowledge.md`

## Flow

1. Goal の目的、制約、完了条件を確認し、開始時の base commit を記録する。
2. Goal を 1 commit 単位の候補へ分割する。
3. 次に扱う 1 commit 分を選び、`default.md` の workflow を実行する。
4. commit 後、Goal の残りと Claude review の実施タイミングを確認する。
5. 残りがあれば次の 1 commit 分に戻る。
6. 必要な Claude review と対応が済んでいなければ実施する。
7. 完了していれば Goal 全体の結果、残リスク、設計判断をまとめる。設計判断がない場合も `設計判断: なし` と書く。

## Commit Slicing

- 1 commit は、単独で説明できるユーザー価値、仕様同期、リファクタ、テスト追加のいずれかに寄せる。
- 仕様同期と実装は、同じ変更の理解に必要なら同じ commit に含めてよい。
- 広いリファクタと振る舞い変更は、レビューしづらくなるなら分ける。
- 途中で 1 commit に収まらないと分かったら、作業を広げず commit 単位を切り直す。元タスクの一部分だけを完了 commit にし、残りを派生タスクとして `backlog/backlog.md` に追記する。

## Design Decisions

- Goal 中の設計判断は、未指定なら画面出力する。
- Goal 完了時に、後から制約になる判断、仕様・CLI 挙動の採否、既存方針から外れる判断だけをまとめて提示する。
- 該当する設計判断がない場合も、Goal 完了時に `設計判断: なし（Goal 報告対象の判断なし）` と明示する。
- ユーザー確認が必要な仕様・CLI 挙動の判断がなかった場合も、Goal 完了時に `ユーザー確認が必要な判断: なし` と明示する。
- 要求どおり実装しただけの内容、既存 specs / backlog に書かれている内容、単なる未実装 TODO は設計判断として扱わない。

## Claude Review

- Claude review は、通常の `review.md` とは別に Goal 完了条件として扱う。Codex から見た別系統モデル（Claude）に Goal 差分全体を見せる。
- レビュー対象は未コミット差分ではなく、commit range とする。例: `base..HEAD` または `checkpoint..HEAD`。
- 1 commit ごとではなく、関連する数 commit をまとめてレビューする。
- 差分が大きい、または SQLite スキーマ / マイグレーション / リンク解決 / CLI 破壊的変更 / vault escape / 破壊的処理（`delete --rm`, `move`）に触れる場合は、数 commit を待たずにその時点までの commit range で早めにレビューする。
- Claude の指摘対応は別 commit として作成し、対応 commit を含む range で再レビューする。
- 各レビュー単位につき Claude review 実行は最大 3 回までとする。対応必須の指摘が残る場合は、残リスクと次の判断をユーザーに確認する。
- 実行手順は `claude-review-request` skill に従う。

## Stop Conditions

- Goal の完了条件が曖昧で、1 commit 単位へ切れない。
- 次の commit が仕様・CLI 挙動・データ保持・削除方針の未確定判断に依存している。
- Goal の途中で、現在の目的と `rules/` / `specs/` / `decisions/` が矛盾している。
- 必須の検証手段がなく、代替検証やユーザー確認でも完了扱いにできない。
- Claude review で対応必須の指摘が残り、上限回数内で解消できない。
