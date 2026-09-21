---
regen: compiled
sources:
  - cmd/mdhop/main.go
  - cmd/mdhop/format.go
  - cmd/mdhop/format_query.go
  - cmd/mdhop/format_resolve.go
  - cmd/mdhop/format_stats.go
  - cmd/mdhop/format_diagnose.go
  - cmd/mdhop/format_search.go
  - cmd/mdhop/format_reachable.go
  - cmd/mdhop/format_graph.go
  - cmd/mdhop/format_status.go
  - cmd/mdhop/format_meta_check.go
  - cmd/mdhop/format_meta_validate.go
  - cmd/mdhop/status.go
  - docs/specs/overview.md
---

# stdout 出力契約ガイド

stdout は主結果、stderr は warning・hint・error・usage の経路。フィールドの正本は `docs/specs/overview.md`、実装の変更入口は次の表で辿る。

| 出力経路 | 実装の入口 | 確認点 |
|---|---|---|
| 共通 JSON / text error | `format.go:98` `textErrorWriter`、`118` `encodeJSON` | writer error を caller まで返す |
| CLI error | `main.go:84` `formatCommandError` | 失敗時は stderr と非ゼロ終了 |
| graph dot | `format_graph.go:36` `printGraphDot` | text writer 経由で error を返す |
| status | `status.go:26`、`format_status.go:10` / `22` | JSON は `untracked` / `modified` / `deleted`、操作は read-only |
| meta-check | `format_meta_check.go:22` / `36` | JSON の `line`、text の `location` |
| meta-validate | `format_meta_validate.go:22` / `36` | missing は line 1、その他は値の位置 |

query は要求しないフィールドを `omitempty` で省略する。status と meta diagnostics の JSON 形状を変更する場合は、このページの source と `cmd/mdhop/format_test.go` を読む。

meta diagnostics の位置は index snapshot 由来であり、既存 index を line 対応へ更新するときの正本は `docs/specs/overview.md:392` / `402`。stdout に warning や hint を混在させない。
