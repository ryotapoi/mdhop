---
regen: full
sources:
  - internal/core/rewrite.go
  - internal/core/util.go
  - internal/core/resolve_maps.go
  - internal/core/move_load.go
  - internal/core/move_rewrite.go
  - internal/core/move_apply.go
  - internal/core/move_link.go
  - internal/core/frontmatter_path_guard.go
---

# 共有ヘルパー呼び出しサイト対応表

変更時に責務の入口を探すための索引。詳細な caller 列挙は `rg` で再抽出し、ここでは横断利用される境界だけを示す。

| 領域 | 定義の入口 | 主な利用側 |
|---|---|---|
| rewrite | `rewrite.go:25` `isPathLinkType`、`82` `rewriteRawLink`、`199` `applyFileRewritesWithRollbackFailures` | build、add、disambiguate、move、repair、simplify |
| ファイル復元 | `rewrite.go:163` `restoreBackupFiles`、`182` `wrapRollbackFailures` | add、disambiguate、move-dir、set |
| path / basename | `util.go:16` `NormalizePath`、`96` `basenameKey` | index、resolve、move、query の path 境界 |
| resolve map | `resolve_maps.go:143` `newResolveMaps` | build、meta-check、simplify |
| move 準備 | `move_load.go:56` `loadSingleMoveFromDB`、`192` `classifyDiskState` | move、move-dir、move-template |
| move rewrite 集計 | `move_rewrite.go:92` incoming、`194` collateral、`262` moved-file outgoing | `move_dir.go:276` の準備経路 |
| move 適用 | `move_apply.go:13` prepare、`34` apply、`58` DB 反映 | move-dir と add / disambiguate |
| moved outgoing | `move_link.go:21` 判定、`137` relative 再計算 | `move_rewrite.go:305` と `move_link.go:53` |

`frontmatter_path` は `isPathLinkType` の検証対象だが rewrite 対象ではない。この違いに触れる変更は `rewrite.go` と `frontmatter_path_guard.go` を併読する。
