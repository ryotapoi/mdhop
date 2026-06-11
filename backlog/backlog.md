# Backlog

## v0.11.0 — frontmatter 検査と search 強化

frontmatter の品質検査と search の強化。v0.10.0 を実 vault に当てた結果（残る phantom、`link_keys` の効き方）を設計判断の材料にする。詳細は [diagnostics.md](diagnostics.md)。

- [ ] frontmatter 任意 key の path-like value 検査（`meta-check`）
- [ ] frontmatter schema validation（`meta-validate`: 必須 key・enum・型・空値。mdhop.yaml の `meta.types` を検査側でも使う）
- [ ] search の computed fields（行数・リンク数を `--fields` / `--sort` で）+ `--fields` の meta key 出力対応
- [x] `--where` の相対日付比較（`updated<today-90d`。**実施決定**）
- [ ] `changed --since` 変更ファイル列挙（git との差分を見てから要否判断）
- [ ] `[[note#見出し]]` の anchor 切れ検出（**実装が軽い場合のみ**。Obsidian 互換の fragment 正規化が重ければ見送り）
- [ ] anchor 検査の設計前に convert.go のパース骨格重複（`parseLinksForConvert` / `parseMarkdownSelfLinks` が parseLinks 系の走査ループを再実装）を畳むか判断する（本文走査をもう 1 種類足すと三重化するため。2026-06-10 audit 由来）
- [ ] computed fields の設計時に search.go の count / main クエリの WHERE 二重適用を query builder 化するか検討する（SELECT 句が動的になると崩れやすい。2026-06-10 audit 由来）
- [ ] examples skill と README / README.ja を v0.11.0 追加分に同期

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] パース層の package 化の再評価
  - **trigger**: v0.10.0 の `meta.link_keys` 着地後、parse 層の入出力（MetaConfig 注入）が安定した時点
  - **経緯**: internal/core は 30 ファイルで `rules/architecture.md` の分割検討条件（20 ファイル超 + 責務グループ明確）に到達。2026-06-10 の audit + module-boundary 判断で「mutation クラスタは resolveMaps / rewriteEntry / dbExecer の密共有で export 面が大きく未成熟、パース層のみが候補だが link_keys で入出力が変わる直前」として package 分割は見送り、file 境界整理（v0.9.0）のみ実施
- [ ] move 系（move.go / move_dir.go / move_helpers.go 計 1,602 行、18 関数パイプライン）の分割再評価
  - **trigger**: move に次の機能要求が来た時点（v0.10.0 / v0.11.0 では触らない）
  - **経緯**: 凝集はしているが backup / rollback のベストエフォート復元コードが分散（2026-06-10 audit）
