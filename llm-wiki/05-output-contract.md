---
regen: compiled
sources:
  - cmd/mdhop/main.go
  - cmd/mdhop/format.go
  - cmd/mdhop/format_query.go
  - cmd/mdhop/format_inspect.go
  - cmd/mdhop/format_resolve.go
  - cmd/mdhop/format_stats.go
  - cmd/mdhop/format_diagnose.go
  - cmd/mdhop/format_search.go
  - cmd/mdhop/format_reachable.go
  - cmd/mdhop/format_graph.go
  - cmd/mdhop/format_status.go
  - cmd/mdhop/format_meta_check.go
  - cmd/mdhop/format_meta_validate.go
  - cmd/mdhop/format_set.go
  - cmd/mdhop/status.go
  - internal/core/query.go
  - internal/core/inspect.go
  - docs/specs/overview.md
---

# stdout 出力契約ガイド

stdout は主結果、stderr は warning・hint・error・usage の経路。フィールドの正本は `docs/specs/overview.md`、実装の変更入口は次の表で辿る。

| 出力経路 | 実装の入口 | 確認点 |
|---|---|---|
| 共通 JSON / text error | `format.go:98` `textErrorWriter`、`118` `encodeJSON` | writer error を caller まで返す |
| CLI error | `main.go` `formatCommandError` | 失敗時は stderr と非ゼロ終了 |
| query | `format_query.go` `printQueryJSON` / `printQueryText` | 関係の選択と空配列、`page`、対象・経由先の preview の出力境界 |
| inspect | `format_inspect.go` `printInspectJSON` / `printInspectText` | 選択した tags / meta、指定した head の空値と省略の区別 |
| graph dot | `format_graph.go:36` `printGraphDot` | text writer 経由で error を返す |
| status | `status.go:26`、`format_status.go:10` / `22` | JSON は `untracked` / `modified` / `deleted`、操作は read-only |
| set | `format_set.go:18` `printSetText` / `33` `printSetJSON` | JSON の `value` は scalar write で string、`--list` で string array。text の list 値は compact JSON array |
| meta-check | `format_meta_check.go:22` / `36` | JSON の `line`、text の `location` |
| meta-validate | `format_meta_validate.go:22` / `36` | missing は line 1、その他は値の位置 |

query は未選択の関係を省略し、選択済みで対象がない関係を空配列にする。inspect は選択していない属性を省略する。core 側の nil / 空の区別は `internal/core/query.go` の `QueryResult` と `internal/core/inspect.go` の `InspectResult`、JSON 側は各 formatter の pointer フィールドを確認する。詳細な形状は [query の出力](../docs/specs/overview.md#query-の-json-と-text) と [inspect の CLI 契約](../docs/specs/overview.md#inspect-の-cli-契約)を参照する。status と meta diagnostics の JSON 形状を変更する場合は、このページの source と `cmd/mdhop/format_test.go` を読む。

meta diagnostics の位置は index snapshot 由来であり、既存 index を line 対応へ更新するときの正本は [コマンド詳細](../docs/specs/overview.md#コマンド詳細必須任意)。stdout に warning や hint を混在させない。

`set` の振る舞い仕様（`--list`、出力形状を含む）の正本は [コマンド詳細](../docs/specs/overview.md#コマンド詳細必須任意)を参照する。
