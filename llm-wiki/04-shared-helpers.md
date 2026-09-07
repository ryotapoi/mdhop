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

`sources` のトップレベル関数のうち、別の本番 Go ファイルから呼ばれる関数を掲載する。呼び出し位置は `internal/core/` と `cmd/mdhop/` の本番コードから抽出する（同じファイル内の呼び出しも含む）。テストとコメントは対象外。型のメソッドは各定義元を参照する。

書き換えの適用境界は `applyFileRewritesWithRollbackFailures`。呼び出し元はエントリの配列を渡し、ファイル別の振り分け・書き込み順序・復元はこの関数が所有する。

## rewrite.go

| 関数・定義位置 | 責務 | 呼び出し位置 |
|---|---|---|
| `isPathLinkType` (`internal/core/rewrite.go:25`) | パスとして検証するリンク型を判定 | `internal/core/build.go:78` / `internal/core/disambiguate.go:308` / `internal/core/link_ambiguity.go:61` / `internal/core/move_rewrite.go:285` / `internal/core/simplify.go:69` |
| `buildRewritePath` (`internal/core/rewrite.go:65`) | 書き換え先の .md 拡張子を除去 | `internal/core/convert.go:149` / `internal/core/rewrite.go:79,102` |
| `rewriteRawLink` (`internal/core/rewrite.go:73`) | リンク構文を保って参照先を置換 | `internal/core/add.go:197` / `internal/core/disambiguate.go:151,333` / `internal/core/move_rewrite.go:164,170,175,320,356,422` / `internal/core/repair.go:121` / `internal/core/simplify.go:146` |
| `replaceOutsideInlineCode` (`internal/core/rewrite.go:114`) | インラインコード外の文字列を置換 | `internal/core/move_apply.go:25` / `internal/core/rewrite.go:268` |
| `writeFilePreservePerm` (`internal/core/rewrite.go:147`) | パーミッションを保って書き込み | `internal/core/move_apply.go:50` / `internal/core/set.go:91` |
| `restoreBackupFiles` (`internal/core/rewrite.go:154`) | バックアップを復元し復元失敗を収集 | `internal/core/add.go:290,296` / `internal/core/disambiguate.go:203,209` / `internal/core/move_apply.go:51` / `internal/core/move_dir.go:115,145,146` |
| `wrapRollbackFailures` (`internal/core/rewrite.go:173`) | 主エラーへ復元失敗と復旧案内を付加 | `internal/core/add.go:282,290,296` / `internal/core/disambiguate.go:197,203,209` / `internal/core/move_dir.go:109,116,147` / `internal/core/scan_rewrite.go:69` / `internal/core/set.go:97` |
| `applyFileRewritesWithRollbackFailures` (`internal/core/rewrite.go:190`) | エントリをファイル別に集めて一括書き換え、失敗時に復元 | `internal/core/add.go:280` / `internal/core/disambiguate.go:195` / `internal/core/move_dir.go:107` / `internal/core/scan_rewrite.go:68` |
| `isBasenameRawLink` (`internal/core/rewrite.go:293`) | リンク構文が basename 参照か判定 | `internal/core/add.go:180` / `internal/core/diagnose.go:87` / `internal/core/disambiguate.go:139` / `internal/core/move_rewrite.go:145,416` |

## util.go

