# Backlog

## v0.5.0

- [ ] `move.go` の重複統合
  - 1,735行。`Move`(L25〜630) と `MoveDir`(L690〜1521) が共通ロジックを独立実装
  - `rewriteOutgoingRelativeLink`(L1639) と `rewriteOutgoingRelativeLinkBatch`(L1525) は同一アルゴリズムの2重実装。差分は batch 版が `movedFromTo` マップを参照する点のみ
  - incoming rewrite フェーズ、ロールバック処理も対称的に重複
  - 一方のバグ修正が他方に漏れるリスクが高い。カバレッジ40%台の原因でもある
  - 対応: rewrite 2関数を統合（`movedFromTo=nil` で単一版として動作）。共通フェーズのヘルパー抽出。必要に応じて `move_single.go` / `move_dir.go` / `move_internal.go` に分割
  - 影響: `move.go`, `move_test.go`

- [ ] DB 存在チェック集約
  - `"index not found: run 'mdhop build' first"` が12箇所にハードコード
  - 箇所: `resolve.go:23`, `stats.go:26`, `update.go:28`, `delete.go:28`, `add.go:35`, `query.go:68`, `disambiguate.go:28`, `diagnose.go:31`, `move.go:28,693`, `db.go:324,342`
  - 対応: `db.go` に `openDBChecked(vaultPath string) (*sql.DB, error)` を追加し12箇所を置換

- [ ] `tagKey()` 関数追加
  - `"tag:name:%s"` が `db.go:179`, `resolve.go:100`, `query.go:224` にハードコード
  - `noteKey()` / `assetKey()` / `phantomKey()` は関数化済みなのに `tagKey()` だけ欠落
  - タグの正規化ルールが変わった場合に3箇所の同時修正が必要
  - 対応: `db.go` に `tagKey(name string) string` を追加し3箇所を置換

- [ ] `upsertNote` / `upsertAsset` 統合
  - `db.go:78-104` と `db.go:114-141` が INSERT SQL・ON CONFLICT 処理・`LastInsertId` フォールバックまで実質同一
  - 差異は `node_key` 構築関数（`noteKey` vs `assetKey`）と `type` リテラルのみ
  - 過去に `LastInsertId` バグがあった箇所（MEMORY.md 記載）で、片方だけ修正されるリスク
  - 対応: 共通の `upsertNode(exec, key, typ, name, path, mtime)` に統合

- [ ] `printRepairText/JSON` と `printSimplifyText/JSON` の重複解消
  - `format.go:685-698` と `format.go:731-745` の `skipped` 出力が完全同一
  - JSON 版（`format.go:701-722` と `format.go:747-768`）も同様
  - 対応: `printSkippedText(w, skipped)` と共通 JSON 型への切り出し

- [ ] エラーラッピング `%w` 統一
  - `%w` は `config.go:49` の1箇所のみ、残り98箇所の `fmt.Errorf` はすべて `%s`
  - 現状は `errors.Is(err, flag.ErrHelp)` しか使っておらず実害なし
  - 将来 `errors.Is` / `errors.As` での分岐が必要になった際に制約になる
  - `build.go:337,352` の `fmt.Errorf("%s", s)` は `errors.New(s)` が意図に近いアンチパターン
  - 対応: `%s` → `%w` 一括置換。テストのエラー文字列アサーションも要確認

- [ ] `queryCollateralRewrites` (カバレッジ 0%)
  - `move.go:632`。MoveDir の Phase 2.5 で root-priority が変化する際に第三者ファイルの basename リンクを修正する処理
  - テストなしだとサイレントにリンクが壊れるリスク
  - 対応: MoveDir で root-priority が変わるシナリオの fixture + テストケース追加
  - 依存: move.go 統合の後にやるとテスト対象が安定する

- [ ] `resolvePathFromDB` (カバレッジ 26.1%)
  - `resolve.go:149`。Resolve の根幹ロジックで note exact → note+.md → asset exact → phantom のフォールバックチェーンを担う
  - asset 経由と phantom fallback のケースが未テスト
  - 対応: path link + asset の組み合わせテスト、phantom fallback テストを `resolve_test.go` に追加

- [ ] `upsertNote`(54%), `parseFrontmatter`(59%), `resolvePathTarget`(64%)
  - `upsertNote`/`upsertAsset`: ON CONFLICT UPDATE パスで `id == 0` の場合に再クエリする分岐が未カバー
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
