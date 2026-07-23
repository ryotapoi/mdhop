---
regen: full
sources:
  - internal/core/db.go
  - internal/core/parse.go
  - internal/core/parse_frontmatter.go
  - internal/core/resolve.go
  - internal/core/link_resolver.go
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
| `frontmatter` | `expandFrontmatterTag()` (← `parseFrontmatterTags()`) | `parse_frontmatter.go:265` / `229` |
| `frontmatter_wikilink` | `parseFrontmatterWikilinks()` | `parse_frontmatter.go:182` |
| `frontmatter_path` | `frontmatterPathLinks()` → `frontmatterPathOccur()` | `parse_frontmatter.go:287` / `307` |

呼び出しの起点:

- `parseLinks()` (`parse.go:25`): wikilink / markdown / tag / frontmatter_wikilink / frontmatter を生成
- `parseLinksWithLinkKeys()` (`parse.go:81`): `parseLinks()` の結果に `frontmatter_path` を追加。build / update / add / move の辺生成サイトで使用

**タグ文字種の非対称**: `parseTags()` (`parse.go:259`) は `isTagRune` (`parse.go:235`) のブラックリストで句読点等を除外するが、`parseFrontmatterTags()` (`parse_frontmatter.go:229`) は YAML スカラー値をそのまま取り込むためブラックリスト対象の句読点も含み得る。`tag` と `frontmatter` で許容文字が揃わないが、実用上は問題にならない。

ADR:
- frontmatter_wikilink 導入経緯 → `docs/decisions/0013-frontmatter-wikilink-detection.md`
- frontmatter_path (meta.link_keys) 導入経緯 → `docs/decisions/0014-frontmatter-link-keys.md`

## resolve での解決方法

`resolve.go:91` (`resolveLinkFromDB`) / `build.go:214` (`resolveLink`) で同じロジックを辿る。

| LinkType | 解決経路 |
|---|---|
| `tag` / `frontmatter` | タグノードを直接解決。resolve 側は `tagKey(target)` (`resolve.go:108`) → `getNodeID()` (`resolve.go:109`)、build 側は `isTagLinkType()` 判定 (`link_resolver.go:102`) → `upsertTag()` (`build.go:228`) |
| `wikilink` / `frontmatter_wikilink` (basename) | `resolveBasenameFromDB()` でバックトラック解決。ルート優先ルール適用 (`build.go:246-259`) |
| `wikilink` / `frontmatter_wikilink` (vault-relative path、`/` なし) | `resolvePathFromDB()` で exact パスマッチ (`resolve.go:129`, `build.go:235`) |
| `markdown` / `frontmatter_path` (相対パス `./` or `../`) | `filepath.Join(dir, target)` で正規化後 `resolvePathFromDB()` (`link_resolver.go:109-114`) |
| `markdown` / `frontmatter_path` (絶対パス `/` prefix) | 先頭 `/` を strip して `resolvePathFromDB()` (`link_resolver.go:123-125`) |
| `markdown` / `frontmatter_path` (basename) | `resolveBasenameFromDB()` (`resolve.go:179`) |

セルフリンク `[[#Heading]]` は target="" / subpath 非空 → 自ノード ID を返す (`link_resolver.go:97-98`)。

## rewrite 対象可否と書き換え規則

`isPathLinkType()` (`rewrite.go:25`) が `true` を返す型のみが rewrite / move / disambiguate の対象候補。

| LinkType | `isPathLinkType` | `rewriteLinkTypes` | 実際の書き換え | .md 拡張子の扱い |
|---|---|---|---|---|
| `wikilink` | ✓ | ✓ | `rewriteRawLink()` で `[[新パス]]` を生成 | `.md` を除去して出力 (`rewrite.go:79`) |
| `markdown` | ✓ | ✓ | `rewriteRawLink()` で `[text](新URL)` を生成 | 元リンクの `.md` 有無を保持 (`rewrite.go:99-105`) |
| `frontmatter_wikilink` | ✓ | ✓ | `rewriteRawLink()` で `[[新パス]]` を生成（wikilink と同一ロジック、`rewrite.go:75`） | `.md` を除去して出力 |
| `frontmatter_path` | ✓ | **✗** | **書き換え不可**（raw 値はリンク構文ではない。`rewrite.go:13-14` のコメント参照） | — |
| `tag` | ✗ | ✗ | 対象外 | — |
| `frontmatter` | ✗ | ✗ | 対象外 | — |

`rewriteLinkTypes` の定義: `rewrite.go:16`。各クエリのバインド済み IN 句は `linkTypeSQLIn()` (`db.go:68`) が生成する。

repair (`repair.go`) は `isBodyPathLinkType()` (`repair.go:189`) のみ使用 → `wikilink` / `markdown` のみを対象とし、frontmatter_wikilink は含まない。

## isBasenameRawLink の実装

`rewrite.go:286`。型ごとの判定:

| LinkType | 判定ロジック |
|---|---|
| `wikilink` / `frontmatter_wikilink` | inner（alias・subpath 除去後）に `/` を含まなければ basename |
| `markdown` | URL 部分（fragment 除去後）に `/` を含まなければ basename |
| `frontmatter_path` | `frontmatterPathOccur()` に委譲（`parse_frontmatter.go:307` と同ロジック） |
| その他 | `false` |
