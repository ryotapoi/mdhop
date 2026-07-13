# Review

## Intent

変更が要求・仕様・既存設計を壊していないことを、作業リスクに応じた深さで確認する。実装後レビューを標準とする。Goal 経由の Normal 以上の Change では、このファイルの review 差配・最終採否・完了責任は Gatekeeper（Change ごとの fresh subagent、実装文脈を引き継がない）が担う。Gatekeeper の照合は、Conductor が Implementer 起動時に確定して Gatekeeper にも不変入力として渡す Change brief（scope・Acceptance・非対象・設計制約）・plan との照合とする（要求文そのものへの再解釈ではなく、確定済み brief を基準にする）。Small は Conductor が直接照合する（Gatekeeper 省略）。Implementer / Gatekeeper のモデル指定は実体の違いに過ぎず、この phase の主体には影響しない。Gatekeeper に GPT 系を指定した場合も、この phase の責務は同一で、起動・運転だけ `change/delegate.md` の Gatekeeper 委譲に従う。Goal を経由しない単発 Change では Gatekeeper / Conductor という役割分担自体が存在しないため、実行中の agent がこのファイルの主体（review 差配・最終採否）を兼ねる。

## Review Depth

- **L0 self-check**: Small 変更（`change/workflow.md` の Intake 分類）。Conductor が `git diff` を読み、要求と検証結果を照合する。skill は呼ばない。
- **Standard**: Small 以外の実装差分。`/code-review`（`high` / `xhigh`）は観点取得に使い、実レビューは standard-review-coordinator が起動する finder subagent に隔離する（How To Run の Review Lane Delegation 参照）。coordinator が返した採用候補を Gatekeeper が採否判断し、採用分の修正は Implementer の実体に従って差し戻す（Claude 系 Implementer は直接修正、GPT 系は watchdog 経由で Implementer（codex）セッションを resume して修正させる。実装対象がない修正であっても Gatekeeper は直接編集せず Implementer へ差し戻す）。差し戻しは Conductor 経由で同一 Implementer を再開させる（上限 2 往復。上限超過時の扱いは `goal.md` の Gatekeeper 節を参照）。
- **Targeted supplement**: 領域固有リスクがある変更。Standard に加えて該当観点の skill を使う。
- **External supplement**: 大きい、曖昧、High-risk、または設計判断が重い変更。Standard に加えて必要な補助レビュー skill を入れる。

## Decision Criteria

- L0 で十分なケース: typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正。
- **Small 以外の実装差分は原則 `/code-review`（`high` / `xhigh`）の観点を通す**（Standard、How To Run の Review Lane Delegation 参照）。避ける余地を減らす。effort は差分の性質で使い分ける（基本 `xhigh`、docs 中心など小差分は `high`）。`/code-review` は観点取得に使い、Phase 0 の差分指定はそのまま使わず現在のレビュー対象の差分に置き換える。実レビューは standard-review-coordinator が観点ごとに起動する finder subagent に隔離する。`ultra` はクラウド・billed・ユーザー手動起動なので自動進行では使わない。
- 構造劣化リスク（巨大化、分岐増加、責務境界の濁り、薄い抽象化、型境界の曖昧さ、canonical layer 逸脱）があれば `thermo-nuclear-code-quality-review` を**必須**で使う。
- review 開始前に、commit に含める code / tests / `backlog/backlog.md` / `docs/specs/` / `llm-wiki/` / `docs/decisions/` / ADR の内容変更が完了していることを確認する。未完了なら review せず、Gatekeeper は Conductor 経由で Implementer に差し戻す（`change/implement.md` の続き）。
- product decision（UX・データ意味・cross-surface 等。カテゴリ一覧は同ファイル）を含む差分は、`.claude/workflow/design-decision-record.md` の基準で現在の要求 / backlog / docs / decisions または Product Decision Ledger から採用案・別案・理由を追えることを確認する。追えない場合、または指摘対応で新しい product decision が発生した場合は、Gatekeeper は Conductor 経由で Implementer に差し戻す。
- 領域固有 supplement の対象:
  <!-- slot: 領域固有レビューのマッピングがあれば追記する（例: 「UI 層 → 対応する specialist skill」）。 -->
  <!-- /slot -->
- **テスト可能な振る舞い変更や bug fix に unit test / regression test がない場合は、原則 blocker として扱う**（`change/verify.md` で未完了。理由がある例外のみ許容）。
- review は粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を探す。
- 指摘に対応しない場合は、理由を plan / commit body / 該当ドキュメントに記録する。
- review は commit 前の局所品質ゲートであり、最終保証ではない。採用した指摘を修正した後に再レビューするかは、差分の大きさ、risk、MUST 指摘の内容、新しい設計判断の有無から判断する。
- 修正後に再レビューしない場合も、対応しない指摘・残リスク・Goal Review で見るべき観点があれば記録する。`レビュー上限超過` は Change 内 review では使わない。

## How To Run

