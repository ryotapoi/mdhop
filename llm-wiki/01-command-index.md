---
regen: full
sources:
  - cmd/mdhop/main.go
  - cmd/mdhop/build.go
  - cmd/mdhop/resolve.go
  - cmd/mdhop/query.go
  - cmd/mdhop/inspect.go
  - cmd/mdhop/format_query.go
  - cmd/mdhop/format_inspect.go
  - cmd/mdhop/stats.go
  - cmd/mdhop/diagnose.go
  - cmd/mdhop/status.go
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
  - internal/core/query_fetch.go
  - internal/core/query_filter.go
  - internal/core/query_preview.go
  - internal/core/query_content.go
  - internal/core/inspect.go
  - internal/core/stats.go
  - internal/core/diagnose.go
  - internal/core/status.go
  - internal/core/meta_check.go
  - internal/core/meta_validate.go
  - internal/core/delete.go
  - internal/core/set.go
  - internal/core/update.go
  - internal/core/add.go
  - internal/core/move.go
  - internal/core/move_dir.go
  - internal/core/move_template.go
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

ルーティング起点: `cmd/mdhop/main.go`（`switch os.Args[1]`）

振る舞い仕様の正本: [コマンド詳細](../docs/specs/overview.md#コマンド詳細必須任意)。`query` と `inspect` の選択・出力は同文書の各 CLI 契約と出力形式を参照する。

## インデックス系コマンド

| コマンド | `cmd/mdhop/` の入口 | `internal/core/` の入口 |
|---|---|---|
| `build` | `build.go` `runBuild` | `build.go` `Build` |
| `add` | `add.go` `runAdd` | `add.go` `Add` |
| `update` | `update.go` `runUpdate` | `update.go` `Update` |
| `set` | `set.go` `runSet` | `set.go` `Set` |
| `delete` | `delete.go` `runDelete` | `delete.go` `Delete` |
| `move` | `move.go` `runMove` | `move.go` `Move`、`move_dir.go` `MoveDir`、`move_template.go` `PlanMoveTemplate` / `MoveTemplate` |
| `disambiguate` | `disambiguate.go` `runDisambiguate` | `disambiguate.go` `Disambiguate` / `DisambiguateScan` |
| `simplify` | `simplify.go` `runSimplify` | `simplify.go` `Simplify` |
| `repair` | `repair.go` `runRepair` | `repair.go` `Repair` |
| `convert` | `convert.go` `runConvert` | `convert.go` `Convert` |

## クエリ系コマンド

| コマンド | `cmd/mdhop/` の入口 | `internal/core/` の入口 |
|---|---|---|
| `resolve` | `resolve.go` `runResolve` | `resolve.go` `Resolve` |
| `query` | `query.go` `runQuery` | `query.go` `Query`。関係取得は `query_fetch.go`、条件は `query_filter.go`、preview は `query_preview.go` |
| `inspect` | `inspect.go` `runInspect` | `inspect.go` `Inspect`。本文の head は `query_content.go` |
| `search` | `search.go` `runSearch` | `search.go` `Search` |
| `reachable` | `reachable.go` `runReachable` | `reachable.go` `Reachable` |
| `graph` | `graph.go` `runGraph` | `graph.go` `Graph` |
| `stats` | `stats.go` `runStats` | `stats.go` `Stats` |
| `diagnose` | `diagnose.go` `runDiagnose` | `diagnose.go` `Diagnose` |
| `status` | `status.go` `runStatus` | `status.go` `Status` |
| `meta-check` | `meta_check.go` `runMetaCheck` | `meta_check.go` `MetaCheck` |
| `meta-validate` | `meta_validate.go` `runMetaValidate` | `meta_validate.go` `MetaValidate` |

## セットアップ系コマンド

| コマンド | `cmd/mdhop/` の入口 | `internal/core/` の入口 |
|---|---|---|
| `init-meta` | `init_meta.go` `runInitMeta` | `init_meta.go` `InitMeta` |

## 備考

- `move` は `--from` 末尾 `/` またはディスク上ディレクトリの場合に directory mode へ分岐する。`--to-template` は `PlanMoveTemplate` で事前検証してから `MoveTemplate` を呼ぶ。通常の directory move は `MoveDir`（`cmd/mdhop/move.go`）。
- `disambiguate` は `--scan` フラグ指定時に `DisambiguateScan` へ分岐する（`cmd/mdhop/disambiguate.go`）。
- `query` の返却対象は `internal/core/query.go` で関係別に選び、本文 preview はページ決定後に `internal/core/query_preview.go` で付ける。`inspect` は関係を辿らず索引済みの一件を読む（`internal/core/inspect.go`）。
- フォーマッタは各 `cmd/mdhop/format_<cmd>.go` に分離。共通ヘルパーは `cmd/mdhop/format.go`。query / inspect の形式は `format_query.go` / `format_inspect.go`。
