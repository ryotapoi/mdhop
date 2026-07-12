---
regen: full
sources:
  - internal/core/rewrite.go
  - internal/core/util.go
  - internal/core/resolve_maps.go
  - internal/core/move_load.go
  - internal/core/move_rewrite.go
  - internal/core/move_apply.go
---

# 共有ヘルパー呼び出しサイト対応表

`internal/core/` 内で定義され、複数のコマンド実装から参照される共有ヘルパーの一覧。
実装内容の再掲は行わない。定義位置・1行役割・主な呼び出しサイトのみ記載。

> `sources` フロントマターはヘルパーの**定義元ファイルのみ**を挙げる（再生成の起点）。
> 本文の「呼び出しサイト」列は、定義元の各ヘルパー名を `internal/core/` と `cmd/mdhop/` で grep して再生成する従属情報であり、呼び出し側ファイルは `sources` に含めない。

---

## rewrite.go 定義ヘルパー

### rewriteRawLink

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/rewrite.go:56` |
| 役割 | rawLink を新ターゲットパスに書き換えた文字列を返す（wikilink / markdown リンク形式保持） |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/add.go` | 189 | basename エッジの rawLink 更新 |
| `internal/core/move_rewrite.go` | 163, 169, 174, 319, 355, 419 | move / dir-move 向け incoming / collateral / moved-file リンク書き換え |
| `internal/core/disambiguate.go` | 150, 374 | 曖昧リンク解消 |
| `internal/core/simplify.go` | 170 | path/relative リンク → basename リンクへ短縮 |
| `internal/core/repair.go` | 142 | 壊れたリンクの修復 |

---

### applyFileRewrites

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/rewrite.go:159` |
| 役割 | rewriteEntry グループをファイルに一括適用し、mtime マップとバックアップを返す |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/add.go` | 268 | ファイル追加時の外部リンク書き換え |
| `internal/core/disambiguate.go` | 193, 399 | 曖昧解消の書き換え適用（2 フェーズ） |
| `internal/core/simplify.go` | 203 | simplify 書き換え適用 |
| `internal/core/repair.go` | 175 | repair 書き換え適用 |
| `internal/core/convert.go` | 148 | リンク形式変換 |
| `internal/core/move_apply.go` | 21 | dir-move の外部リンク書き換え（`groupAndApplyExternalRewrites` 経由） |

---

### replaceOutsideInlineCode

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/rewrite.go:109` |
| 役割 | インラインコード外のみを対象に行内の old → new を置換する |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/rewrite.go` | 206 | `applyFileRewrites` 内の行ごと置換 |
| `internal/core/move_apply.go` | 38 | `applyOutgoingRewritesToContent` 内の行ごと置換 |

---

### writeFilePreservePerm

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/rewrite.go:142` |
| 役割 | ファイルパーミッションを保持したまま内容を上書き書き込みする |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/rewrite.go` | 157, 204, 231 | バックアップ復元・書き換え適用 |
| `internal/core/move_apply.go` | 63 | move / dir-move の移動ファイル書き込み |

---

### isBasenameRawLink

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/rewrite.go:231` |
| 役割 | rawLink が basename 形式（パス区切りなし）かどうかを判定する |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/add.go` | 172 | basename エッジ分類 |
| `internal/core/move_rewrite.go` | 144, 413 | move / dir-move の basename 判定 |
| `internal/core/disambiguate.go` | 138 | 曖昧解消対象フィルタ |
| `internal/core/diagnose.go` | 77 | 診断時の basename リンク識別 |

---

### restoreBackups

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/rewrite.go:150` |
| 役割 | エラー時にバックアップから元ファイルを復元する |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/add.go` | 293, 301 | add 処理の書き換えロールバック |
| `internal/core/disambiguate.go` | 206, 213 | 曖昧解消のロールバック |
| `internal/core/move_apply.go` | 64 | move / dir-move の移動ファイル書き換え失敗時ロールバック |
| `internal/core/move_dir.go` | 113, 136, 137 | move / dir-move 処理のロールバック |

---

## util.go 定義ヘルパー

### basenameKey

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/util.go:42` |
| 役割 | パスから拡張子なし小文字のバsename キーを返す（basename 重複チェックに使用） |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/build.go` | 375 | 重複 basename の検出 |
| `internal/core/resolve_maps.go` | 34, 52, 96, 102, 156 | resolveMaps への追加・削除・再構築 |
| `internal/core/add.go` | 103, 138, 307, 314 | 追加ノードの basename 解決 |
| `internal/core/move_rewrite.go` | 156, 157, 200, 213, 289, 304 | move / dir-move の basename 管理 |
| `internal/core/disambiguate.go` | 51, 303, 360, 366 | 曖昧候補のフィルタ |
| `internal/core/simplify.go` | 215, 254 | simplify 時の basename 一致確認 |
| `internal/core/update.go` | 249, 288 | 更新時の basename 管理 |
| `internal/core/util.go` | 92 | `ambiguousCandidates` 内の一致確認 |

