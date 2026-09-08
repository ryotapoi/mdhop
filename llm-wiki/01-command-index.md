---
regen: full
sources:
  - cmd/mdhop/main.go
  - cmd/mdhop/build.go
  - cmd/mdhop/resolve.go
  - cmd/mdhop/query.go
  - cmd/mdhop/stats.go
  - cmd/mdhop/diagnose.go
  - cmd/mdhop/meta_check.go
  - cmd/mdhop/meta_validate.go
  - cmd/mdhop/delete.go
  - cmd/mdhop/set.go
  - cmd/mdhop/update.go
  - cmd/mdhop/add.go
  - cmd/mdhop/move.go
  - cmd/mdhop/disambiguate.go
  - cmd/mdhop/simplify.go
  - cmd/mdhop/repair.go
  - cmd/mdhop/convert.go
  - cmd/mdhop/search.go
  - cmd/mdhop/reachable.go
  - cmd/mdhop/graph.go
  - cmd/mdhop/init_meta.go
  - internal/core/build.go
  - internal/core/resolve.go
  - internal/core/query.go
  - internal/core/stats.go
  - internal/core/diagnose.go
  - internal/core/meta_check.go
  - internal/core/meta_validate.go
  - internal/core/delete.go
  - internal/core/set.go
  - internal/core/update.go
  - internal/core/add.go
  - internal/core/move.go
  - internal/core/move_dir.go
  - internal/core/disambiguate.go
  - internal/core/simplify.go
  - internal/core/repair.go
  - internal/core/convert.go
  - internal/core/search.go
  - internal/core/reachable.go
  - internal/core/graph.go
  - internal/core/init_meta.go
---

# コマンド索引

ルーティング起点: `cmd/mdhop/main.go:21`（`switch os.Args[1]`）

振る舞い仕様の正本: `docs/specs/overview.md:209`（「コマンド詳細（必須/任意）」）

## インデックス系コマンド

| コマンド | cmd/mdhop 実装ファイル (run 関数:行) | internal/core 中核関数 (ファイル:行) | 仕様ポインタ |
|---|---|---|---|
| `build` | `build.go:26` `runBuild` | `build.go:19` `Build` | overview.md:211 |
| `add` | `add.go:40` `runAdd` | `add.go:32` `Add` | overview.md:237 |
| `update` | `update.go:31` `runUpdate` | `update.go:24` `Update` | overview.md:220 |
| `set` | `set.go:44` `runSet` | `set.go:35` `Set` | overview.md:225 |
| `delete` | `delete.go:48` `runDelete` | `delete.go:25` `Delete` | overview.md:281 |
| `move` | `move.go:48` `runMove` | `move.go:22` `Move` / `move_dir.go:31` `MoveDir` | overview.md:244 |
| `disambiguate` | `disambiguate.go:33` `runDisambiguate` | `disambiguate.go:25` `Disambiguate` / `disambiguate.go:258` `DisambiguateScan` | overview.md:289 |
| `simplify` | `simplify.go:32` `runSimplify` | `simplify.go:25` `Simplify` | overview.md:311 |
| `repair` | `repair.go:33` `runRepair` | `repair.go:34` `Repair` | overview.md:297 |
| `convert` | `convert.go:32` `runConvert` | `convert.go:23` `Convert` | overview.md:324 |

## クエリ系コマンド

| コマンド | cmd/mdhop 実装ファイル (run 関数:行) | internal/core 中核関数 (ファイル:行) | 仕様ポインタ |
|---|---|---|---|
| `resolve` | `resolve.go:36` `runResolve` | `resolve.go:30` `Resolve` | overview.md:334 |
| `query` | `query.go:52` `runQuery` | `query.go:106` `Query` | overview.md:337 |
| `search` | `search.go:48` `runSearch` | `search.go:97` `Search` | overview.md:342 |
| `reachable` | `reachable.go:35` `runReachable` | `reachable.go:43` `Reachable` | overview.md:388 |
| `graph` | `graph.go:32` `runGraph` | `graph.go:48` `Graph` | overview.md:396 |
| `stats` | `stats.go:33` `runStats` | `stats.go:30` `Stats` | overview.md:406 |
| `diagnose` | `diagnose.go:34` `runDiagnose` | `diagnose.go:267` `Diagnose` | overview.md:361 |
| `meta-check` | `meta_check.go:37` `runMetaCheck` | `meta_check.go:60` `MetaCheck` | overview.md:370 |
| `meta-validate` | `meta_validate.go:37` `runMetaValidate` | `meta_validate.go:56` `MetaValidate` | overview.md:379 |

## セットアップ系コマンド

| コマンド | cmd/mdhop 実装ファイル (run 関数:行) | internal/core 中核関数 (ファイル:行) | 仕様ポインタ |
|---|---|---|---|
| `init-meta` | `init_meta.go:33` `runInitMeta` | `init_meta.go:49` `InitMeta` | overview.md:409 |

## 備考

- `move` は `--from` 末尾 `/` またはディスク上ディレクトリの場合に `MoveDir` へ分岐（条件 `cmd/mdhop/move.go:98` `if fromIsDir`、呼び出し `cmd/mdhop/move.go:105` `core.MoveDir`）
- `disambiguate` は `--scan` フラグ指定時に `DisambiguateScan` へ分岐 (`cmd/mdhop/disambiguate.go:54`)
- フォーマッタは各 `cmd/mdhop/format_<cmd>.go` に分離。共通ヘルパーは `cmd/mdhop/format.go`
