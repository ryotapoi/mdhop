# Review Workflow

## ICAR

- **Intent**: 完了前に、差分が要求・仕様・既存設計を壊していないことを確認する。
- **Constraints**:
  - 粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を見る。
  - 小さい変更は self-check でよい。
  - SQLite / SQL / リンク解決 / ルート優先 / vault escape / stdout JSON / 破壊的処理は、mdhop 固有観点を使う。
  - 指摘に対応しない場合は理由を残す。
- **Acceptance**:
  - 選んだレビュー深度と理由が説明できる。
  - 指摘があれば対応済み、または対応しない理由が明確。
  - レビュー後に変更した場合、必要な再検証が済んでいる。
- **Relevant**:
  - 変更差分
  - plan または要求
  - 検証結果
  - 関連する `rules/`, `references/knowledge.md`

## Depth

- **Self-check**: Small 変更。main で `git diff` を読み、要求と検証結果を照合する。
- **Targeted**: 領域固有リスクがある変更。`mdhop-risk-check` または `review-code-all` で確認する。
- **External**: 大きい、曖昧、High-risk、または設計判断が重い変更。`change-review` などの別視点を入れる。
- **Maintenance**: 今回の差分ではなく、複数タスク後の全体構造・負債を見る。`maintenance.md` を使う。

## Stop Conditions

- 指摘対応が仕様・CLI 挙動・設計方針を変える。
- 必要な別視点レビューが実行できない。
