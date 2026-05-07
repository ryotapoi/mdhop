# Goal State

status: running

## Current scope

v0.7.0 の未完了タスクをすべて完了させる。

## Last completed loop

2026-05-07: `linkType` 値を `type LinkType string` の named type に昇格

- `internal/core/db.go` に `type LinkType string` と 5 定数（`LinkTypeWikilink` / `LinkTypeMarkdown` / `LinkTypeTag` / `LinkTypeFrontmatter` / `LinkTypeFrontmatterWikilink`）を追加
- `linkOccur.linkType` / `rewriteEntry.linkType` / `edgeRow.linkType`（テスト）を `LinkType` に昇格
- `rewriteRawLink` / `isBasenameRawLink` / `isPathLinkType` / `rewriteOutgoingRelativeLink` / `insertEdge` のシグネチャを `LinkType` に揃える
- 核モジュール（parse/build/resolve/rewrite）+ 書き換え系（add/update/move/move_helpers/move_dir/disambiguate/simplify/repair/convert）の文字列リテラル比較・代入を定数参照に置換
- テスト（parse_test/build_test/move_test/update_test/add_test/rewrite_test）の `==` / `!=` リテラル比較も定数化
- `pathLinkTypeSQLList` は SQL 直埋めのため文字列のまま据置（コメント整合）
- `go vet ./...` / `go test -count=1 ./...` ともに pass

## Skipped tasks

なし

## Last verification

`go vet ./...`（無出力）、`go test -count=1 ./...`（cmd/mdhop と internal/core ともに ok）

## Next hint

次のループでは backlog v0.7.0 の次タスク「`simplify` を frontmatter wikilink 対応に拡張」へ進む。
