# Review

## Intent

変更が要求・仕様・既存設計を壊していないことを、作業リスクに応じた深さで確認する。実装後レビューを標準とする。execution mode `delegate` では、このファイルで Claude 側の Implementer が担うとされる責務（採否判断・統合・完了責任）は Orchestrator が担う（`change/delegate.md`）。

## Review Depth

- **L0 self-check**: Small 変更（`change/workflow.md` の Intake 分類）。Implementer で `git diff` を読み、要求と検証結果を照合する。skill は呼ばない。
- **Standard**: Small 以外の実装差分。`/code-review`（`high` / `xhigh`）は観点取得に使い、実レビューは standard-review-coordinator が起動する finder subagent に隔離する（How To Run の Review Lane Delegation 参照）。coordinator が返した採用候補を Implementer が採否判断し、採用分の修正は execution mode に従う（`solo` は Implementer が直接修正、`delegate` は `change/delegate.md` の実装エージェントへ再委譲。実装対象がない修正はどの mode でも直接編集してよい）。
- **Targeted supplement**: 領域固有リスクがある変更。Standard に加えて該当観点の skill を使う。
- **External supplement**: 大きい、曖昧、High-risk、または設計判断が重い変更。Standard に加えて必要な補助レビュー skill を入れる。

## Decision Criteria

- L0 で十分なケース: typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正。
- **Small 以外の実装差分は原則 `/code-review`（`high` / `xhigh`）の観点を通す**（Standard、How To Run の Review Lane Delegation 参照）。避ける余地を減らす。effort は差分の性質で使い分ける（基本 `xhigh`、docs 中心など小差分は `high`）。`/code-review` は観点取得に使い、Phase 0 の差分指定はそのまま使わず現在のレビュー対象の差分に置き換える。実レビューは standard-review-coordinator が観点ごとに起動する finder subagent に隔離する。`ultra` はクラウド・billed・ユーザー手動起動なので自動進行では使わない。
- 構造劣化リスク（巨大化、分岐増加、責務境界の濁り、薄い抽象化、型境界の曖昧さ、canonical layer 逸脱）があれば `thermo-nuclear-code-quality-review` を**必須**で使う。
- review 開始前に、commit に含める code / tests / `backlog/backlog.md` / `docs/specs/` / `llm-wiki/` / `docs/decisions/` / ADR の内容変更が完了していることを確認する。未完了なら review せず `change/implement.md` に戻る。
- product decision（UX・データ意味・cross-surface 等。カテゴリ一覧は同ファイル）を含む差分は、`.claude/workflow/design-decision-record.md` の基準で現在の要求 / backlog / docs / decisions または Product Decision Ledger から採用案・別案・理由を追えることを確認する。追えない場合、または指摘対応で新しい product decision が発生した場合は `change/implement.md` に戻る。
- 領域固有 supplement の対象:
  - プロジェクト固有制約に触れる差分 → `project-risk-check`（何が固有制約かは skill 側が判定する）
  <!-- slot: project-risk-check 以外の領域固有レビューのマッピングがあれば追記する（例: 「UI 層 → 対応する specialist skill」）。 -->
  <!-- /slot -->
- **テスト可能な振る舞い変更や bug fix に unit test / regression test がない場合は、原則 blocker として扱う**（`change/verify.md` で未完了。理由がある例外のみ許容）。
- review は粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を探す。
- 指摘に対応しない場合は、理由を plan / commit body / 該当ドキュメントに記録する。
- review は commit 前の局所品質ゲートであり、最終保証ではない。採用した指摘を修正した後に再レビューするかは、差分の大きさ、risk、MUST 指摘の内容、新しい設計判断の有無から判断する。
- 修正後に再レビューしない場合も、対応しない指摘・残リスク・Goal Review で見るべき観点があれば記録する。`レビュー上限超過` は Change 内 review では使わない。

## How To Run

- L0: Implementer で `git diff` を読み、acceptance と照合する。
- Standard: 下の Review Lane Delegation に従い、standard-review-coordinator を起動する。coordinator は `/code-review` で観点を取得し、その観点で finder subagent を起動して結果を統合する。
- Targeted supplement: 必要な領域ごとに別 lane coordinator を起動する。`project-risk-check` が必要なら project-risk-review-coordinator を起動し、skill に従って 2〜5 個の観点 subagent を起動・統合させる。
- External supplement: `thermo-nuclear-code-quality-review` が必要なら structural-review-coordinator を起動し、構造品質レビュー結果を整理させる。ほかに補助レビュー skill が必要なら lane を分けてよい。
- 必要な lane coordinator は 1 メッセージで並列起動してよい。
- 戻りを全部受け取ってから Implementer で統合し、採用分をまとめて反映する。実行中に 1 件ずつ反映しない。

### Review Lane Delegation

