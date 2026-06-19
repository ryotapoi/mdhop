# Review

## Intent

変更が要求・仕様・既存設計を壊していないことを、作業リスクに応じた深さで確認する。実装後レビューを標準とする。

## Review Depth

- **L0 self-check**: Small 変更（`default.md` の Intake 分類）。main で `git diff` を読み、要求と検証結果を照合する。skill は呼ばない。
- **Standard**: Small 以外の実装差分。`/code-review`（`high` / `xhigh`）は観点取得に使い、実レビューは返ってきた観点で自分が起動する finder subagent に隔離する（How To Run の Code Review Delegation 参照）。subagent が返した採用候補を main で採否判断し、修正は main で行う。
- **Targeted supplement**: 領域固有リスクがある変更。Standard に加えて該当観点の skill を使う。
- **External supplement**: 大きい、曖昧、High-risk、または設計判断が重い変更。Standard に加えて別系統レビュー（`codex-review`）を入れる。

## Decision Criteria

- L0 で十分なケース: typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正。
- **Small 以外の実装差分は原則 `/code-review`（`high` / `xhigh`）の観点を通す**（Standard、How To Run の Code Review Delegation 参照）。避ける余地を減らす。effort は差分の性質で使い分ける（基本 `xhigh`、docs 中心など小差分は `high`）。`/code-review` は観点取得に使い、Phase 0 の差分指定はそのまま使わず現在のレビュー対象の差分に置き換える。実レビューは観点ごとに自分が起動する finder subagent に隔離する。`ultra` はクラウド・billed・ユーザー手動起動なので自動進行では使わない。
- 構造劣化リスク（巨大化、分岐増加、責務境界の濁り、薄い抽象化、型境界の曖昧さ、canonical layer 逸脱）があれば `thermo-nuclear-code-quality-review` を**必須**で使う。
- 領域固有 supplement の対象:
  - プロジェクト固有制約に触れる差分 → `project-risk-check`（何が固有制約かは skill 側が判定する）
  <!-- slot: project-risk-check 以外の領域固有レビューのマッピングがあれば追記する（例: 「View 層 → swiftui-pro」）。 -->
  <!-- /slot -->
- **テスト可能な振る舞い変更や bug fix に unit test / regression test がない場合は、原則 blocker として扱う**（`verify.md` で未完了。理由がある例外のみ許容）。
- review は粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を探す。
- 指摘に対応しない場合は、理由を plan / commit body / 該当ドキュメントに記録する。
- レビュー周回は最大 3 周。3 周で収束しなければそれ以上回さず打ち切る。打ち切った場合は残った指摘と周回数を記録し、タスク完了報告（Goal なら Goal 完了報告）で `レビュー上限超過` として通知する。

## How To Run

- L0: main で `git diff` を読み、acceptance と照合する。
- Standard: 下の Code Review Delegation に従い、`/code-review` で観点を取得し、その観点で起動した finder subagent の採用候補を採否判断して反映する。
- Targeted / External supplement: 該当 skill（`thermo-nuclear-code-quality-review`, `codex-review`、および上の領域固有マッピングで指定した skill）を呼ぶ。複数該当するものは 1 メッセージで並列起動してよい。
- 戻りを全部受け取ってから main で統合し、採用分をまとめて反映する。実行中に 1 件ずつ反映しない。

### Code Review Delegation

`/code-review` は観点を返すだけに使い、実レビューは観点ごとに自分が起動する finder subagent に隔離する。main の context を汚さず、最終採否は main に残す。手順は `goal.md` の Code Review Delegation と同一:

- `/code-review`（`high` / `xhigh`）で観点を取得する。Phase 0 が出す差分指定（`@{upstream}...HEAD` / `main...HEAD` / `HEAD~1`）はそのまま使わず、現在のレビュー対象の差分に置き換える。
- main は `Agent` ツールで取得した観点ごとに finder subagent を起動する。`model` を必ず明示する（基本 `sonnet`、判断の重い観点のみ `opus`）。複数観点は 1 メッセージで並列起動してよい。各 subagent には対象差分（diff ファイルのパスまたは range）、担当観点、直前の実装意図メモ（3 行以内、あれば）を渡す。
- 各 subagent は修正せず、採用候補リスト（file:line / 問題 / failure scenario / 推奨対応一行）と却下リスト（指摘と却下理由）を返す。
- main が全戻りを統合して最終採否を行い、修正・テスト・コミットはすべて main で行う。再レビューは finder subagent をもう一周起動する（再レビュー上限 3 周は維持）。
- このレビューでの effort は Standard では `xhigh` を基本とし、docs 中心など小さい差分では `high` を選んでよい。Goal Review での effort 選択基準は `goal.md` を参照する。

Goal 全体の commit range（`<base>..HEAD`）に対する `codex-review` と `/code-review` 観点ベースのレビューは、各 commit のここでの review とは別に Goal 完了条件として `goal.md` の Goal Review で実施する。

## Acceptance

以下のいずれかを満たした状態:

- レビュー指摘 0 件
- 残った指摘すべてが前回と**根拠（why）が同じ**再指摘（A/A' は新規角度なら対応してレビューへ戻る）
- 最大周回数（3 周）で打ち切り、残った指摘を `レビュー上限超過` として完了報告に含める

加えて:

- 選んだ review depth と理由が説明できる
- テスト可能な振る舞い変更 / bug fix に unit / regression test がある、または追加しない理由が明確
- 指摘があれば対応済み、または対応しない理由が明確
- レビュー後の変更に対して必要な再検証が済んでいる

## Maintenance Findings

今回の差分ではなく、複数タスク後の全体構造・負債を見るレビューは `maintenance.md`（L3）で行う。

L3 はレビュー回数の数え方ではない。節目で呼ぶもの（久々に広く触った、バージョンの区切り、同種の修正が続いた、リファクタ候補が複数出た）。単一差分を超える構造劣化や backlog 整理が必要なら、通常レビューから自動遷移せず maintenance 候補として別タスク化する。review 対象範囲内の問題の検出・報告は active scope だが、その修正の着手は `boundary-control` で分類する（差分内の blocker は workflow-required、差分を超える改善は adjacent として capture / report）。

## Stop Conditions

- 指摘対応が仕様・UX・設計方針を変える。
- External supplement が必要なリスクなのに別系統レビューが実行できない。
