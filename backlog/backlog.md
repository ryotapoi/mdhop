# Backlog

## v0.9.0 — 汎用診断機能改善

特定領域だけを path filter で絞れる汎用診断機能群。特定フォルダ・特定運用名に依存する専用機能は作らない。詳細・設計方針・受け入れ条件は [v0.9.0-diagnostics.md](v0.9.0-diagnostics.md)。

- [ ] `diagnose --path` で対象範囲を絞る（最優先。vault 全体の phantom ノイズ解消）
- [ ] path filter（`--path` / `--include-path` / `--exclude-path`）をコマンド間で統一
- [ ] `reachable --from <entry> --path <glob>` 到達性チェック
- [ ] frontmatter 任意 key の path-like value 検査（`meta-check`）
- [ ] `changed --since` 変更ファイル列挙（git との差分を見てから要否判断）
- [ ] frontmatter schema validation（`meta-validate`: 必須 key・enum・型・空値。mdhop.yaml の `meta.types` を検査側でも使う）
- [ ] search の computed fields（行数・リンク数を `--fields` / `--sort` で）+ `--fields` の meta key 出力対応
- [ ] frontmatter key の raw path 値を graph edge にする設定（`meta.link_keys`。`related:` / `sources:` を backlinks / reachable に反映）
- [ ] subgraph export（`graph --path` で node / edge を JSON dump。類似判定等は呼び出し側）
- [ ] `--where` の相対日付比較（`updated<today-90d`。**実施決定**）
- [ ] `[[note#見出し]]` の anchor 切れ検出（**実装が軽い場合のみ**。Obsidian 互換の fragment 正規化が重ければ見送り）

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] `parseFrontmatter` の責務分離検討
  - **問題**: 現状 `parseFrontmatter` は tags / meta / frontmatter_wikilink の 3 系統を返す。今後さらに追加する種別があると肥大化する
  - **対応**: 戻り値が 4 系統以上になるタイミングで `parseResult` 構造体ベースへ移行する判断
  - **由来**: design レビュー（v0.7.0 frontmatter 内 wikilink 対応 (1/2)）で予兆として記録
