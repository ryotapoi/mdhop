# Backlog

## Next

- [x] wikilink と markdown link の相互変換
- [ ] 相対パス指定（`./` や `../`）で同名ファイルが vault 内に1つしかない場合、`[[basename]]` に変換する
- [ ] 絶対パス指定（ルート以外への `path/to` 書式）で同名ファイルが vault 内に1つしかない場合、`[[basename]]` に変換する
- [x] 非 .md ファイルのリンク管理（画像、PDF 等を SQLite で管理し、move/delete で壊れないようにする）
- [ ] ミューテーション系 CLI が `*Result` を捨てている → 書き換え結果等を表示すべき

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
