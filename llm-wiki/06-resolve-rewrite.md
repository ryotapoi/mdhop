---
regen: compiled
sources:
  - internal/core/parse.go
  - internal/core/parse_frontmatter.go
  - internal/core/link_resolver.go
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

**LinkType 定数** (`db.go:44`):

| 値 | 意味 |
|---|---|
| `wikilink` | `[[Target]]` |
| `markdown` | `[text](url)` |
| `tag` | `#tag` |
| `frontmatter` | フロントマターの tags/tag 値 |
| `frontmatter_wikilink` | フロントマター内の `[[...]]` |
| `frontmatter_path` | link_keys に指定されたキーの raw 値 |

**isBasename の判定** (`parse.go:369`): `./` `../` `/` で始まらず `/` を含まなければ basename リンク。

**edge 作成に使う側** は `parseLinksWithLinkKeys`（build/update/add/move）。
`parseLinks` はエッジを作らない read-only パスで使う。

---

## 2. resolve（linkOccur → target node ID）

### 2A. 共通ディスパッチ

`link_resolver.go` `resolveLinkWithBackend(sourcePath, link, backend)` — build 時と resolve コマンド時で共有するメインディスパッチャ。

| 条件 | 処理 | 関数 |
|---|---|---|
| `target == ""` かつ subpath あり | 自己リンク `[[#Heading]]` | backend `resolveSelf` |
| tag / frontmatter 型 | tag 解決 | backend `resolveTag` |
| `link.isRelative` | vault escape 検査 → `filepath.Join(dir, target)` | backend `resolvePath` |
| vault 外逃げ（相対 / 絶対） | `ErrLinkEscapesVault` | `resolveLinkWithBackend` |
| `/` 始まり | 先頭 `/` を除去 | backend `resolvePath` |
| wikilink かつ `/` 含む（パス形式） | vault 相対パス | backend `resolvePath` |
| `link.isBasename` | basename 解決 | backend `resolveBasename` |
| それ以外（markdown パス） | path 解決 | backend `resolvePath` |

### 2B. build 時（インメモリマップ使用）

`build.go` `resolveLink(db, sourcePath, link, rm)` — `mapLinkResolver` を使って共通ディスパッチへ渡す。

`mapLinkResolver.resolveBasename`: note unique → note root → asset unique → asset root → phantom の順。build / add / update / move では edge 作成前に `validateParsedLinks` で曖昧 basename を拒否するため、ここでは未解決 basename が phantom fallback できる。

`resolvePathTarget(db, resolved, link, rm)`: pathSet → pathSet+.md → assetPathSet → phantom の順。

**インメモリマップの構造** (`resolve_maps.go:9` `resolveMaps`):

- `pathSet`: lower path → actual path（note。拡張子なし版も登録）
- `basenameToPath`: lower basename → path（count==1 のみ）
- `rootBasenameToPath`: lower basename → root path（ADR 0004 用）
- asset 系も同構造（`assetPathSet` / `assetBasenameToPath` / `assetRootBasenameToPath`）

マップの初期化: `resolve_maps.go:143` `newResolveMaps(files, assetFiles)`。
インクリメンタル更新: `addNote/removeNote/addAsset/removeAsset` (`resolve_maps.go:28〜100`)。

### 2C. resolve コマンド時（DB クエリ使用）

`resolve.go:30` `Resolve(vaultPath, fromPath, link)` — 公開 API。
内部解決: `resolve.go` `resolveLinkFromDB(db, sourcePath, link)` — `dbLinkResolver` を使って共通ディスパッチへ渡す。

`resolvePathFromDB`: note exact → note+.md → asset → phantom の順（DB クエリ）。

`resolveBasenameFromDB`: `queryBasenameMatches` で全 note を走査 → `pickBasenameMatch` でルート優先適用。複数候補かつ root-priority で一意化できない場合は `ErrAmbiguousLink` を返す。

### 2D. ルート優先ルール（ADR 0004）

