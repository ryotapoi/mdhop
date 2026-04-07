# Backlog

## v0.6.1

- [ ] build.go からファイル収集ヘルパーを `collect.go` に分離（`collectMarkdownFiles`, `collectAssetFiles`, `countBasenames`, `countAssetBasenames`, `escapesVault`, `pathEscapesVault`）
- [ ] util.go から scan 用 resolve maps を `resolve_maps.go` に統合（`noteResolveMaps`, `assetResolveMaps`, `buildNoteResolveMaps`, `buildAssetResolveMaps`）

## v0.7.0

- [ ] `--where` NOT EXISTS 演算子（特定キーを持たないノートの検索。現状 EXISTS の逆がない）
- [ ] サンプルスキル更新（`examples/skills/` 配下を最新仕様に合わせる）

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
