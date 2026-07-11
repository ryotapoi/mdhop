# Goal Workflow

この workflow はこのプロジェクトの Goal 手順の正本。実装作業の発火入口は `goal-workflow` skill とし、`goal-workflow` skill はこのファイルを読んで進める。

## ICAR

- **Intent**: `/goal` で指定された目的を、複数の 1 commit workflow に分割して完了まで進める。
- **Constraints**:
  - 役割は 2 層で固定する: **Orchestrator**（この workflow を進める main。進行管理・commit slicing・検品・レビュー・最終報告を担い、実装は書かない）と **Implementer**（実装担当の subagent）。
  - 実装作業は `goal-workflow` skill を入口にし、この workflow を正本として読む。
  - `/goal` の呼び出し文は、原則として skill への参照と完了対象だけでよい。例: `/goal goal-workflow skill に従い、backlog/backlog.md の「v0.x」を完了して。`
  - Goal 開始時の `HEAD` を base commit として記録する。`base` は Goal 終了まで動かさず、Goal 全体の差分 `<base>..HEAD` と最終報告の起点にする。分割レビューの進捗は `review_cursor`（初期値 `base`）で別に持つ。ブランチは切らず、range で対象を表す。
  - 1 回の実装 workflow は 1 commit 単位に限る。Orchestrator は実装を直接担当せず、Goal が 1 commit だけで完了する場合も、次の 1 Change を選んで fresh Implementer session を 1 つずつ直列起動する。
  - 各 commit は、Goal 全体の途中でも、その commit 単位では review / revert / bisect できる完了状態にする。
  - Goal 全体を 1 plan / 1 commit に押し込まない。次に扱う 1 commit 分を毎回明確に切り出す。
  - 複数案の判断は `change/workflow.md` の境界に従う。可逆で影響が小さい選択は採用案で進め、複数の妥当案が残り、かつ選択が非可逆またはやり直しコストが大きい場合、または正本と矛盾する場合は Stop Conditions に従う。
  - 仕様・UX の不明点は `change/workflow.md` の判断境界に従う。Product Decision Ledger の対象・記録・報告基準は `.agents/workflow/design-decision-record.md` を正本とし、Goal 完了時は各 Change の ledger、review 結果、同期済み docs から `ユーザー判断が必要` の有無を集約する。
  - 進捗・完了の報告は、このセッションのツール結果で裏取りできる事実だけを書く。テストが失敗していれば出力ごと報告し、未検証の項目は未検証と明示する。
  - 後から制約になる判断、仕様変更、未着手作業は、画面出力だけで終わらせず `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` の適切な情報源へ同期する。
  - workflow の review とは別に、commit 済み range への Goal Review（`Goal Review` 節。fresh Codex review）を Goal 完了条件に含める。Goal range に通常の Self Review / `change-review` 相当を再実行しない。
- **Acceptance**:
  - Goal の目的が満たされている。
  - 必要な commit がすべて作成されている。
  - 各 commit が `change/workflow.md` の workflow を満たしている。
  - Goal 開始時 base 以降の commit 済み内容が Goal Review 済み。レビュー上限に到達した場合は、最終修正が未レビューであることを含めて `レビュー上限超過` として報告されている。
  - 必要な仕様・backlog・判断記録が同期されている。
  - ユーザー判断が必要な項目の有無が、各 Change から引き継いだ記録に基づいて完了時に明示されている。
  - 作業ツリーの残差分がない、または残す理由が明確。
- **Relevant**:
  - `goal-workflow` skill
  - `.agents/workflow/change/workflow.md`
  - `.agents/workflow/design-decision-record.md`
  - `codex-fresh-review` skill（Goal Review の既定 reviewer。実装文脈を引き継がない fresh Review subagent に依頼する）
  - `usage-status` skill（CLI Implementer の thread ID を指定してモデルと usage を確認する）
  - `claude-fresh-review` skill（Goal Review では既定で使わない。ユーザーが明示した場合のみ別系統の Claude Code に追加依頼する。低レベル transport は `claude-review-request` 側に委譲し、Codex workflow の通常経路は tmux とする）
  - `backlog/backlog.md`
  - 関連する `docs/rules/`, `docs/specs/`, `docs/decisions/`, `llm-wiki/`（作業地図）

## Flow

