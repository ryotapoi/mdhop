# Backlog

v0.11.0 リリース済み（tag + GitHub Release, 2026-06-11）。完了項目は v0.11.0 タグのコミット履歴に残る。次は v0.12.0（スコープ: タスク 14〜16）。実 vault 当ての所見は [diagnostics.md](diagnostics.md)。

## v0.12.0（2026-06-11 LLM Wiki lint 所見、実装前提）

詳細は [diagnostics.md](diagnostics.md) のタスク 14〜16。

- [x] 14. index 登録パスの Unicode 正規化（NFC 統一）— NFD ファイル名で二重ノード登録・delete 不能になる。データ整合性の問題なので優先度高め
- [ ] 15. meta-check のディレクトリ参照対応 — 末尾 `/` の値をディレクトリとして存在チェック
- [ ] 16. repair に path filter — `--path` / `--exclude` を他コマンドと揃える

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] パース層の package 化の再評価
  - **trigger**: v0.10.0 の `meta.link_keys` 着地後、parse 層の入出力（MetaConfig 注入）が安定した時点
  - **経緯**: internal/core は 30 ファイルで `rules/architecture.md` の分割検討条件（20 ファイル超 + 責務グループ明確）に到達。2026-06-10 の audit + module-boundary 判断で「mutation クラスタは resolveMaps / rewriteEntry / dbExecer の密共有で export 面が大きく未成熟、パース層のみが候補だが link_keys で入出力が変わる直前」として package 分割は見送り、file 境界整理（v0.9.0）のみ実施
- [ ] move 系（move.go / move_dir.go / move_helpers.go 計 1,602 行、18 関数パイプライン）の分割再評価
  - **trigger**: move に次の機能要求が来た時点（v0.10.0 / v0.11.0 では触らない）
  - **経緯**: 凝集はしているが backup / rollback のベストエフォート復元コードが分散（2026-06-10 audit）
