# Review

## Intent

変更が要求・仕様・既存設計を壊していないことを、作業リスクに応じた深さで確認する。実装後レビューを標準とする。

## Review Depth

- **L0 self-check**: Small 変更（`default.md` の Intake 分類）。main で `git diff` を読み、要求と検証結果を照合する。skill は呼ばない。
- **Standard**: Small 以外の実装差分。`/code-review`（effort は high / xhigh を使い分け）をレビュー監督 subagent 経由で実行し、返ってきた採用候補 / 却下リストを main で採否判断して直す（main で直接 `/code-review` を実行しない。`--fix` も使わない）。実行方法と effort の使い分けは How To Run の「Standard」を参照。
- **Targeted supplement**: 領域固有リスクがある変更。Standard に加えて該当観点の skill を使う。
- **External supplement**: 大きい、曖昧、High-risk、または設計判断が重い変更。Standard に加えて別系統レビュー（`codex-review`）を入れる。

## Decision Criteria

- L0 で十分なケース: typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正。
- **Small 以外の実装差分は原則 `/code-review` を通す**（Standard）。避ける余地を減らす。`/code-review` は current diff / current branch を対象にする。`ultra` はクラウド・billed・ユーザー手動起動なので自動進行では使わない。
- `/code-review`（high / xhigh）は main で直接実行せず、レビュー監督 subagent に隔離する。main の context を汚さないためで、最終採否・修正・テスト・コミットは main に残す。effort の使い分け（xhigh: 永続化 / 同期 / 選択モデル / keyboard / 広い UI 挙動に触れる、または差分が大きい場合。high: docs 中心、または上記に触れない小さな差分）は `goal.md` の Goal Review に従う。
- 構造劣化リスク（巨大化、分岐増加、責務境界の濁り、薄い抽象化、型境界の曖昧さ）があれば `thermo-nuclear-code-quality-review` を**必須**で使う。
- 領域固有 supplement の対象:
  - SQLite スキーマ・マイグレーション、SQL（プレースホルダ・`GROUP BY`・集約関数・NULL 三値論理・exists_flag フィルタ）、リンク解決・ルート優先ルール（ADR 0004）、`cmd/mdhop → internal/core` の依存方向、CLI 破壊的変更（stdout JSON）、vault escape、破壊的処理（`delete --rm`, `move`, rewrite 系） → `mdhop-risk-check`
  - 永続化 / マイグレーション / 削除 / 外部連携 / 並行性 / 公開 API → `mdhop-risk-check` に加え、必要なら `codex-review`
- **テスト可能な振る舞い変更や bug fix に unit / regression test がない場合は、原則 blocker として扱う**（理由がある例外のみ許容）。
- review は粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を探す。
- 指摘に対応しない場合は、理由を plan / commit body / 該当ドキュメントに記録する。
- レビュー周回は最大 3 周。3 周で収束しなければそれ以上回さず打ち切る。打ち切った場合は残った指摘と周回数を記録し、タスク完了報告（Goal なら Goal 完了報告）で `レビュー上限超過` として通知する。

## How To Run

- L0: main で `git diff` を読み、acceptance と照合する。
- Standard: `/code-review`（high / xhigh）はレビュー監督 subagent に隔離する。main で直接 `/code-review` を実行しない。
  1. main は `Agent` ツールでレビュー監督 subagent を 1 体起動する（`model: opus`）。Opus を使うのは main の context 隔離が目的で、最終採否は main に残すため（Fable のような巨大 context 隔離ではなく、判断品質を保ちつつ context だけ切り離す）。prompt には次を渡す: 対象の commit range または diff ファイルのパス、レビュー effort（high / xhigh）、直前の実装意図メモ（3 行以内、あれば）。
  2. 監督は `/code-review` の手順を自分で実行する。finder（内部レビュー subagent）を起動する際は `model` を必ず明示する（基本 `sonnet`。判断の重い観点のみ `opus` 可）。
  3. 監督は修正を一切行わない。返すのは (a) 採用候補リスト（`file:line` / 問題 / failure scenario / 推奨対応一行）と (b) 却下リスト（指摘と却下理由）。
  4. main が最終採否を行い、修正・テスト・コミットはすべて main で行う。
  5. 再レビューは監督をもう一周起動する（最大 3 周ルールは維持。下の Decision Criteria 参照）。
- Targeted / External supplement: 該当 skill（`mdhop-risk-check`, `thermo-nuclear-code-quality-review`, `codex-review`）を呼ぶ。複数該当するものは 1 メッセージで並列起動してよい。
- 戻りを全部受け取ってから main で統合し、採用分をまとめて反映する。実行中に 1 件ずつ反映しない。

Goal 全体の commit range に対する `codex-review` と `/code-review` は、各 commit のここでの review とは別に Goal 完了条件として `goal.md` の Goal Review で実施する。

## Acceptance

以下のいずれかを満たした状態:

- レビュー指摘 0 件
- 残った指摘すべてが前回と**根拠（why）が同じ**再指摘（新規角度なら対応してレビューへ戻る）
- 最大周回数（3 周）で打ち切り、残った指摘を `レビュー上限超過` として完了報告に含める

加えて:

- 選んだ review depth と理由が説明できる
- テスト可能な振る舞い変更 / bug fix に unit / regression test がある、または追加しない理由が明確
- 指摘があれば対応済み、または対応しない理由が明確
- レビュー後の変更に対して必要な再検証が済んでいる

## Maintenance Findings

今回の差分ではなく、複数タスク後の全体構造・負債を見るレビューは `maintenance.md`（L3）で行う。

L3 はレビュー回数の数え方ではない。節目で呼ぶもの（久々に広く触った、マイルストーンの区切り、同種の修正が続いた、リファクタ候補が複数出た）。単一差分を超える構造劣化や backlog 整理が必要なら、通常レビューから自動遷移せず maintenance 候補として別タスク化する。

## Stop Conditions

- 指摘対応が仕様・CLI 挙動・設計方針を変える（複数の妥当案がある場合は即停止して確認）
- External supplement が必要なリスクなのに別系統レビューが実行できない
