# Review Workflow

## ICAR

- **Intent**: 完了前に、差分が要求・仕様・既存設計を壊していないことを確認する。
- **Constraints**:
  - 粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を見る。
  - 小さい変更は self-check でよい。
  - Small 以外の実装差分は、`codex-fresh-review` skill で review-only の fresh Review subagent（現在セッションの context を fork しない完全新規 subagent）を起動してレビューする。`change-review` はその reviewer が使うレビュー観点であり、レビュー実行器ではない。
  - Implementer（単発 Change では current agent）の責務はレビュー対象と観点を決めることまでで、実レビューは fresh Review subagent が行う。Implementer 自身による `codex-fresh-review` の代行レビューは禁止する。
  - テスト可能な振る舞い変更や bug fix に unit test / regression test がない場合は、原則 blocker として扱う。
  - review 開始前に、commit に含める code / tests / `backlog/backlog.md` / `docs/specs/` / `llm-wiki/` / `docs/decisions/` / ADR の内容変更が完了していることを確認する。未完了なら review せず `change/implement.md` に戻る。
  - product decision（UX・データ意味・cross-surface 等。カテゴリ一覧は同ファイル）を含む差分は、`.agents/workflow/design-decision-record.md` の基準で現在の要求 / backlog / docs / decisions または Product Decision Ledger から採用案・別案・理由を追えることを確認する。追えない場合、または指摘対応で新しい product decision が発生した場合は `change/implement.md` に戻る。
  - 公開 API / 削除 / 並行性 / 永続化 / 広い UI 挙動などは、`change-review` に加えて `project-risk-check` や別視点レビューを使う。<!-- slot: project-risk-check 以外に足す領域固有レビュー観点があれば追記する（例: UI 層に触れるなら対応する specialist skill）。 --><!-- /slot -->
  - 構造劣化リスクがある場合は `thermo-nuclear-code-quality-review` を必ず使う。
  - 指摘に対応しない場合は理由を残す。
  - Review subagent はファイルを変更しない。Implementer が採否、修正、再検証、必要なら再レビュー、commit を担当する。
  - review は commit 前の局所品質ゲートであり、最終保証ではない。採用した指摘を修正した後に再レビューするかは、差分の大きさ、risk、MUST 指摘の内容、新しい設計判断の有無から判断する。
  - 修正後に再レビューしない場合も、対応しない指摘・残リスク・Goal Review で見るべき観点があれば記録する。`レビュー上限超過` は Change 内 review では使わない。
- **Acceptance**:
  - 選んだレビュー深度と理由が説明できる。
  - review 対象が commit 予定差分全体（code / tests / docs / `backlog/backlog.md` / `docs/decisions/` を含む）である。
  - product decision を含む差分では、報告対象と報告不要な実装判断が分かれている。
  - 指摘があれば対応済み、または対応しない理由が明確。
  - レビュー後に変更した場合、必要な再検証が済んでいる。
- **Relevant**:
  - 変更差分
  - plan または要求
  - 検証結果
  - `codex-fresh-review` skill（fresh Review subagent の起動入口）
  - 関連する `docs/rules/`, `docs/specs/`, `llm-wiki/`（作業地図）

## Depth

- **Self-check**: Small 変更。Implementer が `git diff` を読み、要求と検証結果を照合する。
- **Standard**: Small 以外の実装差分。`codex-fresh-review` skill で fresh Review subagent を起動し、reviewer が `change-review` で必要な深さと追加 skill を判定する。
- **Targeted supplement**: 領域固有リスクがある変更。Review subagent が `change-review` に加えて Constraints に挙げた領域固有観点（`project-risk-check` など）で確認する。構造劣化リスクがある場合は `thermo-nuclear-code-quality-review` を必須とする。
- **External supplement**: 大きい、曖昧、High-risk、または設計判断が重い変更。Review subagent が必要な別視点レビューを入れる。

## Review Subagent

- Review subagent の起動は `codex-fresh-review` skill で行う。現在セッションの会話履歴・実装経緯を継承しない完全新規 subagent（`fork_context: false` 相当）として起動し、reviewer は repo state、diff / range、渡された短い要約から自分で文脈を把握する。
- Implementer は、レビュー対象（commit 前は commit に入れる予定の差分全体、commit 後は `<commit>^..<commit>` 等の固定 range）、変更目的の短い要約、検証結果、関連する `docs/rules/` / `docs/specs/` / `llm-wiki/`（作業地図）、追加で見てほしい観点を渡す。実装経緯や設計判断の理由は渡さない。
- Review subagent は `change-review` をレビュー観点として使い、必要な領域固有 skill の観点も利用する。
- Review subagent はファイル編集・git 書き込み・ビルド・テストを行わず、実害のある finding または `LGTM` を返す。
- Implementer が finding の採否、修正、再検証、必要なら再レビュー、commit を担当する。
- `codex-fresh-review` が実行不能な場合は、そのレビュー範囲を review 済みにせず、Stop Conditions として停止するか呼び出し元の判断に返す。
- この workflow は Codex config の `[agents] max_depth = 3` を前提にする。Goal 経由の最深連鎖は Orchestrator → Implementer → Review subagent → 下位 subagent であり、下位 subagent は depth budget が許す場合だけ使う。depth を変える場合は config と workflow の連鎖設計を合わせる。
- 追加調査や観点分割が有効で、agent depth budget が許す場合だけ、Review subagent は下位 subagent を起動して結果を統合してよい。depth budget が足りない場合は、Review subagent 自身で確認するか、Implementer に戻して委任方針を切り替える。
- reviewer 数、観点数、再レビュー回数は固定しない。

## Maintenance Findings

通常 review では maintenance-audit へ自動遷移しない。今回の差分を超える構造劣化・backlog 整理・ドキュメント整合性問題を見つけた場合は、今回の blocker でない限り別タスクとして `backlog/backlog.md` または `maintenance.md` の対象に切り出す。review 対象範囲内の問題の検出・報告は active scope だが、その修正の着手は `change/workflow.md` の横断スコープ制御で分類する（差分内の blocker は workflow-required、差分を超える改善は adjacent として capture / report）。

## Goal Boundary

この review は 1 commit / Change の commit 前差分だけを対象にする。Goal range では `goal.md` に従い、実行直前に固定した `<review_cursor>..<review_end>` への Goal Review（fresh Codex review。ユーザー明示時は Claude review も）だけを行い、ここでの Self Review / `change-review` を再実行しない。

Goal 経由の Change Review は局所的な correctness / spec / tests を担当し、commit 間の統合、Goal Acceptance、構築・read / write site の貫通、docs / backlog 整合は Goal Review が担当する。Goal を経由しない単発 Change では、Change Review がこれらの貫通・整合も担当する。

## Stop Conditions

- 指摘対応中に `change/workflow.md` の判断境界で Stop に該当する仕様・UX・設計方針が発生した。
- Small 以外の実装差分で `codex-fresh-review` が実行できない。
- 必要な別視点レビューが実行できない。