`pickBasenameMatch(matches)` (`resolve.go:260`):

1. `len(matches) == 1` → そのまま返す
2. 複数ある場合: `isRootFile(path)` が true のものを返す
3. ルート候補もなければ `(_, false)` → `ErrAmbiguousLink`

`isRootFile(path)` (`util.go:107`): path が `/` を含まない（= ルート直下）なら true。

**pathSet キー構造とルート判定の仕組み**: `addNote` (`resolve_maps.go:28`) は `A.md` を `"a"`（拡張子なし）と `"a.md"` の 2 キーで登録する。ルート直下の `A.md` は `"a"` キー、`sub/A.md` は `"sub/a"` キーになるので、`pathSet["a"]` が存在すれば必ずルートファイルを指す。`hasRootInPathSet(bk, pathSet)` (`util.go:112`) はこの性質を使い、`pathSet[bk]` の存在チェックだけでルート候補の有無を判定できる。

**build 時のアンビギュイティ拒否タイミング**: `build.go:81` の `isAmbiguousBasenameLink`。
条件は basename カウント > 1 かつルート直下にファイルがない（`link_ambiguity.go`）。
エラーは `ErrAmbiguousLink` (`errors.go:16`)、最大 `maxBuildErrors(5)` 件まで収集。

**validate 関数** (`link_ambiguity.go` `validateParsedLinks`): add/update/move/move_dir が edge 作成前に呼ぶ。build 側の inline 判定と同ロジック。

---

## 3. rewrite（rawLink → 書き換え済み rawLink → ファイル書き込み）

### 3A. 関数の役割

| 関数 | ファイル:行 | 役割 |
|---|---|---|
| `rewriteRawLink(rawLink, linkType, targetPath)` | `rewrite.go:73` | rawLink の target 部分を新パスに置換して返す |
| `buildRewritePath(targetPath)` | `rewrite.go:65` | `.md` 末尾だけ除去（他の拡張子はそのまま） |
| `replaceOutsideInlineCode(line, old, new)` | `rewrite.go:114` | インラインコードスパン外のみ置換 |
| `applyFileRewritesWithRollbackFailures(vaultPath, groups)` | `rewrite.go:190` | 全ファイルへの書き込み（フェーズ1: 読む、フェーズ2: 書く、失敗時ロールバック） |
| `isBasenameRawLink(rawLink, linkType)` | `rewrite.go:286` | rawLink が basename 形式かを判定 |
| `rewriteOutgoingRelativeLink(rawLink, linkType, from, to, movedFromTo)` | `move_rewrite.go:484` | 相対リンク（`./` `../`）を移動後の新パスへ `filepath.Rel` で再計算 |

**絶対 vs 相対の使い分け**: `buildRewritePath` は vault-relative の target パスを受け取りそのままリンク構文へ埋める（`.md` 除去後）。移動先への絶対（vault-relative）リライトに使う。一方、相対リンク（`./` `../` prefix）は `rewriteOutgoingRelativeLink` (`move_rewrite.go:484`) が `filepath.Rel(filepath.Dir(to), resolvedTarget)` で新パスを再計算するため `buildRewritePath` を呼ばない。

**wikilink のリライト規則** (`rewrite.go:75`): 常に .md なし（`buildRewritePath` が除去）。
**markdown のリライト規則** (`rewrite.go:82`): 元の URL に `.md` があれば `newPath + ".md"` を維持。

### 3B. `frontmatter_path` はリライト対象外

`rewriteLinkTypes` (`rewrite.go:16`): リライト対象の path 系 LinkType を並べる型付き集合。各クエリのバインド済み IN 句は `linkTypeSQLIn()` (`db.go:68`) が生成する。
`frontmatter_path` は集合に含まない（raw 値はリンク構文ではないため書き換えできない）。

`isPathLinkType` (`rewrite.go:25`): バリデーション側（逃げ・曖昧チェック）では `frontmatter_path` を含む。

