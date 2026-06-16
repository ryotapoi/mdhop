---
regen: full
sources:
  - internal/core/db.go
  - internal/core/parse.go
  - internal/core/parse_frontmatter.go
  - internal/core/resolve.go
  - internal/core/rewrite.go
  - internal/core/repair.go
  - docs/decisions/0013-frontmatter-wikilink-detection.md
  - docs/decisions/0014-frontmatter-link-keys.md
---

# LinkType 対応表

`edges.link_type` 列の値セット。定数定義: `internal/core/db.go:44-51`

## 定数一覧

| Go 定数 | DB 文字列値 | 対象 |
|---|---|---|
| `LinkTypeWikilink` | `"wikilink"` | 本文の `[[...]]` |
| `LinkTypeMarkdown` | `"markdown"` | 本文の `[text](url)` |
| `LinkTypeTag` | `"tag"` | 本文の `#tag` |
| `LinkTypeFrontmatter` | `"frontmatter"` | フロントマターの `tags:` キー |
| `LinkTypeFrontmatterWikilink` | `"frontmatter_wikilink"` | フロントマター内の `[[...]]` |
| `LinkTypeFrontmatterPath` | `"frontmatter_path"` | フロントマターのリンクキー値（パス文字列） |

`tagLinkTypes` スライス (`db.go:53`) は `[tag, frontmatter]` の 2 種を束ねる。

## パース箇所

| LinkType | パース関数 | ファイル:行 |
|---|---|---|
| `wikilink` | `parseWikiLinks()` | `parse.go:142` |
| `markdown` | `parseMarkdownLinks()` | `parse.go:190` |
| `tag` | `parseTags()` | `parse.go:259` |
| `frontmatter` | `expandFrontmatterTag()` (← `parseFrontmatterTags()`) | `parse_frontmatter.go:264` / `229` |
| `frontmatter_wikilink` | `parseFrontmatterWikilinks()` | `parse_frontmatter.go:182` |
| `frontmatter_path` | `frontmatterPathLinks()` → `frontmatterPathOccur()` | `parse_frontmatter.go:287` / `307` |

呼び出しの起点:

- `parseLinks()` (`parse.go:25`): wikilink / markdown / tag / frontmatter_wikilink / frontmatter を生成
- `parseLinksWithLinkKeys()` (`parse.go:81`): `parseLinks()` の結果に `frontmatter_path` を追加。build / update / add / move の辺生成サイトで使用

ADR:
- frontmatter_wikilink 導入経緯 → `docs/decisions/0013-frontmatter-wikilink-detection.md`
- frontmatter_path (meta.link_keys) 導入経緯 → `docs/decisions/0014-frontmatter-link-keys.md`

## resolve での解決方法

`resolve.go:83` (`resolveLinkFromDB`) / `build.go:207` (`resolveLink`) で同じロジックを辿る。

| LinkType | 解決経路 |
|---|---|
| `tag` / `frontmatter` | タグノードを直接解決。resolve 側は `tagKey(target)` で lookup (`resolve.go:94`)、build 側は `isTagLinkType()` 判定 (`build.go:215`) → `upsertTag()` (`build.go:216`) |
| `wikilink` / `frontmatter_wikilink` (basename) | `resolveBasenameFromDB()` でバックトラック解決。ルート優先ルール適用 (`build.go:251-260`) |
| `wikilink` / `frontmatter_wikilink` (vault-relative path、`/` なし) | `resolvePathFromDB()` で exact パスマッチ (`resolve.go:129`, `build.go:246`) |
| `markdown` / `frontmatter_path` (相対パス `./` or `../`) | `filepath.Join(dir, target)` で正規化後 `resolvePathFromDB()` (`resolve.go:109`) |
| `markdown` / `frontmatter_path` (絶対パス `/` prefix) | 先頭 `/` を strip して `resolvePathFromDB()` (`resolve.go:123`) |
| `markdown` / `frontmatter_path` (basename) | `resolveBasenameFromDB()` (`resolve.go:134`) |

セルフリンク `[[#Heading]]` は target="" / subpath 非空 → 自ノード ID を返す (`resolve.go:84`)。

## rewrite 対象可否と書き換え規則

`isPathLinkType()` (`rewrite.go:20`) が `true` を返す型のみが rewrite / move / disambiguate の対象候補。

| LinkType | `isPathLinkType` | SQL `pathLinkTypeSQLList` | 実際の書き換え | .md 拡張子の扱い |
|---|---|---|---|---|
| `wikilink` | ✓ | ✓ | `rewriteRawLink()` で `[[新パス]]` を生成 | `.md` を除去して出力 (`rewrite.go:74`) |
| `markdown` | ✓ | ✓ | `rewriteRawLink()` で `[text](新URL)` を生成 | 元リンクの `.md` 有無を保持 (`rewrite.go:94-100`) |
| `frontmatter_wikilink` | ✓ | ✓ | `rewriteRawLink()` で `[[新パス]]` を生成（wikilink と同一ロジック、`rewrite.go:58`） | `.md` を除去して出力 |
| `frontmatter_path` | ✓ | **✗** | **書き換え不可**（raw 値はリンク構文ではない。`rewrite.go:13-14` のコメント参照） | — |
| `tag` | ✗ | ✗ | 対象外 | — |
| `frontmatter` | ✗ | ✗ | 対象外 | — |

`pathLinkTypeSQLList` の定義: `rewrite.go:15` （`'wikilink', 'markdown', 'frontmatter_wikilink'`）

repair (`repair.go`) は `isBodyPathLinkType()` (`repair.go:219`) のみ使用 → `wikilink` / `markdown` のみを対象とし、frontmatter_wikilink は含まない。

## isBasenameRawLink の実装

`rewrite.go:231`。型ごとの判定:

| LinkType | 判定ロジック |
|---|---|
| `wikilink` / `frontmatter_wikilink` | inner（alias・subpath 除去後）に `/` を含まなければ basename |
| `markdown` | URL 部分（fragment 除去後）に `/` を含まなければ basename |
| `frontmatter_path` | `frontmatterPathOccur()` に委譲（`parse_frontmatter.go:307` と同ロジック） |
| その他 | `false` |
