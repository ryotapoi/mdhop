# Goal State

status: running

## Current scope

v0.7.0 の未完了タスクをすべて完了させる。

## Last completed loop

2026-05-07: `simplify` を frontmatter wikilink 対応に拡張

- `internal/core/simplify.go` の linkType フィルタを `lo.linkType != LinkTypeWikilink && lo.linkType != LinkTypeMarkdown` から `!isPathLinkType(lo.linkType)` に変更し、frontmatter wikilink も simplify の対象に取り込み
- 旧フィルタの「frontmatter_wikilink を意図的に除外」コメントを削除（`isPathLinkType` 経由で他の書き換え系コマンドと同一基準になり、コメントによる例外説明が不要になったため）
- 新 fixture `testdata/vault_simplify_frontmatter/`（Index.md + sub/B.md + sub/C.md + dir1/M.md + dir2/M.md）を追加
- `simplify_test.go` に 2 ケース追加: `TestSimplifyFrontmatterWikilink`（DryRun で書き換え対象 / Skipped 一覧を検証: alias / subpath 保持、ambiguous は skip）と `TestSimplifyFrontmatterWikilinkApplied`（実書き換えで quoted style と alias / subpath が保たれ、ambiguous は変更されず、本文 wikilink も同時に書き換わる）
- 書き換えロジック自体は既存 `rewriteRawLink` / `applyFileRewrites` が `frontmatter_wikilink` を `LinkTypeWikilink` 同等に扱うため変更不要
- `go vet ./...`（無出力）/ `go test -count=1 ./...`（cmd/mdhop と internal/core ともに ok）

## Skipped tasks

なし

## Last verification

`go vet ./...`（無出力）、`go test -count=1 ./...`（cmd/mdhop と internal/core ともに ok）

## Next hint

v0.7.0 残タスクは 2 件:
1. `frontmatter wikilink の生 raw scan が YAML コメントを edge 化する不具合`（`parse.go:399` の修正、quoted/bare 判定ロジックを共有 or 抽出）
2. `add_test.go` の既存ヘルパー検証で `frontmatter_wikilink` を含めるよう拡張（fixture 拡張 + 既存フィルタを `isPathLinkType` に統一）

次ループは順番通り (1) のバグ修正へ進む。
