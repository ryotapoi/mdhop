# Backlog

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] `parseFrontmatter` の責務分離検討
  - **問題**: 現状 `parseFrontmatter` は tags / meta / frontmatter_wikilink の 3 系統を返す。今後さらに追加する種別があると肥大化する
  - **対応**: 戻り値が 4 系統以上になるタイミングで `parseResult` 構造体ベースへ移行する判断
  - **由来**: design レビュー（v0.7.0 frontmatter 内 wikilink 対応 (1/2)）で予兆として記録
