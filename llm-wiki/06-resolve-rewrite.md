---
regen: compiled
sources:
  - internal/core/parse.go
  - internal/core/parse_reference.go
  - internal/core/markdown_destination.go
  - internal/core/parse_table.go
  - internal/core/parse_frontmatter.go
  - internal/core/link_resolver.go
  - internal/core/frontmatter_path_guard.go
  - internal/core/meta_check.go
  - internal/core/link_ambiguity.go
  - internal/core/resolve.go
  - internal/core/resolve_maps.go
  - internal/core/rewrite.go
  - internal/core/rewrite_frontmatter.go
  - internal/core/build.go
  - internal/core/disambiguate.go
  - internal/core/move.go
  - internal/core/move_load.go
  - internal/core/move_rewrite.go
  - internal/core/move_apply.go
  - internal/core/move_link.go
  - internal/core/simplify.go
  - internal/core/repair.go
  - internal/core/util.go
  - internal/core/errors.go
  - internal/core/db.go
  - internal/core/query_entry.go
  - internal/core/convert.go
  - docs/rules/03-data-model.md
  - docs/specs/overview.md
  - docs/decisions/0004-root-priority-for-ambiguous-basename.md
  - docs/decisions/0014-frontmatter-link-keys.md
  - docs/decisions/0021-shared-link-resolver-backend.md
  - docs/decisions/0023-frontmatter-wikilink-quoted-only.md
---

# リンク解決・リライトの編纂ガイド

raw link が入力されてから解決・書き換えられるまでの作業入口。リンク解釈の正本は `docs/specs/overview.md:471`、共通 resolver の設計判断は ADR 0021 を読む。

## 1. parse（入力 → linkOccur）

| ファイル | 関数 | 役割 |
|---|---|---|
| `markdown_destination.go` | `markdownDestination()` / `encodeMarkdownDestination()` | 原文の lexical 復号、fragment 分離、単発 percent decode と意味を保つ再出力 |
| `parse_table.go` | `bodyTableLines()` | raw の位置を保ち、本文 scanner の対象行で表文脈を認識する |
| `parse.go:30` | `parseLinks(content)` | frontmatter と本文をパースする |
| `parse_reference.go:11` / `122` | `referenceDefinition()` / `parseBodyMarkdown()` | 同一文書の定義を先に集め、使用箇所を destination へ対応させる |
| `parse.go:148` | `parseLinksWithLinkKeys(content, linkKeys)` | `parseLinks` の結果に `frontmatter_path` を追加する |
| `parse_frontmatter.go:39` | `parseFrontmatter(lines)` | YAML node から metadata、tags、quoted-only wikilink を集める |
| `parse_frontmatter.go:83` | `collectFrontmatterWikilinks(key, val, offset)` | scalar または sequence を再帰し、quoted scalar だけへ進む |
| `parse_frontmatter.go:101` | `wikilinksFromQuotedScalar(key, val, offset)` | double / single-quoted scalar の値を `frontmatter_wikilink` にする |
| `parse_frontmatter.go:208` | `frontmatterPathLinks(meta, linkKeys)` | link_keys の raw 値を `frontmatter_path` に変換する |
| `parse_frontmatter.go:229` | `frontmatterPathOccur(value, line)` | 単一 link-key 値を分類し、空・URL・wikilink 値を除外する |

`tags` は quoted-only wikilink 抽出の対象外で、`parseFrontmatterTags` が扱う。bare scalar、bare list item、block scalar 内の `[[...]]` は `frontmatter_wikilink` を作らない。根拠と対象外の影響は ADR 0023 を読む。本文の `[[...]]` は `parseWikiLinks()`（`parse.go:215`）が別に扱う。

edge 生成サイトは `parseLinksWithLinkKeys` を使う（例: `build_prepare.go:71`）。`parseLinks` 単体は `frontmatter_path` を発生させない。

参照リンクの使用原文は `edges.raw_link`、未復号の定義 destination は `edges.reference_target`、表文脈は `edges.in_table` に残す。decoded target/subpath と raw を混ぜない。定義だけの行は edge にしない。構文と対象外は `docs/specs/overview.md` の「リンク構文・タグ構文」を参照する。

## 2. resolve（linkOccur → target node ID）

`resolveLinkWithBackend()`（`link_resolver.go:46`）が build、DB resolve、DB を変更しない path 検証の共通ディスパッチである。

| 条件 | 処理 |
|---|---|
| `target == ""` かつ subpath あり | 自己リンクとして backend の `resolveSelf` |
| tag / frontmatter 型 | backend の `resolveTag` |
| 相対パス | vault escape を検査して source の directory から解決 |
| `/` 始まり | 先頭 `/` を除去して vault 相対 path として解決 |
| basename | backend の `resolveBasename` |
| それ以外 | backend の `resolvePath` |

