# Backlog

## v0.5.0

- [x] `move.go` の重複統合
  - 完了: move.go(498行), move_dir.go(763行), move_helpers.go(263行) に分割
  - 7ヘルパー抽出: rewriteOutgoingRelativeLink統合, queryCollateralRewrites, groupAndApplyExternalRewrites, applyOutgoingRewritesToContent, updateExternalEdgesAndMtimes, promotePhantom, outgoingRewrite型

- [x] DB 存在チェック集約
  - 完了: `db.go` に `openDBChecked(vaultPath string) (*sql.DB, error)` を追加し12箇所を置換

- [x] `tagKey()` 関数追加
  - `"tag:name:%s"` が `db.go:179`, `resolve.go:100`, `query.go:224` にハードコード
  - `noteKey()` / `assetKey()` / `phantomKey()` は関数化済みなのに `tagKey()` だけ欠落
  - タグの正規化ルールが変わった場合に3箇所の同時修正が必要
  - 対応: `db.go` に `tagKey(name string) string` を追加し3箇所を置換

- [x] `upsertNote` / `upsertAsset` 統合
  - 完了: 共通の `upsertNode(db, key, typ, name, path, mtime)` に統合。conflict-update パスのテスト追加

- [x] `printRepairText/JSON` と `printSimplifyText/JSON` の重複解消
  - 完了: `printSkippedText`, `toSkippedJSON`, `rewriteResultJSONOutput`, `printRewriteResultJSON` を抽出し repair/simplify の4関数を委譲に簡素化

- [x] エラーラッピング `%w` 統一
  - 完了: 98箇所の `fmt.Errorf("%s",...)` はすべて string 埋め込みで `%w` 置換対象なし。`fmt.Errorf("%s", s)` アンチパターン3箇所（build.go×2, query.go×1）を `errors.New()` に修正

- [x] `queryCollateralRewrites` (カバレッジ 0%)
  - `move.go:632`。MoveDir の Phase 2.5 で root-priority が変化する際に第三者ファイルの basename リンクを修正する処理
  - テストなしだとサイレントにリンクが壊れるリスク
  - 対応: MoveDir で root-priority が変わるシナリオの fixture + テストケース追加
  - 依存: move.go 統合完了済み。move_helpers.go にヘルパーが安定
  - 完了: MoveDir Phase 2.5 テスト2件追加（preRoot=F,postRoot=F 分岐 + isBasenameRawLink フィルタ）。カバレッジ 85%→90%

- [ ] `resolvePathFromDB` (カバレッジ 26.1%)
  - `resolve.go:149`。Resolve の根幹ロジックで note exact → note+.md → asset exact → phantom のフォールバックチェーンを担う
  - asset 経由と phantom fallback のケースが未テスト
  - 対応: path link + asset の組み合わせテスト、phantom fallback テストを `resolve_test.go` に追加

- [ ] `parseFrontmatter`(59%), `resolvePathTarget`(64%)
  - `parseFrontmatter`: frontmatter の `tags` が scalar（コンマ区切り）形式の場合が未テスト
  - `resolvePathTarget`: build Pass 2 の中枢関数で asset 経由の path 解決と phantom fallback が薄い
  - 対応: 各関数のテストケース追加。`build_test.go`, `parse_test.go` に追加

## v0.6.0

- [ ] frontmatter メタデータの SQLite 格納と構造化クエリ
  - frontmatter のキーバリューを DB に格納（本文は入れない。ファイルシステムが SSoT）
  - 構造化条件での絞り込みクエリ対応（`--where "status=draft"` 等）
  - タグ + frontmatter の複合検索
  - 目的: Coding Agent のコンテキスト管理コスト削減。管理情報を SQLite に集約し、CLI 経由でアクセスパターンを制限する
  - v0.5.0 完了後にタスク分解する

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
