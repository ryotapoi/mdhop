# Review

## Intent

変更が要求・仕様・既存設計を壊していないことを、作業リスクに応じた深さで確認する。

## Review Depth

- **L0 self-check**: Small 変更（`default.md` の Intake 分類）。自分で diff、要求、検証結果を照合する
- **L1 targeted**: 領域固有リスクがある変更。該当する専門観点だけ確認する
- **L2 external**: 大きい、曖昧、High-risk、または設計判断が重い変更。別視点レビューを入れる
- **L3 maintenance**: 今回の差分ではなく、複数タスク後の全体構造・負債を見る。`maintenance.md` を使う

L3 はレビュー回数の数え方ではない。節目で呼ぶもの（久々に広く触った、マイルストーンの区切り、同種の修正が続いた、リファクタ候補が複数出た）。

## Decision Criteria

- L0 で十分なケース: typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正
- L1 を検討する領域: SQLite スキーマ・マイグレーション、SQL（プレースホルダ・`GROUP BY`・集約関数・NULL 三値論理・exists_flag フィルタ）、リンク解決・ルート優先ルール（ADR 0004）、`cmd/mdhop → internal/core` の依存方向、CLI 破壊的変更、vault escape、`delete --rm` / `move` 等の破壊的処理、外部連携、公開 API、並行性
- L2 を検討する条件: 大きい diff、High-risk、設計判断が重い、L1 で重大な指摘が出た
- review は粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を探す
- 指摘に対応しない場合は、理由を plan / commit body / 該当ドキュメントに記録する

## How To Run

- L0: main で自分で `git diff` を読み、acceptance と照合する
- L1 / L2: `review-code-all` を呼ぶ。リスクに応じて中で mdhop / Codex 等を選別する

`review-code-all` は内部でリスクに応じた skill を呼ぶ司令塔。詳細は当該スキル参照。

## Acceptance

- 選んだ review depth と理由が説明できる
- 指摘があれば対応済み、または対応しない理由が明確
- レビュー後の変更に対して必要な再検証が済んでいる

## Stop Conditions

- 指摘対応が仕様・CLI 挙動・設計方針を変える（複数の妥当案がある場合は即停止して確認）
- レビュー周回が 3 周目に入る前 → 状況報告して指示待ち（選択肢提示しない）
- L2 が必要なリスクなのに別視点レビューが実行できない
