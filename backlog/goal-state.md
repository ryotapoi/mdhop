# Goal State

status: running

## Current scope

v0.7.0 の未完了タスクをすべて完了させる。

## Last completed loop

2026-05-07: frontmatter wikilink の生 raw scan が YAML コメントを edge 化する不具合を修正

- `internal/core/parse.go` の `parseFrontmatterWikilinks` で各行を `parseWikiLinks` に渡す前に `stripYAMLComment` でコメント部分を除去するように変更
- `stripYAMLComment` ヘルパーを `parse.go` に新設: 行を rune ごとに走査し、`'`/`"` quoted の外で「行頭 or 直前が space/tab」の `#` 以降を切り捨てる。quoted 内の `#`（例: `"[[B#Heading]]"`）は保持
- block scalar (`|`, `>`) や複数行 quoted は今回のスコープ外（タスク説明も「quoted/bare の判定」とだけ書かれている）。実害が出たら別タスクで対応
- `parse_test.go` に 3 ケース追加: `TestParseFrontmatterWikilinkIgnoresBareComment`（`related: ok # [[B]] comment` で B を edge にしない、別行の quoted `[[C]]` は edge 化）、`TestParseFrontmatterWikilinkKeepsHashInsideQuoted`（`"[[B#Heading]]"` の subpath 保持）、`TestParseFrontmatterWikilinkCommentAfterQuoted`（quoted 後の ` # ... [[X]]` は除外）
- 既存の `TestParseFrontmatterWikilinkSubpath` 等の quoted 内 `#` ケースは引き続き通る（subpath 保持を確認済み）
- `go vet ./...`（無出力）/ `go test -count=1 ./...`（cmd/mdhop と internal/core ともに ok）

## Skipped tasks

なし

## Last verification

`go vet ./...`（無出力）、`go test -count=1 ./...`（cmd/mdhop と internal/core ともに ok）

## Next hint

v0.7.0 残タスクは 1 件:
1. `add_test.go` の既存ヘルパー検証で `frontmatter_wikilink` を含めるよう拡張（fixture 拡張 + 既存フィルタを `isPathLinkType` に統一）

次ループはこれで v0.7.0 完了 → `goal-done.md` 作成。