| 関数・定義位置 | 責務 | 呼び出し位置 |
|---|---|---|
| `normalizeTextNFC` (`internal/core/util.go:11`) | 文字列を NFC 正規化 | `internal/core/build.go:240` / `internal/core/db.go:207,211` / `internal/core/diagnose.go:120` / `internal/core/link_ambiguity.go:14,33` / `internal/core/link_resolver.go:45` / `internal/core/meta_check.go:202` / `internal/core/query_entry.go:93` / `internal/core/resolve.go:180,247` / `internal/core/util.go:18,79,103` |
| `NormalizePath` (`internal/core/util.go:16`) | パス表記と Unicode を正規化 | `cmd/mdhop/delete.go:67,97` / `cmd/mdhop/move.go:103,104,133,134` / `internal/core/add.go:53` / `internal/core/build.go:271,349,404,421` / `internal/core/convert.go:42` / `internal/core/db.go:199,203` / `internal/core/delete.go:47` / `internal/core/disambiguate.go:79,237,282,379` / `internal/core/link_resolver.go:31,82` / `internal/core/meta_check.go:208,213` / `internal/core/move.go:60,61` / `internal/core/move_load.go:31,32,174` / `internal/core/move_rewrite.go:489,497` / `internal/core/move_template.go:88,149` / `internal/core/pathfilter.go:11,24,59,63,73` / `internal/core/query_entry.go:60` / `internal/core/reachable.go:60` / `internal/core/resolve.go:37,130,250` / `internal/core/resolve_maps.go:29,43,51,71,82,89,166,177,204,215` / `internal/core/set.go:36` / `internal/core/simplify.go:49` / `internal/core/update.go:46,252,273` / `internal/core/util.go:31,65,122,125,127` |
| `newVaultDiskPathResolver` (`internal/core/util.go:26`) | 正規化パスから実ディスクパスを引く resolver を初期化 | `internal/core/add.go:38` / `internal/core/build.go:48` / `internal/core/disambiguate.go:170` / `internal/core/init_meta_infer.go:120` / `internal/core/repair.go:50` / `internal/core/rewrite.go:155,199` / `internal/core/scan_rewrite.go:46` / `internal/core/set.go:61` / `internal/core/update.go:29` |
| `basename` (`internal/core/util.go:77`) | 拡張子を除いたファイル名を取得 | `internal/core/add.go:304,335` / `internal/core/build.go:152` / `internal/core/delete.go:54` / `internal/core/move_dir.go:204,272` / `internal/core/move_rewrite.go:215` / `internal/core/simplify.go:130` / `internal/core/update.go:56` / `internal/core/util.go:97` |
| `countLines` (`internal/core/util.go:85`) | 本文の行数を計算 | `internal/core/add.go:270` / `internal/core/build.go:102` / `internal/core/update.go:141` |
| `basenameKey` (`internal/core/util.go:96`) | note の basename 比較キーを生成 | `internal/core/add.go:110,145,319,326` / `internal/core/build.go:359` / `internal/core/disambiguate.go:50,271,319,325` / `internal/core/link_ambiguity.go:38` / `internal/core/move_rewrite.go:157,158,201,214,290,305` / `internal/core/resolve_maps.go:34,57,107,113,167` / `internal/core/simplify.go:183,222` |
| `assetBasenameKey` (`internal/core/util.go:102`) | 拡張子を含む asset の比較キーを生成 | `internal/core/build.go:367` / `internal/core/link_ambiguity.go:45` / `internal/core/move_rewrite.go:151,152,231,244` / `internal/core/resolve_maps.go:73,92,124,205` / `internal/core/simplify.go:111,203,233` |
| `isRootFile` (`internal/core/util.go:107`) | ルート直下のファイルか判定 | `internal/core/add.go:135,145,320` / `internal/core/resolve.go:265` / `internal/core/resolve_maps.go:36,62,75,97,171,209` / `internal/core/util.go:114` |
| `hasRootInPathSet` (`internal/core/util.go:112`) | ルート優先候補の存在を判定 | `internal/core/link_ambiguity.go:17,24` / `internal/core/move_rewrite.go:167,168,207,208,237,238` |
| `resolveToVaultRelative` (`internal/core/util.go:119`) | リンク先を vault 相対パスへ変換 | `internal/core/repair.go:173` / `internal/core/simplify.go:85` |
| `isFieldActive` (`internal/core/util.go:131`) | 指定された出力フィールドが有効か判定 | `internal/core/diagnose.go:283,284,309,317` / `internal/core/query.go:137,145,155,165,173,183,191` / `internal/core/stats.go:39,45,51,57,63,69` |

