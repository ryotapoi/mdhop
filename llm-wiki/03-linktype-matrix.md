---
regen: full
sources:
  - internal/core/db.go
  - internal/core/parse.go
  - internal/core/parse_reference.go
  - internal/core/markdown_destination.go
  - internal/core/parse_table.go
  - internal/core/parse_frontmatter.go
  - internal/core/resolve.go
  - internal/core/link_resolver.go
  - internal/core/build.go
  - internal/core/build_prepare.go
  - internal/core/rewrite.go
  - internal/core/repair.go
  - internal/core/frontmatter_path_guard.go
  - docs/decisions/0013-frontmatter-wikilink-detection.md
  - docs/decisions/0014-frontmatter-link-keys.md
  - docs/decisions/0023-frontmatter-wikilink-quoted-only.md
---

# LinkType 対応表

`edges.link_type` の値セットと、パース・解決・書き換えでの扱いを辿る地図。定数定義は `internal/core/db.go:43-51`。

## 定数一覧

| Go 定数 | DB 文字列値 | 対象 |
|---|---|---|
| `LinkTypeWikilink` | `"wikilink"` | 本文の `[[...]]` |
| `LinkTypeMarkdown` | `"markdown"` | 本文の `[text](url)` |
| `LinkTypeMarkdownReference` | `"markdown_reference"` | 本文の参照リンク使用箇所 |
| `LinkTypeTag` | `"tag"` | 本文の `#tag` |
| `LinkTypeFrontmatter` | `"frontmatter"` | フロントマターの `tags:` キー |
| `LinkTypeFrontmatterWikilink` | `"frontmatter_wikilink"` | `tags` 以外の quoted frontmatter 値内の `[[...]]` |
| `LinkTypeFrontmatterPath` | `"frontmatter_path"` | `meta.link_keys` に指定したキーの raw path 値 |

`tagLinkTypes`（`db.go:52`）は `tag` と `frontmatter` を束ねる。

## パース箇所

| LinkType | パース関数 | ファイル:行 |
|---|---|---|
| `wikilink` | `bodyTableLines()` → `parseWikiLinks()` | `parse_table.go:7` / `parse.go:222` |
| `markdown` | `parseMarkdownLinks()` → `markdownDestinationOccur()` → `markdownDestination()` | `parse.go:310` / `parse_reference.go:125` / `markdown_destination.go:115` |
| `markdown_reference` | `referenceDefinition()` → `parseBodyMarkdown()` → `markdownDestinationOccur()` | `parse_reference.go:11` / `139` / `125` |
| `tag` | `parseTags()` | `parse.go:350` |
| `frontmatter` | `parseFrontmatterTags()` → `expandFrontmatterTag()` | `parse_frontmatter.go:149` / `185` |
| `frontmatter_wikilink` | `parseFrontmatter()` → `collectFrontmatterWikilinks()` → `wikilinksFromQuotedScalar()` | `parse_frontmatter.go:39` / `83` / `101` |
| `frontmatter_path` | `frontmatterPathLinks()` → `frontmatterPathOccur()` | `parse_frontmatter.go:208` / `229` |

`parseLinks()`（`parse.go:30`）は frontmatter と本文を読み、`parseLinksWithLinkKeys()`（`parse.go:153`）だけが設定済み `link_keys` の `frontmatter_path` を追加する。edge 生成側は後者を使う（例: `build_prepare.go:71`）。

参照定義は `parse.go:43` で先に収集し、使用箇所から `markdown_reference` edge を作る。DB の `reference_target` に未復号の destination 原文、`in_table` に表文脈を保存する（`docs/rules/03-data-model.md`）。

`frontmatter_wikilink` は double / single quote の scalar と、その list item だけから抽出する。bare scalar、bare list item、block scalar は除外し、`tags` は `parseFrontmatterTags` が担当する。互換性の根拠は ADR 0023（`docs/decisions/0023-frontmatter-wikilink-quoted-only.md`）。旧来の raw-text scan は ADR 0013 を参照し、現行仕様は ADR 0023 を優先する。

## resolve での解決方法

resolve command は `resolve.go` の `Resolve()` で source と exact raw が一致する保存 edge を先に読む。異義重複の判定は同関数、exact raw がない場合の正規化一致は `resolveLinkFromDB()` → `edgeExists()` を辿る。

共通ディスパッチは `resolveLinkWithBackend()`（`link_resolver.go:46`）。build は `resolveLink()`（`build.go:124`）、DB を使う正規化 resolve は `resolveLinkFromDB()`（`resolve.go`）からここへ渡す。

| LinkType | 解決経路 |
|---|---|
| `tag` / `frontmatter` | タグとして解決（`link_resolver.go:53`、build は `build.go:137`、DB は `resolve.go:143`） |
| `wikilink` / `frontmatter_wikilink` | basename は `resolveBasename`、パス形式は `resolvePath` へ渡す |
| `markdown` / `frontmatter_path` | 相対パス、`/` 始まり、basename、vault 相対 path を共通ディスパッチで分類する |
| `markdown_reference` | build 時は保存先 destination を共通ディスパッチで解決。resolve command は `Resolve()` の exact raw 経路のみを使う |

DB 側は `resolvePathFromDB()`（`resolve.go:165`）と `resolveBasenameFromDB()`（`resolve.go:234`）、build 側は `resolvePathTarget()`（`build.go:165`）と `mapLinkResolver.resolveBasename()`（`build.go:149`）を使う。`[[#Heading]]` は自己リンクとして `resolveSelf` に渡る（`link_resolver.go:49`）。

## 書き換えと検証の対象

`rewriteLinkTypes`（`rewrite.go:17`）は `wikilink`、`markdown`、`frontmatter_wikilink` のみ。`rewriteRawLink()`（`rewrite.go:113`）は表文脈付きの wikilink を組み立て、Markdown destination の再出力は `encodeMarkdownDestination()`（`markdown_destination.go:146`）へ渡す。

`isPathLinkType()`（`rewrite.go:26`）は上記に `frontmatter_path` も加える。これは escape / 曖昧 basename の検証対象にするためで、raw path 値そのものはリンク構文でなく書き換えられない。`frontmatter_path` の設計根拠は ADR 0014（`docs/decisions/0014-frontmatter-link-keys.md`）。

`markdown_reference` も `isPathLinkType()` に含まれるが、`rewriteLinkTypes` には含まれない。move/add は `frontmatter_path_guard.go:19` で保存 destination の解決先を検証し、定義の rewrite が必要な操作を拒否する。

`repair` は `isBodyPathLinkType()`（`repair.go:189`）を使い、本文の `wikilink` と `markdown` のみを対象にする。quoted frontmatter wikilink は、他の rewrite 操作では対象になり得ても `repair` の対象ではない。
