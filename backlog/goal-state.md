# Goal State

status: done

## Current scope

v0.7.0 の未完了タスクをすべて完了させる → 達成

## Last completed loop

2026-05-07: `add_test.go` の既存ヘルパー検証で `frontmatter_wikilink` を含めるよう拡張

- `internal/core/add_test.go` の `TestAddAutoDisambiguateDBUpdated` (672 行) と `TestAddAutoDisambiguateRebuildConsistent` (935 行) のフィルタを `e.linkType == LinkTypeWikilink || e.linkType == LinkTypeMarkdown` から `!isPathLinkType(e.linkType)` での continue に統一。これで `frontmatter_wikilink` edge も検証網に乗る
- 新 fixture `testdata/vault_add_disambiguate_frontmatter/` を新設: `A.md` に frontmatter wikilink (`parent: "[[B]]"`) と body wikilink (`[[B]]`)、`sub/B.md` を配置。既存 `vault_add_disambiguate` を変更すると 12 箇所のテスト（特に `TestAddAutoDisambiguateBasic` の行リテラル検証 + `Rewritten = 5` 期待値）に副作用が広がるため別 vault を選択
- 新規テスト 2 本: `TestAddAutoDisambiguateFrontmatterDBUpdated` / `TestAddAutoDisambiguateFrontmatterRebuildConsistent`。`isPathLinkType` フィルタ + frontmatter_wikilink edge 件数 > 0 の二重チェックで、auto-disambiguate 後の DB edge に basename rawLink が残らないことを確認
- `go vet ./...` 無出力 / `go test -count=1 ./...` cmd/mdhop と internal/core ともに ok / `go build ./...` 成功

## Skipped tasks

なし

## Last verification

`go vet ./...`（無出力）、`go test -count=1 ./...`（cmd/mdhop と internal/core ともに ok）、`go build ./...`（成功）

## Next hint

v0.7.0 全タスク完了。`goal-done.md` を作成。次のループは v0.7.0 リリース手続き or 新バージョン計画（ユーザー指示待ち）。
