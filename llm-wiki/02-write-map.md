---
regen: compiled
sources:
  - internal/core/build.go
  - internal/core/build_prepare.go
  - internal/core/add.go
  - internal/core/update.go
  - internal/core/delete.go
  - internal/core/set.go
  - internal/core/move.go
  - internal/core/move_dir.go
  - internal/core/move_load.go
  - internal/core/move_rewrite.go
  - internal/core/move_apply.go
  - internal/core/move_link.go
  - internal/core/disambiguate.go
  - internal/core/simplify.go
  - internal/core/convert.go
  - internal/core/repair.go
  - internal/core/init_meta.go
  - internal/core/rewrite.go
  - internal/core/frontmatter_path_guard.go
  - cmd/mdhop/init_meta.go
  - docs/decisions/0003-move-auto-detects-disk-state.md
  - docs/decisions/0004-root-priority-for-ambiguous-basename.md
  - docs/decisions/0005-delete-rm-flag.md
  - docs/decisions/0008-move-collateral-rewrite.md
  - docs/decisions/0012-remove-external-rewrite-stale-check.md
---

# 書き込み系コマンドの破壊性・波及マップ

書き込み経路の入口と、ディスク・DB のどちらを先に確認すべきかの地図。振る舞いの正本は各コマンドの `docs/specs/overview.md` と ADR を読む。

| コマンド | 主な変更先 | 読む入口 |
|---|---|---|
| `build` | temp DB を rename で置換 | 入力収集・検証は `build_prepare.go:30`、DB 書き込みは `build.go:31` |
| `add` / `update` / `set` | note、edge、meta | `add.go:32` / `update.go:24` / `set.go:37` |
| `delete` | node 削除または phantom 化、`--rm` はディスク削除 | `delete.go:25`、理由は ADR 0005 |
| `move` / `move-dir` | rename と incoming / collateral / outgoing rewrite | `move.go:22` → `move_dir.go:70` |
| `disambiguate` / `simplify` / `repair` / `convert` | 対象ファイルのリンク表記 | 各 core ファイルの公開関数を入口にする |
| `init-meta --write` | vault root の `mdhop.yaml` | `cmd/mdhop/init_meta.go` と `internal/core/init_meta.go` |

## build の境界

`Build` は `prepareBuild` でファイル収集・解析・リンク検証を完了してから temp DB を作る。temp ファイルの除去に失敗した場合も既存 DB を置換しない。build の失敗処理を変えるときは `build.go:31` と `build_prepare.go:30` を対で読む。

## move の責務分割

単体 `Move` は移動情報を準備して `executeMoves` へ委譲し、directory mode も同じ executor を通る。incoming / collateral の収集は `move_rewrite.go:92` / `194`、移動 note の outgoing 収集は `move_rewrite.go:262`、個別の outgoing 判定と相対パス再計算は `move_link.go:21` / `137` が所有する。

書き込み前の候補検証・適用・復元は `rewrite.go` と `move_apply.go` に分離されている。raw frontmatter path は書き換えず、移動後も解決先が変わらないことを `frontmatter_path_guard.go` で検証する。理由は ADR 0014 を参照する。

## 更新順序の注意

通常の mutation はディスク操作を先に行い、その後 DB transaction を反映する。build は例外で、入力検証後に temp DB を完成させてから rename する。失敗時の復元範囲はコマンドごとに異なるため、実装変更時は該当関数の rollback 経路と ADR 0005 / 0012 を確認する。
