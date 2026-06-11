# Review

## Intent

変更が要求・仕様・既存設計を壊していないことを、作業リスクに応じた深さで確認する。実装後レビューを標準とする。

## Review Depth

- **L0 self-check**: Small 変更（`default.md` の Intake 分類）。main で `git diff` を読み、要求と検証結果を照合する。skill は呼ばない。
- **Standard**: Small 以外の実装差分。`/code-review xhigh` をローカル実行し、結果を見て直す（`--fix` は付けず指摘を受け取り、採否判断して反映）。
- **Targeted supplement**: 領域固有リスクがある変更。Standard に加えて該当観点の skill を使う。
- **External supplement**: 大きい、曖昧、High-risk、または設計判断が重い変更。Standard に加えて別系統レビュー（`codex-review`）を入れる。

## Decision Criteria

- L0 で十分なケース: typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正。
- **Small 以外の実装差分は原則 `/code-review xhigh` を通す**（Standard）。避ける余地を減らす。`/code-review` は current diff / current branch を対象にする。`ultra` はクラウド・billed・ユーザー手動起動なので自動進行では使わない。
- 構造劣化リスク（巨大化、分岐増加、責務境界の濁り、薄い抽象化、型境界の曖昧さ）があれば `thermo-nuclear-code-quality-review` を使う。
- 領域固有 supplement の対象:
  - SQLite スキーマ・マイグレーション、SQL（プレースホルダ・`GROUP BY`・集約関数・NULL 三値論理・exists_flag フィルタ）、リンク解決・ルート優先ルール（ADR 0004）、`cmd/mdhop → internal/core` の依存方向、CLI 破壊的変更（stdout JSON）、vault escape、破壊的処理（`delete --rm`, `move`, rewrite 系） → `mdhop-risk-check`
  - 永続化 / マイグレーション / 削除 / 外部連携 / 並行性 / 公開 API → `mdhop-risk-check` に加え、必要なら `codex-review`
- diff が 1000 行を超える場合は、`codex-review` を実行する前にレート制限リスクをユーザーに確認する。
- **テスト可能な振る舞い変更や bug fix に unit / regression test がない場合は、原則 blocker として扱う**（理由がある例外のみ許容）。
- review は粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を探す。
- 指摘に対応しない場合は、理由を plan / commit body / 該当ドキュメントに記録する。
- レビュー周回が 3 周目以降に入っても止まらない。超過の事実（周回数・要因となった指摘・収束結果）を記録し、タスク完了報告（Goal なら Goal 完了報告）で `レビュー上限超過` として通知する。

## How To Run

- L0: main で `git diff` を読み、acceptance と照合する。
- Standard: `/code-review xhigh` を実行し、戻ってきた指摘を採否判断して反映する。
- Targeted / External supplement: 該当 skill（`mdhop-risk-check`, `thermo-nuclear-code-quality-review`, `codex-review`）を呼ぶ。複数該当するものは 1 メッセージで並列起動してよい。
- 戻りを全部受け取ってから main で統合し、採用分をまとめて反映する。実行中に 1 件ずつ反映しない。

Goal 全体の commit range に対する `codex-review` と `/code-review` は、各 commit のここでの review とは別に Goal 完了条件として `goal.md` の Goal Review で実施する。

## Acceptance

以下のどちらかを満たした状態:

- レビュー指摘 0 件
- 残った指摘すべてが前回と**根拠（why）が同じ**再指摘（新規角度なら対応してレビューへ戻る）

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
- レビュー周回を重ねても対応必須の指摘が解消も却下もできず、収束の見込みがない
- External supplement が必要なリスクなのに別系統レビューが実行できない