- L0: Conductor が `git diff` を読み、acceptance と照合する（Small のみ）。
- Standard: 下の Review Lane Delegation に従い、Gatekeeper が standard-review-coordinator を起動する。coordinator は `/code-review` で観点を取得し、その観点で finder subagent を起動して結果を統合する。
- Targeted supplement: 必要な領域ごとに別 lane coordinator を起動し、該当観点の skill を Read させて観点 subagent を起動・統合させる。
- External supplement: `thermo-nuclear-code-quality-review` が必要なら structural-review-coordinator を起動し、構造品質レビュー結果を整理させる。ほかに補助レビュー skill が必要なら lane を分けてよい。
- 必要な lane coordinator は 1 メッセージで並列起動してよい。
- 戻りを全部受け取ってから Gatekeeper で統合し、採用分をまとめて反映する。実行中に 1 件ずつ反映しない。

### Review Lane Delegation

review lane はレビュー実行と候補整理だけを担当する。Gatekeeper の context を汚さず、最終採否・修正の差配は Gatekeeper に残る（検証・コミットは Gatekeeper の責務ではなく、修正は Implementer へ差し戻し、コミットは Conductor が行う）。

lane coordinator と finder subagent の起動は、結果を起動呼び出しの戻り値で受け取る同期実行を基本とする。background になった subagent の完了通知は待たず、返らなければ `SendMessage` で能動的に回収する（`change/workflow.md` の Subagent / Skill 参照）。

#### standard-review-coordinator

`/code-review` は観点を返すだけに使い、実レビューは観点ごとの finder subagent に隔離する。

- `/code-review`（`high` / `xhigh`）で観点を取得する。Phase 0 が出す差分指定（`@{upstream}...HEAD` / `main...HEAD` / `HEAD~1`）はそのまま使わず、現在のレビュー対象の差分に置き換える。
- standard-review-coordinator は `Agent` ツールで取得した観点ごとに finder subagent を起動する。`model` を必ず明示する（基本 `sonnet`、判断の重い観点のみ `opus`）。複数観点は 1 メッセージで並列起動してよい。各 subagent には対象差分（diff ファイルのパスまたは range）、担当観点、実装意図の文脈を渡す。実装意図の文脈は、Claude 系 Implementer で `tmp/solo-plan-<change>.md` があればそのパスを渡し、なければ直前の実装意図メモ（3 行以内、あれば）でよい。plan file は文脈提供であり、plan vs diff の照合は依頼しない。
- `/code-review` の観点に加えて、全列挙観点の finder を常設で 1 体起動する。担当: diff が追加・変更した型・フィールド・enum ケースごとに、リポジトリ横断で全構築・全 read/write・全変換サイトを file:line で列挙し、1 箇所ずつ変更の反映を確認する。同じ意味論の変換関数が別経路に同型で存在しないかを、既存の兄弟フィールド・兄弟関数の消費サイトからの逆引きで探すことまで含める（欠落型欠陥＝diff 外の反映漏れは diff 実読と通常観点では検出できないことが実測されているため、この観点だけ列挙を義務化する）。
- 各 subagent は修正せず、採用候補リスト（file:line / 問題 / failure scenario / 推奨対応一行）と却下リスト（指摘と却下理由）を返す。
- standard-review-coordinator は全戻りを統合し、採用候補 / 却下候補を Gatekeeper に返す。修正はしない。
- このレビューでの effort は Standard では `xhigh` を基本とし、docs 中心など小さい差分では `high` を選んでよい。

#### structural-review-coordinator

- `thermo-nuclear-code-quality-review` を Read し、対象差分に適用する。
- 構造劣化リスクを finding 形式で整理して Gatekeeper に返す。修正はしない。

Gatekeeper が全 lane の戻りを統合して最終採否を行う。採用分の修正は Implementer の実体に従って差し戻す（Claude 系 Implementer は直接修正、GPT 系は watchdog 経由で Implementer（codex）セッションを resume して修正させる）。実装対象がない修正であっても Gatekeeper は直接編集しない。自分の修正を自分で受け入れると採否の独立性が崩れ、`goal.md` の diff hash 照合の前提も壊れるため、すべて Conductor 経由で Implementer に差し戻す（複数の指摘は 1 回の差し戻しにまとめる）。差し戻しは Conductor 経由で同一 Implementer を `SendMessage` で再開させる（上限 2 往復。上限に達しても未解決の MUST が残る場合は commit せず停止してユーザーに確認する。残りが SHOULD 以下のみの場合に限り、Gatekeeper が残リスク受容を明示して accept したときだけ commit に進める）。検証は Implementer が再実行し、コミットは Conductor が行う。再レビューは、差分の大きさ、risk、MUST 指摘の内容、新しい設計判断の有無から必要な lane だけをもう一周起動する。

Goal 全体の commit range では、ここでの Change Review / `/code-review` を再実行しない。Goal range は `goal.md` に従い、実行直前に固定した `<review_cursor>..<review_end>` への Goal Review（`goal.md` の reviewer 規定に従う fresh reviewer）だけを行う。

Goal 経由の Change Review（Gatekeeper が担当）は局所的な correctness / spec / tests を担当し、commit 間の統合、Goal Acceptance、構築・read / write site の貫通、docs / backlog 整合は Goal Review が担当する。Goal を経由しない単発 Change では、Change Review がこれらの貫通・整合も担当する。

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