1. Goal の目的、制約、完了条件を確認し、開始時の base commit を記録する。ブランチは切らない。
2. Goal を 1 commit 単位の候補へ分割する。
3. 次に扱う 1 commit 分を選び、fresh Implementer に渡す。Goal が 1 commit だけの場合も同じ。
4. commit 後、Goal の残りと Goal Review の実施タイミングを確認する。
5. 残りがあれば次の 1 commit 分に戻る。
6. 必要な Goal Review と対応が済んでいなければ実施する。
7. 完了または停止する時は、Goal 全体の結果、残リスク、ユーザー判断が必要な項目、レビュー上限超過の有無をまとめる。停止時は停止理由と解決すべきことが分かるようにする。

## Commit Slicing

- 1 commit に独立した複数作業を混ぜない。
- 1 commit は、単独で説明できるユーザー価値、仕様同期、リファクタ、テスト追加のいずれかに寄せる。
- 仕様同期と実装は、同じ変更の理解に必要なら同じ commit に含めてよい。
- 広いリファクタと振る舞い変更は、レビューしづらくなるなら分ける。
- 途中で 1 commit として不自然になったら、作業を広げず commit 単位を切り直す。
- Goal に必要な残作業は、次の Change として続けるか、別タスクが適切なら `backlog/backlog.md` に残す。どちらの場合も漏らさない。

## Implementer

- Goal 経由の Change は、commit 数に関わらず、fresh Implementer session に渡す。
- fresh Implementer は full-history fork ではなく新規コンテキストで起動する。前 commit / 前 Implementer の会話履歴は渡さず、必要な事実は commit、差分、現在のファイル、backlog、review result など現在の repository state から確認させる。
- Orchestrator は実装を直接担当しない。Orchestrator の責務は、base / review_cursor 管理、commit slicing、次の Change 選定、Goal Review、最終報告に限る。
- Orchestrator は次の 1 Change を選び、fresh Implementer session を 1 つずつ直列起動する。同じ worktree で複数の Implementer を並行実行しない。Goal が 1 commit だけで完了する場合も Implementer を 1 つ起動する。
- Implementer の既定は `worker` Custom Agent（`gpt-5.6-terra` / medium）。ユーザーが Goal Implementer のモデルを明示指定した場合だけ `worker` を使わず、`model` と `reasoning_effort` を直接指定して fresh subagent を起動する。難度や利用可否を理由に別モデルへ暗黙 fallback しない。
- Orchestrator は `agents.spawn_agent` を `fork_turns: "none"` で使う。既定では `agent_type: "worker"` を指定し、ユーザー明示モデルでは `agent_type` を省略する。起動結果と rollout / usage の実モデルが指定と一致しなければ、その結果を採用せず停止する。
- Implementer の完了は `agents.wait_agent` で待ち、追加指示は `agents.followup_task` または `agents.send_message`、中断は `agents.interrupt_agent`、状態確認は `agents.list_agents` を使う。完了・中断を確認するまで別 writer を起動しない。
- Implementer は渡された Change だけを active scope とし、Goal 全体を再計画・再分割しない。
- 通常は `change/workflow.md` に従い、調査から commit まで完了して戻る。
- 1 commit として不自然だと分かった場合は、作業を広げず事実を Orchestrator に返す。Orchestrator が commit 単位を切り直す。
- 戻りの表示形式は固定しないが、scope / result、commit SHA または stop 理由、検証コマンドと結果、review status、ledger / follow-up / 残存リスクの有無は必ず引き継ぐ。
- Implementer が中断した場合、fresh 再起動より先に同じ agent への追加指示・再開を試みてよい。
- Implementer / subagent の報告どうしが食い違う場合、Orchestrator はどちらかを採用する前に実ソース・実測で裏取りしてから記録・報告する。
- 直接実行の例外は Goal 経由の作業には適用しない。Goal を経由しない単発 Change だけは、現在の agent が直接実行してよい。
- Implementer は、独立委任が効率または品質を高める調査・実装補助・検証を必要に応じて下位 subagent に任せてよい。下位 subagent のモデルは Implementer モデル保証の対象外。

## Unresponsive Implementer

- Implementer への待機が複数回 timeout した場合、Orchestrator は同じ agent へ status request を送り、現在 phase / 実行中コマンド / 残作業 / blocker を求める。応答がなければ明示的に interrupt し、終了を確認するまで別 writer を起動しない。
- 終了確認後、Orchestrator は `git status --short`、`git diff --stat`、必要な `git diff`、`git log --oneline -n` で実状態を確認する。
- 元 Implementer の終了を確認でき、未コミット差分が今回の Change scope 内にあると判断できる場合は、その差分と同じ Change scope を 1 つの fresh recovery Implementer に渡し、review / verify / 必要修正 / commit を続行させる。Orchestrator は実状態の確認と引き継ぎに留め、実装を代行しない。元 Implementer の終了を確認できない場合は停止する。
- 差分が scope 外、破壊的、または完了状態を判断できない場合は、差分を破棄せず停止してユーザー確認する。
- この回収手順は例外処理であり、通常の Implementer に定期報告ファイルや常時 ledger を要求しない。

