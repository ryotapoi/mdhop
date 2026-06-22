# Review Workflow

## ICAR

- **Intent**: 完了前に、差分が要求・仕様・既存設計を壊していないことを確認する。
- **Constraints**:
  - 粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を見る。
  - 小さい変更は self-check でよい。
  - Small 以外の実装差分は原則 `change-review` を通す。
  - テスト可能な振る舞い変更や bug fix に unit test / regression test がない場合は、原則 blocker として扱う。
  - review 開始前に、commit に含める code / tests / `backlog/backlog.md` / `docs/specs/` / `llm-wiki/` / `docs/decisions/` / ADR の内容変更が完了していることを確認する。未完了なら review せず `change/implement.md` に戻る。
  - 公開 API / 削除 / 並行性 / 永続化 / 広い UI 挙動などは、`change-review` に加えて `project-risk-check` や別視点レビューを使う。<!-- slot: project-risk-check 以外に足す領域固有レビュー観点があれば追記する（例: SwiftUI / TCA boundary なら該当 skill）。 --><!-- /slot -->
  - 構造劣化リスクがある場合は `thermo-nuclear-code-quality-review` を必ず使う。
  - 指摘に対応しない場合は理由を残す。
  - reviewer を呼ぶ回数は、初回を含めて合計最大 3 回。`Review 1 -> Fix 1 -> Review 2 -> Fix 2 -> Review 3 -> Fix 3` まで行ったら Review 4 は行わない。Review 3 後の Fix 3 は未レビューの最終修正になるため、上限到達として最後の指摘、行った修正、未レビューの最終修正、残リスクを記録し、タスク完了報告（Goal なら Goal 完了報告）で `レビュー上限超過` として通知する。
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

- **Self-check**: Small 変更。main で `git diff` を読み、要求と検証結果を照合する。
- **Standard**: Small 以外の実装差分。原則 `change-review` を使い、必要な深さと追加 skill を判定する。
- **Targeted supplement**: 領域固有リスクがある変更。`change-review` に加えて Constraints に挙げた領域固有観点（`project-risk-check` など）で確認する。構造劣化リスクがある場合は `thermo-nuclear-code-quality-review` を必須とする。
- **External supplement**: 大きい、曖昧、High-risk、または設計判断が重い変更。`change-review` に加えて必要な別視点レビューを入れる。

## Maintenance Findings

通常 review では maintenance-audit へ自動遷移しない。今回の差分を超える構造劣化・backlog 整理・ドキュメント整合性問題を見つけた場合は、今回の blocker でない限り別タスクとして `backlog/backlog.md` または `maintenance.md` の対象に切り出す。review 対象範囲内の問題の検出・報告は active scope だが、その修正の着手は `change/workflow.md` の横断スコープ制御で分類する（差分内の blocker は workflow-required、差分を超える改善は adjacent として capture / report）。

## Goal Boundary

この review は 1 commit / Change の commit 前差分だけを対象にする。Goal range では `goal.md` に従い、実行直前に固定した `<review_cursor>..<review_end>` への Cross-Agent Review だけを行い、ここでの Self Review / `change-review` を再実行しない。

## Stop Conditions

- 指摘対応を進めるために、その時点の情報では適切な仕様・UX・設計方針を決められず、ユーザー判断や不足情報なしに進めること自体が不適切。
- 必要な別視点レビューが実行できない。
