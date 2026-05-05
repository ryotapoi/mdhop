---
name: mdhop-risk-check
description: Use for mdhop-specific plan or implementation checks when changes touch CLI behavior, SQLite/SQL, link resolution, root-priority rules, vault paths, rewrite/mutate commands, stdout JSON, or module boundaries.
---

# mdhop Risk Check

## ICAR

- **Intent**: mdhop 固有のプロダクト制約・アーキテクチャ制約・既知の落とし穴に照らして、計画または実装のリスクを確認する。
- **Constraints**:
  - 汎用レビューではなく、mdhop 固有の実害に絞る。
  - 仕様・CLI 挙動判断が必要なら、実装判断として決めずユーザー確認に回す。
  - 具体的な過去知見は `references/knowledge.md` を参照し、skill 本体には増やしすぎない。
- **Acceptance**:
  - `LGTM` またはリスク一覧がある。
  - リスクには影響、根拠、推奨対応がある。
  - 必要な場合、更新すべき `rules/`, `backlog/backlog.md`, `decisions/`, `references/knowledge.md`, `specs/` が明確。
- **Relevant**:
  - ユーザー依頼、plan、または未コミット差分
  - `rules/01-concept.md`
  - `rules/02-requirements.md`
  - `rules/03-data-model.md`
  - `rules/overview.md`
  - `rules/architecture.md`
  - `rules/information-management.md`
  - `references/knowledge.md`

## Checkpoints

- CLI 表面仕様が `rules/overview.md` と矛盾しないか。
- stdout JSON に未定義フィールドや warnings を混ぜていないか。
- SQLite スキーマ、SQL、NULL 三値論理、`exists_flag`、`modernc.org/sqlite` の既知の罠を踏んでいないか。
- Obsidian 互換の wikilink / markdown link / tag / frontmatter 解釈を壊していないか。
- ルート優先ルール、basename 曖昧性、phantom/tag/asset の扱いに影響がないか。
- `move`, `delete --rm`, rewrite 系でディスク操作の順序・対象・復元性を壊していないか。
- vault escape や絶対パスを安全に拒否できているか。
- `cmd/mdhop → internal/core` の依存方向と責務分離を守っているか。
- DB に Markdown 本文 TEXT を保存しない方針を崩していないか。
- 変更した仕様に対して、build / add / update / delete / move / query / resolve など影響するコマンドのテスト計画があるか。
