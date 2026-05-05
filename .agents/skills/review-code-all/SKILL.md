---
name: review-code-all
description: Use to review implementation diffs with risk-based depth after implementation and verification. Small changes can use self-check; mdhop-specific or high-risk changes should use mdhop-risk-check, review-code-mdhop, or change-review as needed.
argument-hint: [plan-file-path]
---

# Code Review

## ICAR

- **Intent**: 実装差分が要求・仕様・既存設計を壊していないことを、作業リスクに応じた深さで確認する。
- **Constraints**:
  - 粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を見る。
  - 小さい変更に重いレビューを載せない。
  - 指摘に対応しない場合は理由を残す。
  - レビュー後に変更したら、必要な検証を再実行する。
- **Acceptance**:
  - 選んだ review depth と理由が説明できる。
  - 指摘があれば対応済み、または対応しない理由が明確。
  - レビュー後の変更に対して必要な再検証が済んでいる。
- **Relevant**:
  - `git diff`, `git diff --cached`
  - plan またはユーザー要求
  - 検証結果
  - `.agents/workflow/review.md`
  - `rules/`
  - `references/knowledge.md`
  - `mdhop-risk-check`
  - `change-review`

## Depth

- **L0 self-check**: typo、docs、テスト追加だけ、1 ファイルの明確な修正。main で diff、要求、検証結果を照合する。
- **L1 targeted**: 通常の機能追加・複数ファイル変更。必要に応じて `mdhop-risk-check` または `review-code-mdhop` で mdhop 固有観点を確認する。
- **L2 external**: High-risk、大きい diff、設計判断が重い、曖昧、または L1 で重大な指摘が出た。L1 に加えて `change-review` などの別視点を入れる。

High-risk 例: SQLite スキーマ、SQL 意味変更、リンク解決、ルート優先ルール、stdout JSON、vault escape、`move` / `delete --rm` 等の破壊的処理、公開 API、外部連携、並行性。

## Checks

- `go build ./...` と `go test ./...` が通っているか。変更リスクに応じて `go vet ./...` も実行する。
- CLI 表面仕様変更では `bin/mdhop <args>` で stdout / stderr / 終了コードを確認したか。
- 破壊的処理は testdata 等の一時 vault で確認したか。
- 仕様変更は `rules/overview.md`、派生ドキュメント、テストと整合しているか。

## Output

- depth と理由
- 指摘一覧（MUST / SHOULD / NIT）
- 対応内容、または対応しない理由
- 実行した再検証
