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

振る舞い仕様の正本: `docs/specs/overview.md`（§コマンド詳細（必須/任意）, 行193–）

## インデックス系コマンド

| コマンド | cmd/mdhop 実装ファイル (run 関数:行) | internal/core 中核関数 (ファイル:行) | 仕様ポインタ |
|---|---|---|---|
| `build` | `build.go:9` `runBuild` | `build.go:19` `Build` | overview.md:195 |
| `add` | `add.go:11` `runAdd` | `add.go:33` `Add` | overview.md:208 |
| `update` | `update.go:11` `runUpdate` | `update.go:26` `Update` | overview.md:203 |
| `delete` | `delete.go:23` `runDelete` | `delete.go:25` `Delete` | overview.md:234 |
| `move` | `move.go:12` `runMove` | `move.go:25` `Move` / `move_dir.go:28` `MoveDir` | overview.md:215 |
| `disambiguate` | `disambiguate.go:11` `runDisambiguate` | `disambiguate.go:26` `Disambiguate` / `disambiguate.go:275` `DisambiguateScan` | overview.md:242 |
| `simplify` | `simplify.go:11` `runSimplify` | `simplify.go:26` `Simplify` | overview.md:264 |
| `repair` | `repair.go:11` `runRepair` | `repair.go:35` `Repair` | overview.md:250 |
| `convert` | `convert.go:11` `runConvert` | `convert.go:25` `Convert` | overview.md:277 |

## クエリ系コマンド

| コマンド | cmd/mdhop 実装ファイル (run 関数:行) | internal/core 中核関数 (ファイル:行) | 仕様ポインタ |
|---|---|---|---|
| `resolve` | `resolve.go:11` `runResolve` | `resolve.go:21` `Resolve` | overview.md:287 |
| `query` | `query.go:10` `runQuery` | `query.go:88` `Query` | overview.md:290 |
| `search` | `search.go:10` `runSearch` | `search.go:94` `Search` | overview.md:295 |
| `reachable` | `reachable.go:10` `runReachable` | `reachable.go:38` `Reachable` | overview.md:337 |
| `graph` | `graph.go:11` `runGraph` | `graph.go:48` `Graph` | overview.md:345 |
| `stats` | `stats.go:10` `runStats` | `stats.go:19` `Stats` | overview.md:354 |
| `diagnose` | `diagnose.go:10` `runDiagnose` | `diagnose.go:257` `Diagnose` | overview.md:312 |
| `meta-check` | `meta_check.go:10` `runMetaCheck` | `meta_check.go:54` `MetaCheck` | overview.md:320 |
| `meta-validate` | `meta_validate.go:10` `runMetaValidate` | `meta_validate.go:53` `MetaValidate` | overview.md:329 |

## セットアップ系コマンド

| コマンド | cmd/mdhop 実装ファイル (run 関数:行) | internal/core 中核関数 (ファイル:行) | 仕様ポインタ |
|---|---|---|---|
| `init-meta` | `init_meta.go:12` `runInitMeta` | `init_meta.go:49` `InitMeta` | overview.md:357 |

## 備考

- `move` は `--from` 末尾 `/` またはディスク上ディレクトリの場合に `MoveDir` へ分岐（条件 `cmd/mdhop/move.go:33` `if fromIsDir`、呼び出し `cmd/mdhop/move.go:41` `core.MoveDir`）
- `disambiguate` は `--scan` フラグ指定時に `DisambiguateScan` へ分岐 (`cmd/mdhop/disambiguate.go:32`)
- フォーマッタは各 `cmd/mdhop/format_<cmd>.go` に分離。共通ヘルパーは `cmd/mdhop/format.go`