review lane はレビュー実行と候補整理だけを担当する。Implementer の context を汚さず、最終採否・修正の差配・検証・コミットは Implementer に残す。

lane coordinator と finder subagent の起動は、結果を起動呼び出しの戻り値で受け取る同期実行を基本とする。background になった subagent の完了通知は待たず、返らなければ `SendMessage` で能動的に回収する（`change/workflow.md` の Subagent / Skill 参照）。

#### standard-review-coordinator

`/code-review` は観点を返すだけに使い、実レビューは観点ごとの finder subagent に隔離する。

- `/code-review`（`high` / `xhigh`）で観点を取得する。Phase 0 が出す差分指定（`@{upstream}...HEAD` / `main...HEAD` / `HEAD~1`）はそのまま使わず、現在のレビュー対象の差分に置き換える。
- standard-review-coordinator は `Agent` ツールで取得した観点ごとに finder subagent を起動する。`model` を必ず明示する（基本 `sonnet`、判断の重い観点のみ `opus`）。複数観点は 1 メッセージで並列起動してよい。各 subagent には対象差分（diff ファイルのパスまたは range）、担当観点、実装意図の文脈を渡す。実装意図の文脈は、execution mode `solo` で `tmp/solo-plan-<change>.md` があればそのパスを渡し、なければ直前の実装意図メモ（3 行以内、あれば）でよい。plan file は文脈提供であり、plan vs diff の照合は依頼しない。
- 各 subagent は修正せず、採用候補リスト（file:line / 問題 / failure scenario / 推奨対応一行）と却下リスト（指摘と却下理由）を返す。
- standard-review-coordinator は全戻りを統合し、採用候補 / 却下候補を Implementer に返す。修正はしない。
- このレビューでの effort は Standard では `xhigh` を基本とし、docs 中心など小さい差分では `high` を選んでよい。

#### project-risk-review-coordinator

- `project-risk-check` を Read し、対象差分に必要な観点クラスタを 2〜5 個に分ける。
- 観点クラスタごとに subagent を起動し、プロジェクト固有リスクの事実を集める。
- 戻りを dedup し、重要度を付けて Implementer に返す。修正はしない。

#### structural-review-coordinator

- `thermo-nuclear-code-quality-review` を Read し、対象差分に適用する。
- 構造劣化リスクを finding 形式で整理して Implementer に返す。修正はしない。

Implementer が全 lane の戻りを統合して最終採否を行い、採用分の修正は execution mode に従って反映する（`solo` は直接修正、`delegate` は `change/delegate.md` の実装エージェントへ再委譲。実装対象がない修正はどの mode でも直接編集可）。検証・コミットは Change の完了責任者（Implementer、`delegate` では Orchestrator）が行う。再レビューは、差分の大きさ、risk、MUST 指摘の内容、新しい設計判断の有無から必要な lane だけをもう一周起動する。

Goal 全体の commit range では、ここでの Self Review / `/code-review` を再実行しない。Goal range は `goal.md` に従い、実行直前に固定した `<review_cursor>..<review_end>` への Goal Review（`goal.md` の reviewer 規定に従う fresh reviewer）だけを行う。

Goal 経由の Change Review は局所的な correctness / spec / tests を担当し、commit 間の統合、Goal Acceptance、構築・read / write site の貫通、docs / backlog 整合は Goal Review が担当する。Goal を経由しない単発 Change では、Change Review がこれらの貫通・整合も担当する。

## Acceptance

- 選んだ review depth と理由が説明できる
- review 対象が commit 予定差分全体（code / tests / docs / `backlog/backlog.md` / `docs/decisions/` を含む）である
- product decision を含む差分では、報告対象と報告不要な実装判断が分かれている
- テスト可能な振る舞い変更 / bug fix に unit / regression test がある、または追加しない理由が明確
- 指摘があれば対応済み、または対応しない理由が明確
- レビュー後の変更に対して必要な再検証が済んでいる
- 修正後に再レビューしない場合、その判断理由と残リスクが説明できる

## Maintenance Findings

今回の差分ではなく、複数タスク後の全体構造・負債を見るレビューは `maintenance.md`（L3）で行う。

L3 はレビュー回数の数え方ではない。節目で呼ぶもの（久々に広く触った、バージョンの区切り、同種の修正が続いた、リファクタ候補が複数出た）。単一差分を超える構造劣化や backlog 整理が必要なら、通常レビューから自動遷移せず maintenance 候補として別タスク化する。review 対象範囲内の問題の検出・報告は active scope だが、その修正の着手は `boundary-control` で分類する（差分内の blocker は workflow-required、差分を超える改善は adjacent として capture / report）。

## Stop Conditions

- 指摘対応中に `change/workflow.md` の判断境界で Stop に該当する仕様・UX・設計方針が発生した。
- External supplement が必要なリスクなのに補助レビューが実行できない。