## resolve_maps.go

| 関数・定義位置 | 責務 | 呼び出し位置 |
|---|---|---|
| `newResolveMaps` (`internal/core/resolve_maps.go:143`) | note と asset の解決マップを初期化 | `internal/core/build.go:47` / `internal/core/meta_check.go:88` |
| `buildNoteResolveMaps` (`internal/core/resolve_maps.go:161`) | note 用の解決マップを構築 | `internal/core/resolve_maps.go:144` / `internal/core/simplify.go:41` |
| `buildAssetResolveMaps` (`internal/core/resolve_maps.go:199`) | asset 用の解決マップを構築 | `internal/core/resolve_maps.go:145` / `internal/core/simplify.go:42` |

## move_load.go

| 関数・定義位置 | 責務 | 呼び出し位置 |
|---|---|---|
| `validateMoveDirOptions` (`internal/core/move_load.go:30`) | ディレクトリ移動の引数を検証 | `internal/core/move_dir.go:44` |
| `loadSingleMoveFromDB` (`internal/core/move_load.go:56`) | 単一ファイルの移動情報を DB から取得 | `internal/core/move.go:38` |
| `loadMovesFromDB` (`internal/core/move_load.go:82`) | ディレクトリ配下の移動情報を DB から取得 | `internal/core/move_dir.go:48` |
| `checkDestinationsFree` (`internal/core/move_load.go:124`) | 移動先の登録状態を検証 | `internal/core/move.go:42` / `internal/core/move_dir.go:52` / `internal/core/move_template.go:74` |
| `collectDiskOnlyFiles` (`internal/core/move_load.go:147`) | 未登録の移動対象ファイルを収集 | `internal/core/move_dir.go:55` |
| `classifyDiskState` (`internal/core/move_load.go:192`) | 通常移動か移動済みかを判定 | `internal/core/move.go:45` / `internal/core/move_dir.go:59` / `internal/core/move_template.go:77` |
| `checkMovedFilesNotStale` (`internal/core/move_load.go:216`) | 移動対象の mtime を検証 | `internal/core/move.go:49` / `internal/core/move_dir.go:63` / `internal/core/move_template.go:81` |

## move_rewrite.go

| 関数・定義位置 | 責務 | 呼び出し位置 |
|---|---|---|
| `adjustMapsForDirMove` (`internal/core/move_rewrite.go:38`) | 移動後の解決マップを構築 | `internal/core/move_dir.go:72` |
| `collectIncomingRewritesForDir` (`internal/core/move_rewrite.go:90`) | 移動対象への被リンクの書き換えを収集 | `internal/core/move_dir.go:84` |
| `collectCollateralRewritesForDir` (`internal/core/move_rewrite.go:192`) | 移動に伴い解決先が変わる第三者リンクを収集 | `internal/core/move_dir.go:88` |
| `buildMovedFileRewrites` (`internal/core/move_rewrite.go:260`) | 移動ファイル自身のリンク書き換えを構築 | `internal/core/move_dir.go:98` |

## move_apply.go

| 関数・定義位置 | 責務 | 呼び出し位置 |
|---|---|---|
| `prepareMovedFileRewrites` (`internal/core/move_apply.go:13`) | 移動ノートの候補を副作用前に検証し、DB 再解析にも使う内容を準備 | `internal/core/move_dir.go:109` |
| `applyMovedFileRewrites` (`internal/core/move_apply.go:34`) | 準備済みの移動ノート内容を書き換え、復元情報を返す | `internal/core/move_dir.go:122` |
| `updateExternalEdgesAndMtimes` (`internal/core/move_apply.go:58`) | 外部リンクの edge と mtime を DB に反映 | `internal/core/add.go:372` / `internal/core/disambiguate.go:214` / `internal/core/move_dir.go:250` |
| `promotePhantom` (`internal/core/move_apply.go:94`) | phantom を実ノードへ昇格し edge を付け替え | `internal/core/add.go:335` / `internal/core/move_dir.go:283` |
