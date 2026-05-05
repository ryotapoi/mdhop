---
name: review-plan-all
description: Use to review an implementation plan with risk-based depth before implementation. Small plans can use self-check; mdhop-specific or high-risk plans should use mdhop-risk-check, review-plan-mdhop, or change-review as needed.
argument-hint: [plan-file-path]
---

# Plan Review

## ICAR

- **Intent**: plan の事実誤認・設計劣化・検証不足・分割漏れを、作業リスクに応じた深さで見つける。
- **Constraints**:
  - 小さい変更に重いレビューを載せない。
  - 原則 1 plan = 1 commit。独立した成果が混ざるなら分割を提案する。
  - 指摘は実害・仕様逸脱・検証不足・設計劣化に絞る。
  - 判断が必要な指摘はユーザー確認に回す。
- **Acceptance**:
  - 選んだ review depth と理由が説明できる。
  - 指摘が plan に反映済み、または対応しない理由が明確。
  - 未解決の不明点がない。ある場合はユーザー確認待ちとして止まっている。
- **Relevant**:
  - plan
  - `.agents/workflow/plan.md`
  - `rules/`
  - `decisions/`
  - `references/knowledge.md`
  - `mdhop-risk-check`
  - `change-review`

## Depth

- **L0 self-check**: typo、docs、テスト追加だけ、1 ファイルの明確な修正。main で plan と acceptance を照合する。
- **L1 targeted**: 通常の機能追加・複数ファイル変更。必要に応じて `mdhop-risk-check` または `review-plan-mdhop` で mdhop 固有観点を確認する。
- **L2 external**: High-risk、設計判断が重い、曖昧、または L1 で重大な指摘が出た。L1 に加えて `change-review` などの別視点を入れる。

High-risk 例: SQLite スキーマ、SQL 意味変更、リンク解決、ルート優先ルール、stdout JSON、vault escape、`move` / `delete --rm` 等の破壊的処理、公開 API、外部連携、並行性。

## Output

- depth と理由
- 指摘一覧（MUST / SHOULD / NIT）
- plan に反映した内容、または反映しない理由
- 残リスクと検証方針
