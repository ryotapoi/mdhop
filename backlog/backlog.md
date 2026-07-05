# Backlog

## v0.16.0

`--where` 結合ルールの統一（2026-07-05 確定）。

- [ ] search/query: 複数 `--where` フラグを常に AND にする（同一キー OR の暗黙ルールを廃止）
  - **現状**: 複数 `--where` の条件はキーでグループ化され、同一キーは OR・異なるキーは AND（INTERSECT）で結合される（`internal/core/where.go:286` の `MetaFilterSQL`）。このルールは v0.6.0 計画時（f7fa44f）、` && ` も ` || ` もなかった「1 フラグ = 1 条件」の世界で「同一キーの等値 AND は常に空集合なので OR 以外に意図がありえない」というキー一致からの意図推測として入ったもので、明文化された設計記録はない。` && `（dbe3aca）と ` || `（2be48c3）の追加で明示演算子が揃い、役目を終えた。`!=` は同一キーでも除外の積（実質 AND）でルール自体に例外があった
  - **確定仕様**: 複数 `--where` フラグ間は常に AND（キーの一致を見ない）。OR は 1 つの式内の ` || ` で明示する。`!=` の除外セマンティクス（キーが存在し、かつ該当値を一切含まない）は変更しない
  - **破壊的変更**: `--where "k=a" --where "k=b"` が OR → AND（スカラーキーでは空集合）に変わる。v1.0 前に実施する
  - **対処案**: `MetaFilterSQL` のキー単位グループ化をフラグ（式）単位のグループ化に変更し、式グループ間を INTERSECT で結合。同一キー OR を検証している既存テストは AND 検証 + ` || ` 書き換え例に差し替え
  - **ドキュメント同期**（同一コミット）: `docs/specs/overview.md` の `--where` 結合ルール節、`docs/rules/02-requirements.md:78`、`docs/rules/03-data-model.md:193-194` の SQL 生成パターン、`cmd/mdhop/usage.go`（query / search の `--where` 説明に「複数フラグは AND」を明記）。README と `examples/skills/mdhop/` は結合ルールを記述していないことを確認済み（2026-07-05）だが、実装時に念のため再確認する

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] パース層の package 化の再評価
  - **trigger**: v0.10.0 の `meta.link_keys` 着地後、parse 層の入出力（MetaConfig 注入）が安定した時点
  - **経緯**: internal/core は 30 ファイルで `docs/rules/architecture.md` の分割検討条件（20 ファイル超 + 責務グループ明確）に到達。2026-06-10 の audit + module-boundary 判断で「mutation クラスタは resolveMaps / rewriteEntry / dbExecer の密共有で export 面が大きく未成熟、パース層のみが候補だが link_keys で入出力が変わる直前」として package 分割は見送り、file 境界整理（v0.9.0）のみ実施

## 登録見送り（2026-07-03 maintenance audit）

- package 分割・format_meta_* 重複統合・同型スケルトン共通化: ignore 判断（便益がコストを下回る）
- meta_check.go / meta_validate.go のリネーム: 不採用。mdhop の慣例「CLI コマンド名 = ソースファイル名」を壊し、`mdhop meta-check` の実装を探す際の新しい乖離を作る。責務の判別問題は各ファイル先頭の doc comment で解決する（v0.13.0 の meta-validate プロファイル実装時に付与）
