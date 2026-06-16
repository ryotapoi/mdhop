---
regen: compiled
sources:
  - internal/core/parse.go
  - internal/core/parse_frontmatter.go
  - internal/core/resolve.go
  - internal/core/resolve_maps.go
  - internal/core/rewrite.go
  - internal/core/build.go
  - internal/core/disambiguate.go
  - internal/core/move.go
  - internal/core/simplify.go
  - internal/core/repair.go
  - internal/core/util.go
  - internal/core/errors.go
  - internal/core/db.go
  - docs/specs/overview.md
  - docs/decisions/0004-root-priority-for-ambiguous-basename.md
---

# リンク解決・リライトの編纂ガイド

rawLink が入力されてから解決・リライトされるまでの流れを、関数連携として示す。
仕様の正本 → `docs/specs/overview.md`（resolve のルール節）。設計判断 → 各 ADR。

---

## 1. parse（入力 → linkOccur）

| ファイル | 関数 | 役割 |
|---|---|---|
| `parse.go:25` | `parseLinks(content)` | フロントマター + ボディを全量パース |
| `parse.go:81` | `parseLinksWithLinkKeys(content, linkKeys)` | `parseLinks` + `link_keys` を frontmatter_path に変換 |
| `parse_frontmatter.go:287` | `frontmatterPathLinks(meta, linkKeys)` | link_keys の raw 値を `frontmatter_path` 型の linkOccur に変換 |
| `parse_frontmatter.go:307` | `frontmatterPathOccur(value, line)` | 単一 link-key 値を linkOccur に分類（空・URL・`[[` はスキップ） |
| `parse.go:57` | `walkBodyLines(lines, fmEnd, fn)` | フロントマター・コードフェンスをスキップするボディ走査スケルトン |

**linkOccur 型** (`parse.go:8`): `target, isBasename, isRelative, linkType, rawLink, subpath, lineStart, lineEnd`

**LinkType 定数** (`db.go:45`):

| 値 | 意味 |
|---|---|
| `wikilink` | `[[Target]]` |
| `markdown` | `[text](url)` |
| `tag` | `#tag` |
| `frontmatter` | フロントマターの tags/tag 値 |
| `frontmatter_wikilink` | フロントマター内の `[[...]]` |
| `frontmatter_path` | link_keys に指定されたキーの raw 値 |

**isBasename の判定** (`parse.go:347`): `./` `../` `/` で始まらず `/` を含まなければ basename リンク。

**edge 作成に使う側** は `parseLinksWithLinkKeys`（build/update/add/move）。
`parseLinks` はエッジを作らない read-only パスで使う。

---

## 2. resolve（linkOccur → target node ID）

### 2A. build 時（インメモリマップ使用）

`build.go:207` `resolveLink(db, sourcePath, link, rm)` — メインディスパッチャ。

| 条件 | 処理 | 関数 |
|---|---|---|
| `target == ""` かつ subpath あり | 自己リンク `[[#Heading]]` → sourceID を返す | `build.go:209` |
| tag / frontmatter 型 | upsertTag | `build.go:215` |
| `link.isRelative` | `filepath.Join(dir, target)` → `resolvePathTarget` | `build.go:226` |
| vault 外逃げ（相対） | `ErrLinkEscapesVault` | `build.go:228` |
| vault 外逃げ（絶対） | `ErrLinkEscapesVault` | `build.go:235` |
| `/` 始まり | 先頭 `/` を除去 → `resolvePathTarget` | `build.go:240` |
| wikilink かつ `/` 含む（パス形式） | `resolvePathTarget` | `build.go:246` |
| `link.isBasename` | note unique → note root → asset unique → asset root → phantom | `build.go:251` |
| それ以外（markdown パス） | `resolvePathTarget` | `build.go:282` |

`resolvePathTarget(db, resolved, link, rm)` (`build.go:286`): pathSet → pathSet+.md → assetPathSet → phantom の順。

**インメモリマップの構造** (`resolve_maps.go:9` `resolveMaps`):