build は `resolveLink()`（`build.go:124`）と `mapLinkResolver`、resolve command は `resolveLinkFromDB()`（`resolve.go:106`）と `dbLinkResolver` を使う。DB の path / basename 解決は `resolve.go:144` / `213`。ルート優先は `pickBasenameMatch()`（`resolve.go:294`）と ADR 0004 を参照する。

resolve command は `resolve.go` から source と exact raw が一致する保存 edge を優先する。同義重複は解決し、異義重複は曖昧エラーにする。表文脈と定義は再走査しない。exact raw がない場合だけ従来の正規化解決へ進む（reference は exact raw のみ）。保存 destination の通常解決と phantom promotion は `parse_reference.go:108`、`move_apply.go:126` を辿る。

`frontmatter_path` の dry validation は `resolveFrontmatterPathDry()`（`frontmatter_path_guard.go:99`）を使う。これは path を返すだけで phantom / tag を作らず、raw path が操作後に別の対象へ変わらないかの検証にも使う。

## 3. rewrite（rawLink → 書き換え済み rawLink → ファイル書き込み）

`rewriteLinkTypes`（`rewrite.go:17`）は `wikilink`、`markdown`、`frontmatter_wikilink` だけを列挙する。`isPathLinkType()`（`rewrite.go:26`）は `frontmatter_path` も含め、escape と曖昧 basename の検証に使う。この二つを混同しない。

`markdown_reference` も `isPathLinkType()` に含まれるが rewrite 対象ではない。add/move の事前検証は `frontmatter_path_guard.go:19` を使い、保存 destination の解決先が変わる操作を拒否する。`convert.go:255` の自己リンク追加走査も参照定義行を飛ばす。

| 関数 | ファイル:行 | 役割 |
|---|---|---|
| `rewriteRawLink(rawLink, linkType, targetPath, table...)` | `rewrite.go:112` | Markdown の decoded fragment を保ち、path/fragment を encode して置換する。表内 wiki alias は context 付きで保持する |
| `applyFileRewritesWithRollbackFailures(vaultPath, rewrites)` | `rewrite.go:231` | 全ファイルの候補を検証してから書き込み、失敗時は rollback する |
| `isBasenameRawLink(rawLink, linkType, table...)` | `rewrite.go:357` | Markdown の decoded target / context 付き wiki target の basename 判定 |
| `rewriteMovedOutgoingLink(link, from, to, preMoveTargetPath, maps)` | `move_link.go:22` | moved file の outgoing link を移動後の解決情報で判定する |
| `rewriteOutgoingRelativeLink(rawLink, linkType, from, to, movedFromTo, preMoveTargetPath)` | `move_link.go:144` | moved file 内の相対 link を移動後の起点から再計算する |

quoted frontmatter wikilink は `rewriteLinkTypes` に含まれ、rewrite entry の `rawLink` は `rewriteRawLink` の wikilink 系分岐で変換される。実更新では `rewriteFrontmatterCandidate` を使い、全候補の source/decode 対応を確認してから書き込む。move は外部ファイルと移動ノートの候補を一括準備する。対応を証明できない候補は副作用前に操作全体を拒否する。`frontmatter_path` は raw 値なので書き換えない。操作前検証と手動更新が必要になる理由は ADR 0014 を参照する。

`repair` は `isBodyPathLinkType()`（`repair.go:189`）で本文の `wikilink` と `markdown` に限定する。frontmatter wikilink を `repair` が直すとは読まない。

## 4. 変更時の読むべき場所

- quoted-only frontmatter 抽出: `parse_frontmatter.go:39`、`83`、`101` と ADR 0023
- Markdown 参照リンク: `parse.go:43`、`parse_reference.go:11` / `108` / `122`、`resolve.go:49`、`frontmatter_path_guard.go:19`
- convert の Markdown 自己リンク: `convert.go:255` / `271`。括弧付き fragment の検証は `convert_test.go`、仕様は `docs/specs/overview.md`
- link_keys の raw path: `parse_frontmatter.go:208`、`229`、`frontmatter_path_guard.go:19` と ADR 0014
- 共通 resolve dispatch: `link_resolver.go:46`、build は `build.go:124`、DB resolve は `resolve.go:106`
- path / basename の DB 解決: `resolve.go:144`、`213`、ルート優先は `resolve.go:294` と ADR 0004
- move の incoming rewrite: `move_rewrite.go:95`、moved file の outgoing 収集: `move_rewrite.go:251`、個別判定: `move_link.go:22` / `144`
- rewrite の型集合と raw link 変換: `rewrite.go:17`、`112`、`357`
- repair の本文限定: `repair.go:189`

## 5. 正本へのポインタ

- リンク解釈と frontmatter の互換性: `docs/specs/overview.md:471`
- ルート優先: `docs/decisions/0004-root-priority-for-ambiguous-basename.md`
- frontmatter raw path (`link_keys`): `docs/decisions/0014-frontmatter-link-keys.md`
- quoted-only frontmatter wikilink: `docs/decisions/0023-frontmatter-wikilink-quoted-only.md`
- DB schema: `docs/rules/03-data-model.md`