## Final Report

- 完了時も停止時も、報告形式は状況に合わせて分かりやすく整える。固定テンプレートに無理に合わせない。
- `ユーザー判断が必要: なし` または必要な判断内容を必ず明示する。
- `ユーザー判断が必要` は `.agents/workflow/design-decision-record.md` の基準で、各 Change の ledger、review 結果、同期済み docs から判断する。記憶だけで `なし` と判断しない。
- `レビュー上限超過: なし` または対象単位・回数・最後の指摘・行った修正・最終修正が未レビューであること・残リスクを状況に合わせて明示する。収束した review も、どの review が通ったかを状況に合わせて報告する。
- 停止時は、停止理由と解決すべきことが分かるようにする。

## Goal Review

- Goal Review は、通常の `change/review.md` とは別に Goal 完了条件として扱う。
- Change Review は個々の commit の局所的な correctness / spec / tests を見る。Goal Review は commit 間の統合、Goal Acceptance、構築・read / write site の貫通、docs / backlog 整合を見る。
- Goal Review は、実装文脈を引き継がない fresh な Codex による review（`codex-fresh-review` skill）を行い、その PASS 相当を通過とする。各 commit の Self Review は `change/review.md` で完了済みとして扱い、Goal range に対して通常の Self Review / `change-review` 相当は再実行しない。
- reviewer は実装文脈を引き継がない fresh reviewer とする。fresh であることを必須とし、実装と同系統でも fresh なら reviewer になれる。ユーザーが明示した場合のみ、`claude-fresh-review` で別系統の Claude Code によるレビューを追加し、その場合は両方の PASS 相当を通過条件とする。既定では追加しない。
- レビュー依頼には「変更したフィールド・型・メッセージについて、全構築サイト・全 read / write サイトを列挙して貫通漏れがないか確認する」観点を含める（複数ある組み立て経路の一部だけ修正される欠陥クラスに直効するため）。
- レビュー対象は未コミット差分ではなく、未レビュー範囲の commit range とする（分割しない場合は `review_cursor == base`）。Goal Review の実行直前に `review_start = review_cursor`、`review_end = 現在の HEAD の実 SHA` を確定し、1 回の review 中は `<review_start>..<review_end>` を動かさない。ブランチは切らないので range で対象を表す。
- 1 commit ごとではなく、関連する数 commit をまとめてレビューする。毎回でなくてよい。PASS 相当なら `review_cursor` を `review_end` まで進める（`base` は動かさない）。
- 差分が大きい、または永続化 / 同期 / 外部 API / 広い UI 挙動に触れる場合は、数 commit を待たずにその時点までの range で早めにレビューする。
- 指摘対応は別 commit として作成し、対応 commit を含む range で再レビューする。follow-up review でも実行直前に新しい `review_end` を取り直す。
- 各レビュー単位につき reviewer を呼ぶ回数は、初回を含めて合計最大 3 回。`Review 1 -> Fix 1 -> Review 2 -> Fix 2 -> Review 3 -> Fix 3` まで行ったら Review 4 は行わない。Review 3 後の Fix 3 は未レビューの最終修正になるため、同じ review 単位を上限到達として打ち切り、Goal 作業は続ける。
- fresh Codex review は `codex-fresh-review` skill にレビュー対象の commit range `<review_start>..<review_end>` を渡して実行する。
- Claude review を追加する場合は `claude-fresh-review` skill に同じ range を渡して実行する。修正は Codex 側が行い、Claude は外部レビュアーとして指摘を返す。Herdr は明示的に使う場合だけの transport とし、workflow の通常経路では使わない。reviewer が複数いる場合、PASS / 指摘 / レビュー上限は reviewer ごとに扱い、最終報告で個別に分かるようにする。

## Stop Conditions

- Goal の完了条件が曖昧で、1 commit 単位へ切れない。
- 次の commit が `change/workflow.md` の判断境界で Stop に該当する重要な仕様・UX・データ保持・削除方針に依存している。
- Goal の途中で、現在の目的と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している。
- 必須の検証を代替手段でも裏付けられず、完了扱いにできない。
- Goal Review を完全に実施できない。
- 指定された Implementer agent type / モデルを実測可能な起動経路で保証できない、または rollout の実モデルが指定と一致しない。