- `pathSet`: lower path → actual path（note。拡張子なし版も登録）
- `basenameToPath`: lower basename → path（count==1 のみ）
- `rootBasenameToPath`: lower basename → root path（ADR 0004 用）
- asset 系も同構造（`assetPathSet` / `assetBasenameToPath` / `assetRootBasenameToPath`）

マップの初期化: `resolve_maps.go:132` `newResolveMaps(files, assetFiles)`。
インクリメンタル更新: `addNote/removeNote/addAsset/removeAsset` (`resolve_maps.go:28〜88`)。

### 2B. resolve コマンド時（DB クエリ使用）

`resolve.go:21` `Resolve(vaultPath, fromPath, link)` — 公開 API。
内部ディスパッチャ: `resolve.go:83` `resolveLinkFromDB(db, sourcePath, link)` — `resolveLink` と対応する DB 版。

| 条件 | 処理 | 関数 |
|---|---|---|
| 自己リンク | `getNodeID(db, noteKey(sourcePath))` | `resolve.go:85` |
| tag / frontmatter 型 | `getNodeID(db, tagKey)` | `resolve.go:94` |
| 相対パス | `NormalizePath(Join(dir, target))` → `resolvePathFromDB` | `resolve.go:109` |
| vault 外逃げ | `ErrLinkEscapesVault` | `resolve.go:110` / `resolve.go:118` |
| `/` 始まり | 先頭 `/` 除去 → `resolvePathFromDB` | `resolve.go:124` |
| wikilink パス形式 | `resolvePathFromDB` | `resolve.go:129` |
| basename | `resolveBasenameFromDB` | `resolve.go:134` |

`resolvePathFromDB` (`resolve.go:144`): note exact → note+.md → asset → phantom の順（DB クエリ）。

`resolveBasenameFromDB` (`resolve.go:194`): `queryBasenameMatches` で全 note を走査 → `pickBasenameMatch` でルート優先適用。

### 2C. ルート優先ルール（ADR 0004）

`pickBasenameMatch(matches)` (`resolve.go:275`):

1. `len(matches) == 1` → そのまま返す
2. 複数ある場合: `isRootFile(path)` が true のものを返す
3. ルート候補もなければ `(_, false)` → `ErrAmbiguousLink`

`isRootFile(path)` (`util.go:53`): path が `/` を含まない（= ルート直下）なら true。

**build 時のアンビギュイティ拒否タイミング**: `build.go:81` の `isAmbiguousBasenameLink`。
条件は basename カウント > 1 かつルート直下にファイルがない（`util.go:67`）。
エラーは `ErrAmbiguousLink` (`errors.go:16`)、最大 `maxBuildErrors(5)` 件まで収集。

**validate 関数** (`util.go:113` `validateParsedLinks`): add/update/move/move_dir が edge 作成前に呼ぶ。build 側の inline 判定と同ロジック。

---

## 3. rewrite（rawLink → 書き換え済み rawLink → ファイル書き込み）

### 3A. 関数の役割

| 関数 | ファイル:行 | 役割 |
|---|---|---|
| `rewriteRawLink(rawLink, linkType, targetPath)` | `rewrite.go:56` | rawLink の target 部分を新パスに置換して返す |
| `buildRewritePath(targetPath)` | `rewrite.go:48` | `.md` 末尾だけ除去（他の拡張子はそのまま） |
| `replaceOutsideInlineCode(line, old, new)` | `rewrite.go:109` | インラインコードスパン外のみ置換 |
| `applyFileRewrites(vaultPath, groups)` | `rewrite.go:159` | 全ファイルへの書き込み（フェーズ1: 読む、フェーズ2: 書く、失敗時ロールバック） |
| `isBasenameRawLink(rawLink, linkType)` | `rewrite.go:231` | rawLink が basename 形式かを判定 |

**wikilink のリライト規則** (`rewrite.go:58`): 常に .md なし（`buildRewritePath` が除去）。
**markdown のリライト規則** (`rewrite.go:77`): 元の URL に `.md` があれば `newPath + ".md"` を維持。

### 3B. `frontmatter_path` はリライト対象外