---

### isFieldActive

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/util.go:213` |
| 役割 | フィールド名が有効なフィールドリスト（空＝全有効）に含まれるか判定する |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/query.go` | 119, 127, 137, 147, 155, 165, 173 | query の各フィールド出力制御 |
| `internal/core/stats.go` | 28, 34, 40, 46, 52, 58 | stats の各フィールド出力制御 |
| `internal/core/diagnose.go` | 273, 274, 299, 307 | diagnose の各フィールド出力制御 |

---

### isAmbiguousBasenameLink / ambiguousCandidates

| 項目 | 値 |
|------|-----|
| `isAmbiguousBasenameLink` 定義 | `internal/core/util.go:67` |
| `ambiguousCandidates` 定義 | `internal/core/util.go:86` |
| 役割 | basename リンクが複数候補に解決される（曖昧）かを判定し、候補パスリストを返す |

主な呼び出しサイト:

| ファイル | 行 | 呼び出し関数 |
|----------|----|-------------|
| `internal/core/build.go` | 81, 82 | `isAmbiguousBasenameLink` / `ambiguousCandidates` |
| `internal/core/util.go` | 124, 125 | `validateParsedLinks` 内 |
| `internal/core/meta_check.go` | 169 | `isAmbiguousBasenameLink` |
| `internal/core/frontmatter_path_guard.go` | 61, 62 | `isAmbiguousBasenameLink` / `ambiguousCandidates` |

---

### CleanupEmptyDirs

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/util.go:135` |
| 役割 | 対象パスリストに含まれるディレクトリが空になったら削除する |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `cmd/mdhop/delete.go` | 105 | delete コマンドの後処理 |

---

### resolveToVaultRelative

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/util.go:201` |
| 役割 | 相対リンクをソースパス基準で vault 相対パスに変換する |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/simplify.go` | 109 | simplify の相対リンク解決 |
| `internal/core/repair.go` | 202 | repair の相対リンク解決 |

---

## resolve_maps.go 定義ヘルパー

### newResolveMaps

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/resolve_maps.go:132` |
| 役割 | ノードファイル・アセットファイルリストから `resolveMaps` 構造体を初期化する |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/build.go` | 47 | build 時の初期マップ構築 |
| `internal/core/meta_check.go` | 82 | meta-check の解決マップ初期化 |

---

### buildNoteResolveMaps / buildAssetResolveMaps

| 項目 | 値 |
|------|-----|
| `buildNoteResolveMaps` 定義 | `internal/core/resolve_maps.go:150` |
| `buildAssetResolveMaps` 定義 | `internal/core/resolve_maps.go:188` |
| 役割 | ノード / アセット専用の解決マップを個別に構築する |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/resolve_maps.go` | 133, 134 | `newResolveMaps` 内で呼び出し |
| `internal/core/simplify.go` | 50, 51 | simplify の独立マップ初期化 |

---

## move_* 定義ヘルパー（他コマンドから呼び出されるもの）

### groupAndApplyExternalRewrites

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/move_apply.go:13` |
| 役割 | 外部リンク書き換えエントリをグループ化して `applyFileRewrites` を呼び出す薄いラッパー |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/move_dir.go` | 105 | move / dir-move の外部リンク書き換え（`executeMoves`） |

---

### applyOutgoingRewritesToContent

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/move_apply.go:26` |
| 役割 | 移動ファイル本体の outgoing リンクを `replaceOutsideInlineCode` で行ごとに書き換える |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/move_apply.go` | 59 | `applyMovedFileRewrites` 内の移動ファイル内容書き換え |

---

### applyMovedFileRewrites

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/move_apply.go:47` |
| 役割 | move / dir-move の移動ファイル本体に outgoing rewrite を適用し、`rewriteBackup` を返す |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/move_dir.go` | 111 | move / dir-move の移動ファイル本体書き換え（`executeMoves`） |

---

### updateExternalEdgesAndMtimes

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/move_apply.go:74` |
| 役割 | 外部エッジの raw_link と mtime を DB に反映し、書き換えリストを返す |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/move_dir.go` | 231 | move / dir-move の DB 更新（`executeMoves`） |

---

### promotePhantom

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/move_apply.go:110` |
| 役割 | ファントムノードを実ノードに昇格させ、エッジを付け替える |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/add.go` | 340 | add 時のファントム昇格 |
| `internal/core/move_dir.go` | 264 | move / dir-move 時のファントム昇格（`executeMoves`） |

---

### rewriteOutgoingRelativeLink

| 項目 | 値 |
|------|-----|
| 定義 | `internal/core/move_rewrite.go:490` |
| 役割 | 相対パスリンクを移動後の新パスに基づき再計算する |

主な呼び出しサイト:

| ファイル | 行 | 用途 |
|----------|----|------|
| `internal/core/move_rewrite.go` | 330 | `buildMovedFileRewrites` 内の move / dir-move 処理 |
