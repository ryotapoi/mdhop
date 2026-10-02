---
regen: full
sources:
  - internal/core/db.go
  - internal/core/parse.go
  - internal/core/parse_frontmatter.go
  - internal/core/resolve.go
  - internal/core/link_resolver.go
  - internal/core/build.go
  - internal/core/build_prepare.go
  - internal/core/rewrite.go
  - internal/core/repair.go
  - docs/decisions/0013-frontmatter-wikilink-detection.md
  - docs/decisions/0014-frontmatter-link-keys.md
  - docs/decisions/0023-frontmatter-wikilink-quoted-only.md
---

# LinkType 対応表

`edges.link_type` の値セットと、パース・解決・書き換えでの扱いを辿る地図。定数定義は `internal/core/db.go:43-50`。

## 定数一覧

| Go 定数 | DB 文字列値 | 対象 |
|---|---|---|
| `LinkTypeWikilink` | `"wikilink"` | 本文の `[[...]]` |
| `LinkTypeMarkdown` | `"markdown"` | 本文の `[text](url)` |
| `LinkTypeTag` | `"tag"` | 本文の `#tag` |
| `LinkTypeFrontmatter` | `"frontmatter"` | フロントマターの `tags:` キー |
| `LinkTypeFrontmatterWikilink` | `"frontmatter_wikilink"` | `tags` 以外の quoted frontmatter 値内の `[[...]]` |
| `LinkTypeFrontmatterPath` | `"frontmatter_path"` | `meta.link_keys` に指定したキーの raw path 値 |

`tagLinkTypes`（`db.go:52`）は `tag` と `frontmatter` を束ねる。

## パース箇所

| LinkType | パース関数 | ファイル:行 |
|---|---|---|
| `wikilink` | `parseWikiLinks()` | `parse.go:200` |
| `markdown` | `parseMarkdownLinks()` | `parse.go:283` |
| `tag` | `parseTags()` | `parse.go:351` |
| `frontmatter` | `parseFrontmatterTags()` → `expandFrontmatterTag()` | `parse_frontmatter.go:149` / `185` |
| `frontmatter_wikilink` | `parseFrontmatter()` → `collectFrontmatterWikilinks()` → `wikilinksFromQuotedScalar()` | `parse_frontmatter.go:39` / `83` / `101` |
| `frontmatter_path` | `frontmatterPathLinks()` → `frontmatterPathOccur()` | `parse_frontmatter.go:208` / `229` |

`parseLinks()`（`parse.go:28`）は frontmatter と本文を読み、`parseLinksWithLinkKeys()`（`parse.go:117`）だけが設定済み `link_keys` の `frontmatter_path` を追加する。edge 生成側は後者を使う（例: `build_prepare.go:71`）。

`frontmatter_wikilink` は double / single quote の scalar と、その list item だけから抽出する。bare scalar、bare list item、block scalar は除外し、`tags` は `parseFrontmatterTags` が担当する。互換性の根拠は ADR 0023（`docs/decisions/0023-frontmatter-wikilink-quoted-only.md`）。旧来の raw-text scan は ADR 0013 を参照し、現行仕様は ADR 0023 を優先する。

## resolve での解決方法

共通ディスパッチは `resolveLinkWithBackend()`（`link_resolver.go:46`）。build は `resolveLink()`（`build.go:124`）、DB を使う resolve は `resolveLinkFromDB()`（`resolve.go:91`）からここへ渡す。

| LinkType | 解決経路 |
|---|---|
| `tag` / `frontmatter` | タグとして解決（`link_resolver.go:53`、build は `build.go:137`、DB は `resolve.go:107`） |
| `wikilink` / `frontmatter_wikilink` | basename は `resolveBasename`、パス形式は `resolvePath` へ渡す |
| `markdown` / `frontmatter_path` | 相対パス、`/` 始まり、basename、vault 相対 path を共通ディスパッチで分類する |

DB 側は `resolvePathFromDB()`（`resolve.go:129`）と `resolveBasenameFromDB()`（`resolve.go:198`）、build 側は `resolvePathTarget()`（`build.go:165`）と `mapLinkResolver.resolveBasename()`（`build.go:149`）を使う。`[[#Heading]]` は自己リンクとして `resolveSelf` に渡る（`link_resolver.go:49`）。

## 書き換えと検証の対象

`rewriteLinkTypes`（`rewrite.go:16`）は `wikilink`、`markdown`、`frontmatter_wikilink` のみ。`rewriteRawLink()`（`rewrite.go:109`）はこのうち wikilink 系を同じ形式で組み立てる。

`isPathLinkType()`（`rewrite.go:25`）は上記に `frontmatter_path` も加える。これは escape / 曖昧 basename の検証対象にするためで、raw path 値そのものはリンク構文でなく書き換えられない。`frontmatter_path` の設計根拠は ADR 0014（`docs/decisions/0014-frontmatter-link-keys.md`）。

`repair` は `isBodyPathLinkType()`（`repair.go:189`）を使い、本文の `wikilink` と `markdown` のみを対象にする。quoted frontmatter wikilink は、他の rewrite 操作では対象になり得ても `repair` の対象ではない。