`pathLinkTypeSQLList` (`rewrite.go:15`): リライト対象の path 系 LinkType を並べた SQL リスト（値は正本を読む）。
`frontmatter_path` はリストに含まない（raw 値はリンク構文ではないため書き換えできない）。

`isPathLinkType` (`rewrite.go:20`): バリデーション側（逃げ・曖昧チェック）では `frontmatter_path` を含む。

### 3C. 呼び出し元

| 呼び出し元 | 目的 | 主な参照箇所 |
|---|---|---|
| `move.go` | move 後の被リンクを書き換え | `move.go:192` `isBasenameRawLink` / `move.go:196` `rewriteRawLink` |
| `disambiguate.go` | basename → フルパスに書き換え | `disambiguate.go:115` / `disambiguate.go:150` |
| `disambiguate.go` | scan モード（DB なし） | `disambiguate.go:355` / `disambiguate.go:374` |
| `simplify.go` | 解決可能な path/relative リンクを basename リンクへ短縮 | `simplify.go:170` / `simplify.go:203` |
| `repair.go` | broken / vault-escape の path リンクを basename リンクへ修正 | `repair.go:142` / `repair.go:175` |

---

## 4. 変更時の読むべき場所

### パース処理を変えるとき

- `parse.go:25` `parseLinks` / `parse.go:57` `walkBodyLines`
- `parse_frontmatter.go:307` `frontmatterPathOccur`（link_keys の分類ロジック）
- 対応テスト: `parse_test.go`, `resolve_maps_test.go`

### 解決ロジックを変えるとき

- build 時（インメモリ）: `build.go:207` `resolveLink` + `build.go:286` `resolvePathTarget`
- resolve コマンド時（DB）: `resolve.go:83` `resolveLinkFromDB` + `resolve.go:144` `resolvePathFromDB`
- 二つは **ロジックを平行に保つ必要がある**（`resolve.go:83` のコメント参照）
- ルート優先: `resolve.go:275` `pickBasenameMatch` / `util.go:67` `isAmbiguousBasenameLink`
- ADR 0004: `docs/decisions/0004-root-priority-for-ambiguous-basename.md`

### リライト処理を変えるとき

- `rewrite.go:56` `rewriteRawLink`（wikilink / markdown の対称性に注意）
- `rewrite.go:159` `applyFileRewrites`（フェーズ分割・ロールバック）
- `rewrite.go:231` `isBasenameRawLink`（move.go が判定ロジックに使用）
- 対応テスト: `rewrite_test.go`, `move_test.go`, `disambiguate_test.go`

### マップを変えるとき

- `resolve_maps.go:9` `resolveMaps` 構造体（フィールド追加はインクリメンタル更新メソッドにも反映）
- `resolve_maps.go:28` `addNote` / `resolve_maps.go:45` `removeNote`（asymmetry: addNote は pathToID を更新しない）
- `resolve_maps.go:93` `rebuildBasenameToPath`（Add 時の extraPaths 引数）

---

## 5. エラーの種類と発生場所

| エラー | 定義 | 発生 |
|---|---|---|
| `ErrAmbiguousLink` | `errors.go:16` | build バリデーション `build.go:81` / `util.go:124` / `resolve.go:206` |
| `ErrLinkEscapesVault` | `errors.go:19` | build `build.go:228` `build.go:235` / resolve `resolve.go:110` `resolve.go:118` / `util.go:118` |
| `ErrLinkNotFound` | `errors.go:18` | resolve コマンド `resolve.go:63` / `resolvePathFromDB` `resolve.go:188` / `resolveBasenameFromDB` `resolve.go:232` |

---

## 6. 正本へのポインタ

- リンク解決のルール全文: `docs/specs/overview.md` の「resolve のルール（要点）」節
- ルート優先の設計判断: `docs/decisions/0004-root-priority-for-ambiguous-basename.md`
- DB スキーマ（edges.link_type 等）: `docs/rules/03-data-model.md`
- link_keys 機能の判断: `docs/decisions/0014-frontmatter-link-keys.md`
- NFC 正規化: `docs/decisions/0020-nfc-path-normalization.md`