### 3C. 呼び出し元

| 呼び出し元 | 目的 | 主な参照箇所 |
|---|---|---|
| `move_rewrite.go` | move 後の被リンクを書き換え | `move_rewrite.go:145` `isBasenameRawLink` / `move_rewrite.go:164` `rewriteRawLink` |
| `disambiguate.go` | basename → フルパスに書き換え | `disambiguate.go:139` / `disambiguate.go:151` |
| `disambiguate.go` | scan モード（DB なし） | `disambiguate.go:262` / `disambiguate.go:337` |
| `simplify.go` | 解決可能な path/relative リンクを basename リンクへ短縮 | `simplify.go:146` |
| `repair.go` | broken / vault-escape の path リンクを basename リンクへ修正 | `repair.go:159` / `repair.go:172` |

---

## 4. 変更時の読むべき場所

### パース処理を変えるとき

- `parse.go:25` `parseLinks` / `parse.go:57` `walkBodyLines`
- `parse_frontmatter.go:307` `frontmatterPathOccur`（link_keys の分類ロジック）
- 対応テスト: `parse_test.go`, `resolve_maps_test.go`

### 解決ロジックを変えるとき

- 共通ディスパッチ: `link_resolver.go` `resolveLinkWithBackend`
- build 時（インメモリ）: `build.go` `resolveLink` + `mapLinkResolver` + `resolvePathTarget`
- resolve コマンド時（DB）: `resolve.go` `resolveLinkFromDB` + `dbLinkResolver` + `resolvePathFromDB`
- ルート優先: `resolve.go` `pickBasenameMatch` / `link_ambiguity.go` `isAmbiguousBasenameLink`
- ADR 0004: `docs/decisions/0004-root-priority-for-ambiguous-basename.md`

### リライト処理を変えるとき

- `rewrite.go:73` `rewriteRawLink`（wikilink / markdown の対称性に注意）
- `rewrite.go:190` `applyFileRewritesWithRollbackFailures`（フェーズ分割・ロールバック）
- `rewrite.go:286` `isBasenameRawLink`（move.go が判定ロジックに使用）
- 対応テスト: `rewrite_test.go`, `move_test.go`, `disambiguate_test.go`

### マップを変えるとき

- `resolve_maps.go:9` `resolveMaps` 構造体（フィールド追加はインクリメンタル更新メソッドにも反映）
- `resolve_maps.go:28` `addNote` / `resolve_maps.go:50` `removeNote`（asymmetry: addNote は pathToID を更新しない）
- `resolve_maps.go:104` `rebuildBasenameToPath`（Add 時の extraPaths 引数）

---

## 5. エラーの種類と発生場所

| エラー | 定義 | 発生 |
|---|---|---|
| `ErrAmbiguousLink` | `errors.go:17` | build バリデーション `build.go:85` / `link_ambiguity.go:72` / `resolve.go:191` |
| `ErrLinkEscapesVault` | `errors.go:21` | build `build.go:81` `build.go:83` / resolve `link_resolver.go:111` `link_resolver.go:119` |
| `ErrLinkNotFound` | `errors.go:19` | resolve コマンド `resolve.go:72` / `resolvePathFromDB` `resolve.go:173` / `resolveBasenameFromDB` `resolve.go:217` |
| `ErrEntryNotFound` | `errors.go:20` | query の tag / phantom / name entry lookup `query_entry.go:80` / `query_entry.go:84` / `query_entry.go:126` |

---

## 6. 正本へのポインタ

- リンク解決のルール全文: `docs/specs/overview.md` の「resolve のルール（要点）」節
- ルート優先の設計判断: `docs/decisions/0004-root-priority-for-ambiguous-basename.md`
- DB スキーマ（edges.link_type 等）: `docs/rules/03-data-model.md`
- link_keys 機能の判断: `docs/decisions/0014-frontmatter-link-keys.md`
- NFC 正規化: `docs/decisions/0020-nfc-path-normalization.md`
