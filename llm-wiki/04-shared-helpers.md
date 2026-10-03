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
  - internal/core/query_content.go
---

# 共有ヘルパー呼び出しサイト対応表

変更時に責務の入口を探すための索引。詳細な caller 列挙は `rg` で再抽出し、ここでは横断利用される境界だけを示す。

| 領域 | 定義の入口 | 主な利用側 |
|---|---|---|
| rewrite | `rewrite.go:26` `isPathLinkType`、`112` `rewriteRawLink`、`231` `applyFileRewritesWithRollbackFailures` | build、add、disambiguate、move、repair、simplify |
| ファイル復元 | `rewrite.go:185` `restoreBackupFiles`、`214` `wrapRollbackFailures` | add、disambiguate、move-dir、set |
| head 抽出 | `query_content.go:11` `readHead` | query、search の `--include-head` |
| path / basename | `util.go:17` `NormalizePath`、`158` `basenameKey` | index、resolve、move、query の path 境界 |
| resolve map | `resolve_maps.go:174` `newResolveMaps` | build、meta-check、simplify |
| move 準備 | `move_load.go:56` `loadSingleMoveFromDB`、`206` `classifyDiskState` | move、move-dir、move-template |
| move rewrite 集計 | `move_rewrite.go:95` incoming、`197` collateral、`251` moved-file outgoing | `move_dir.go:289` の準備経路 |
| move 適用 | `move_apply.go:11` prepare、`40` apply、`80` DB 反映 | `move_dir.go:75` の共通 executor |
| moved outgoing | `move_link.go:22` 判定、`144` relative 再計算 | `move_rewrite.go:251` と `move_link.go:75` |

`frontmatter_path` は `isPathLinkType` の検証対象だが rewrite 対象ではない。この違いに触れる変更は `rewrite.go` と `frontmatter_path_guard.go` を併読する。

move の復元対象には移動 note と外部 rewrite ファイルの元 mtime も含む。`move_apply.go:40` で backup し、`rewrite.go:185` で復元する。`readHead` は必要な行だけを結果用 slice にコピーし、ファイル読取エラーは返す。出力の仕様は `docs/specs/overview.md` を参照する。
