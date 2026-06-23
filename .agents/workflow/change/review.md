# Review Workflow

## ICAR

- **Intent**: 完了前に、差分が要求・仕様・既存設計を壊していないことを確認する。
- **Constraints**:
  - 粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を見る。
  - 小さい変更は self-check でよい。
  - Small 以外の実装差分は fresh Review subagent を起動し、`change-review` と必要な領域固有 skill の観点で確認する。
  - テスト可能な振る舞い変更や bug fix に unit test / regression test がない場合は、原則 blocker として扱う。
  - review 開始前に、commit に含める code / tests / `backlog/backlog.md` / `docs/specs/` / `llm-wiki/` / `docs/decisions/` / ADR の内容変更が完了していることを確認する。未完了なら review せず `change/implement.md` に戻る。
  - 公開 API / 削除 / 並行性 / 永続化 / 広い UI 挙動などは、`change-review` に加えて `project-risk-check` や別視点レビューを使う。<!-- slot: project-risk-check 以外に足す領域固有レビュー観点があれば追記する（例: SwiftUI / TCA boundary なら該当 skill）。 --><!-- /slot -->
  - 構造劣化リスクがある場合は `thermo-nuclear-code-quality-review` を必ず使う。
  - 指摘に対応しない場合は理由を残す。
  - Review subagent はファイルを変更しない。Change worker が採否、修正、再検証、必要なら再レビュー、commit を担当する。
  - review は commit 前の局所品質ゲートであり、最終保証ではない。採用した指摘を修正した後に再レビューするかは、差分の大きさ、risk、MUST 指摘の内容、新しい設計判断の有無から判断する。
  - 修正後に再レビューしない場合も、対応しない指摘・残リスク・Goal の Cross-Agent Review で見るべき観点があれば記録する。`レビュー上限超過` は Change 内 review では使わない。
- **Acceptance**:
  - 選んだレビュー深度と理由が説明できる。
  - review 対象が commit 予定差分全体（code / tests / docs / `backlog/backlog.md` / `docs/decisions/` を含む）である。
  - 指摘があれば対応済み、または対応しない理由が明確。
  - レビュー後に変更した場合、必要な再検証が済んでいる。
- **Relevant**:
  - 変更差分
  - plan または要求
  - 検証結果
  - 関連する `docs/rules/`, `docs/specs/`, `llm-wiki/`（作業地図）

## Depth

- **Self-check**: Small 変更。Change worker が `git diff` を読み、要求と検証結果を照合する。
- **Standard**: Small 以外の実装差分。fresh Review subagent を起動し、`change-review` で必要な深さと追加 skill を判定する。
- **Targeted supplement**: 領域固有リスクがある変更。Review subagent が `change-review` に加えて Constraints に挙げた領域固有観点（`project-risk-check` など）で確認する。構造劣化リスクがある場合は `thermo-nuclear-code-quality-review` を必須とする。
- **External supplement**: 大きい、曖昧、High-risk、または設計判断が重い変更。Review subagent が必要な別視点レビューを入れる。

## Review Subagent

- Change worker は、Review subagent に Change の目的、commit 予定差分全体、検証結果、関連する `docs/rules/` / `docs/specs/` / `llm-wiki/`（作業地図）を渡す。
- Review subagent は `change-review` と、必要な領域固有 skill の観点を利用する。
- Review subagent はファイルを変更せず、実害のある finding または `LGTM` を返す。
- この workflow は Codex config の `[agents] max_depth = 3` を前提にする。Goal 経由の最深連鎖は Goal main → Change worker → Review subagent → 下位 subagent であり、下位 subagent は depth budget が許す場合だけ使う。depth を変える場合は config と workflow の連鎖設計を合わせる。
- 追加調査や観点分割が有効で、agent depth budget が許す場合だけ、Review subagent は下位 subagent を起動して結果を統合してよい。depth budget が足りない場合は、Review subagent 自身で確認するか、Change worker に戻して委任方針を切り替える。
- reviewer 数、観点数、再レビュー回数は固定しない。

## Maintenance Findings

通常 review では maintenance-audit へ自動遷移しない。今回の差分を超える構造劣化・backlog 整理・ドキュメント整合性問題を見つけた場合は、今回の blocker でない限り別タスクとして `backlog/backlog.md` または `maintenance.md` の対象に切り出す。review 対象範囲内の問題の検出・報告は active scope だが、その修正の着手は `change/workflow.md` の横断スコープ制御で分類する（差分内の blocker は workflow-required、差分を超える改善は adjacent として capture / report）。

## Goal Boundary

この review は 1 commit / Change の commit 前差分だけを対象にする。Goal range では `goal.md` に従い、実行直前に固定した `<review_cursor>..<review_end>` への Cross-Agent Review だけを行い、ここでの Self Review / `change-review` を再実行しない。

## Stop Conditions

- 指摘対応を進めるために、その時点の情報では適切な仕様・UX・設計方針を決められず、ユーザー判断や不足情報なしに進めること自体が不適切。
- 必要な別視点レビューが実行できない。
