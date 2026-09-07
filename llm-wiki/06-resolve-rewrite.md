---
regen: compiled
sources:
  - internal/core/parse.go
  - internal/core/parse_frontmatter.go
  - internal/core/link_resolver.go
  - internal/core/frontmatter_path_guard.go
  - internal/core/meta_check.go
  - internal/core/link_ambiguity.go
  - internal/core/resolve.go
  - internal/core/resolve_maps.go
  - internal/core/rewrite.go
  - internal/core/build.go
  - internal/core/disambiguate.go
  - internal/core/move.go
  - internal/core/move_load.go
  - internal/core/move_rewrite.go
  - internal/core/move_apply.go
  - internal/core/simplify.go
  - internal/core/repair.go
  - internal/core/util.go
  - internal/core/errors.go
  - internal/core/db.go
  - internal/core/query_entry.go
  - docs/rules/03-data-model.md
  - docs/specs/overview.md
  - docs/decisions/0004-root-priority-for-ambiguous-basename.md
  - docs/decisions/0014-frontmatter-link-keys.md
  - docs/decisions/0021-shared-link-resolver-backend.md
  - docs/decisions/0023-frontmatter-wikilink-quoted-only.md
---

# リンク解決・リライトの編纂ガイド

raw link が入力されてから解決・書き換えられるまでの作業入口。リンク解釈の正本は `docs/specs/overview.md:436-454`、共通 resolver の設計判断は ADR 0021 を読む。

## 1. parse（入力 → linkOccur）

| ファイル | 関数 | 役割 |
|---|---|---|
| `parse.go:25` | `parseLinks(content)` | frontmatter と本文をパースする |
| `parse.go:81` | `parseLinksWithLinkKeys(content, linkKeys)` | `parseLinks` の結果に `frontmatter_path` を追加する |
| `parse_frontmatter.go:39` | `parseFrontmatter(lines)` | YAML node から metadata、tags、quoted-only wikilink を集める |
| `parse_frontmatter.go:83` | `collectFrontmatterWikilinks(val, offset)` | scalar または sequence を再帰し、quoted scalar だけへ進む |
| `parse_frontmatter.go:101` | `wikilinksFromQuotedScalar(val, offset)` | double / single-quoted scalar の値を `frontmatter_wikilink` にする |
| `parse_frontmatter.go:206` | `frontmatterPathLinks(meta, linkKeys)` | link_keys の raw 値を `frontmatter_path` に変換する |
| `parse_frontmatter.go:226` | `frontmatterPathOccur(value, line)` | 単一 link-key 値を分類し、空・URL・wikilink 値を除外する |

`tags` は quoted-only wikilink 抽出の対象外で、`parseFrontmatterTags` が扱う。bare scalar、bare list item、block scalar 内の `[[...]]` は `frontmatter_wikilink` を作らない。根拠と対象外の影響は ADR 0023 を読む。本文の `[[...]]` は `parseWikiLinks()`（`parse.go:142`）が別に扱う。

edge 生成サイトは `parseLinksWithLinkKeys` を使う（例: `build.go:74`）。`parseLinks` 単体は `frontmatter_path` を発生させない。

## 2. resolve（linkOccur → target node ID）

`resolveLinkWithBackend()`（`link_resolver.go:63`）が build、DB resolve、DB を変更しない path 検証の共通ディスパッチである。

| 条件 | 処理 |
|---|---|
| `target == ""` かつ subpath あり | 自己リンクとして backend の `resolveSelf` |
| tag / frontmatter 型 | backend の `resolveTag` |
| 相対パス | vault escape を検査して source の directory から解決 |
| `/` 始まり | 先頭 `/` を除去して vault 相対 path として解決 |
| basename | backend の `resolveBasename` |
| それ以外 | backend の `resolvePath` |

build は `resolveLink()`（`build.go:214`）と `mapLinkResolver`、resolve command は `resolveLinkFromDB()`（`resolve.go:91`）と `dbLinkResolver` を使う。DB の path / basename 解決は `resolve.go:129` / `179`。ルート優先は `pickBasenameMatch()`（`resolve.go:260`）と ADR 0004 を参照する。

`frontmatter_path` の dry validation は `resolveFrontmatterPathDry()`（`frontmatter_path_guard.go:88`）を使う。これは path を返すだけで phantom / tag を作らず、raw path が操作後に別の対象へ変わらないかの検証にも使う。

## 3. rewrite（rawLink → 書き換え済み rawLink → ファイル書き込み）

`rewriteLinkTypes`（`rewrite.go:12-20`）は `wikilink`、`markdown`、`frontmatter_wikilink` だけを列挙する。`isPathLinkType()`（`rewrite.go:22-31`）は `frontmatter_path` も含め、escape と曖昧 basename の検証に使う。この二つを混同しない。

| 関数 | ファイル:行 | 役割 |
|---|---|---|
| `rewriteRawLink(rawLink, linkType, targetPath)` | `rewrite.go:82` | link 構文の target 部分を新パスへ置換する |
| `applyFileRewritesWithRollbackFailures(vaultPath, rewrites)` | `rewrite.go:199` | 全ファイルの候補を検証してから書き込み、失敗時は rollback する |
| `isBasenameRawLink(rawLink, linkType)` | `rewrite.go:314` | raw link が basename 形式かを判定する |
| `rewriteOutgoingRelativeLink(rawLink, linkType, from, to, movedFromTo)` | `move_rewrite.go:489` | moved file 内の相対 link を移動後の起点から再計算する |

quoted frontmatter wikilink は `rewriteLinkTypes` に含まれ、rewrite entry の `rawLink` は `rewriteRawLink` の wikilink 系分岐で変換される。実更新では `rewriteFrontmatterCandidate` を使い、全候補の source/decode 対応を確認してから書き込む。move は外部ファイルと移動ノートの候補を一括準備する。対応を証明できない候補は副作用前に操作全体を拒否する。`frontmatter_path` は raw 値なので書き換えない。操作前検証と手動更新が必要になる理由は ADR 0014 を参照する。

`repair` は `isBodyPathLinkType()`（`repair.go:186`）で本文の `wikilink` と `markdown` に限定する。frontmatter wikilink を `repair` が直すとは読まない。

## 4. 変更時の読むべき場所

- quoted-only frontmatter 抽出: `parse_frontmatter.go:39`、`83`、`101` と ADR 0023
- link_keys の raw path: `parse_frontmatter.go:206`、`226`、`frontmatter_path_guard.go:18` と ADR 0014
- 共通 resolve dispatch: `link_resolver.go:63`、build は `build.go:214`、DB resolve は `resolve.go:91`
- path / basename の DB 解決: `resolve.go:129`、`179`、ルート優先は `resolve.go:260` と ADR 0004
- move の incoming rewrite: `move_rewrite.go:90`、moved file の outgoing rewrite: `move_rewrite.go:260`
- rewrite の型集合と raw link 変換: `rewrite.go:12`、`73`、`293`
- repair の本文限定: `repair.go:186`

## 5. 正本へのポインタ

- リンク解釈と frontmatter の互換性: `docs/specs/overview.md:436-454`
- ルート優先: `docs/decisions/0004-root-priority-for-ambiguous-basename.md`
- frontmatter raw path (`link_keys`): `docs/decisions/0014-frontmatter-link-keys.md`
- quoted-only frontmatter wikilink: `docs/decisions/0023-frontmatter-wikilink-quoted-only.md`
- DB schema: `docs/rules/03-data-model.md`
