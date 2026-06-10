# Backlog

## v0.9.0 — コード・ドキュメント全体見直し

v0.10.0 / v0.11.0 の大型機能追加の前に、コード全体とドキュメントを見直す整備リリース。観点は変更容易性・管理のしやすさ・わかりやすさ（実行速度は対象外）。次リリースで触る継ぎ目（path filter を入れる search / query / diagnose、parse / frontmatter 系、index スキーマ、format 出力）を重点に見る。タスクは仮置きで、見直し開始時に確定する。

- [ ] コード全体の構造見直し（モジュール境界・責務・重複・わかりやすさ）
- [ ] ドキュメント全体の見直し（rules/ / README / examples skill の整合・SSoT）
- [ ] `parseFrontmatter` の責務分離の要否判断（Later 項目参照。v0.11.0 の meta 系で圧がかかる前に）

## v0.10.0 — path filter とグラフ到達性

特定領域だけを path filter で絞れる汎用診断機能群の前半。「範囲を絞って診て、辿れる」を 1 つの capability として出す。特定フォルダ・特定運用名に依存する専用機能は作らない。詳細・設計方針・受け入れ条件は [diagnostics.md](diagnostics.md)。

- [ ] `diagnose --path` で対象範囲を絞る（最優先。vault 全体の phantom ノイズ解消）
- [ ] path filter（`--path` / `--include-path` / `--exclude-path`）をコマンド間で統一
- [ ] frontmatter key の raw path 値を graph edge にする設定（`meta.link_keys`。`related:` / `sources:` を backlinks / reachable に反映）
- [ ] `reachable --from <entry> --path <glob>` 到達性チェック（`meta.link_keys` の後に実装。raw path edge がないと false positive が出る）
- [ ] subgraph export（`graph --path` で node / edge を JSON dump。類似判定等は呼び出し側）
- [ ] examples skill（`examples/skills/mdhop/`）と README / README.ja を v0.10.0 追加分に同期

## v0.11.0 — frontmatter 検査と search 強化

frontmatter の品質検査と search の強化。v0.10.0 を実 vault に当てた結果（残る phantom、`link_keys` の効き方）を設計判断の材料にする。詳細は [diagnostics.md](diagnostics.md)。

- [ ] frontmatter 任意 key の path-like value 検査（`meta-check`）
- [ ] frontmatter schema validation（`meta-validate`: 必須 key・enum・型・空値。mdhop.yaml の `meta.types` を検査側でも使う）
- [ ] search の computed fields（行数・リンク数を `--fields` / `--sort` で）+ `--fields` の meta key 出力対応
- [ ] `--where` の相対日付比較（`updated<today-90d`。**実施決定**）
- [ ] `changed --since` 変更ファイル列挙（git との差分を見てから要否判断）
- [ ] `[[note#見出し]]` の anchor 切れ検出（**実装が軽い場合のみ**。Obsidian 互換の fragment 正規化が重ければ見送り）
- [ ] examples skill と README / README.ja を v0.11.0 追加分に同期

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] `parseFrontmatter` の責務分離検討
  - **問題**: 現状 `parseFrontmatter` は tags / meta / frontmatter_wikilink の 3 系統を返す。今後さらに追加する種別があると肥大化する
  - **対応**: 戻り値が 4 系統以上になるタイミングで `parseResult` 構造体ベースへ移行する判断
  - **由来**: design レビュー（v0.7.0 frontmatter 内 wikilink 対応 (1/2)）で予兆として記録
