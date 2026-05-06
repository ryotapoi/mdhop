# Goal State

status: running

## Current scope

`backlog/backlog.md` の v0.7.0 の全タスク完了。

## Last completed loop

2026-05-07 (A パス): backlog v0.7.0「frontmatter 内 wikilink 対応 (2/2): 書き換えコマンド対応」を完了。

実装内容:
- `internal/core/rewrite.go`: `pathLinkTypeSQLList`（const）と `isPathLinkType()` を新設し、`'wikilink' / 'markdown' / 'frontmatter_wikilink'` の SSoT を集約。`rewriteRawLink` / `isBasenameRawLink` の `case "wikilink":` を `case "wikilink", "frontmatter_wikilink":` に拡張（同じ `[[...]]` 構文）
- `internal/core/add.go`: 検証ガード（line 249）を `isPathLinkType` に統一、auto-disambiguate の SQL に `frontmatter_wikilink` を追加
- `internal/core/update.go`: 検証ガード（line 136）を `isPathLinkType` に統一
- `internal/core/move.go`: 外向き basename リンクのフィルタと incoming SQL（2 箇所）を `isPathLinkType` / `pathLinkTypeSQLList` に置換
- `internal/core/move_helpers.go`: `collectIncomingRewritesForDir` / `lookupEdgeTargetPath` / `queryCollateralRewrites` の SQL と `buildMovedFileRewrites` のフィルタを `pathLinkTypeSQLList` / `isPathLinkType` に統一。コメント `wikilink/markdown link types` を `path-resolving link types (see pathLinkTypeSQLList)` に更新
- `internal/core/disambiguate.go`: incoming edges SQL（2 箇所）と scan 側フィルタを `pathLinkTypeSQLList` / `isPathLinkType` に統一
- `internal/core/build.go`: `pr.Links` バリデーションループ（line 87）を `!isPathLinkType` に統一（line 258 の `wikilink|frontmatter_wikilink` path scope は別概念のため据え置き）
- `internal/core/simplify.go` / `repair.go`: 本文専用フィルタを手書きリテラル + 「frontmatter_wikilink を意図的に除外」コメント追加で残置（書き換えコマンドではないため `isPathLinkType` に合流させない）
- 新規テスト 6 件:
  - `TestAdd_FrontmatterWikilinkEscapesVault`（vault escape ガード）
  - `TestAddAutoDisambiguateFrontmatterWikilink`（Pattern A 自動 disambiguate + DB raw_link 検証）
  - `TestUpdate_FrontmatterWikilinkEscapesVault`（Update 検証ガード）
  - `TestMove_FrontmatterWikilink_BasenameChange`（quoted/bare/subpath 維持 + DB raw_link 検証）
  - `TestMove_FrontmatterWikilink_AliasPreserved`（alias 維持）
  - `TestDisambiguateScan_FrontmatterWikilink`（scan モードでの quoted/bare 維持）

設計判断（goal-decisions.md 記録済み）:
- `pathLinkTypeSQLList` / `isPathLinkType` を `rewrite.go` に集約（SSoT 化）
- `simplify.go` / `repair.go` は frontmatter wikilink を意図的に除外、`isBodyPathLinkType` 述語化は backlog Later に派生
- 既存 `add_test.go` の linkType フィルタは据え置き、別タスク化

派生タスク（backlog Later 追記済み）:
- `simplify.go` / `repair.go` の手書き 2 型フィルタを述語化（`linkType` named type 昇格と同期想定）
- `add_test.go` の既存ヘルパー検証で `frontmatter_wikilink` を含めるよう拡張

レビュー: `/review-code-all` 系で 2 周完了
- 1 周目: facts/design/mdhop で SHOULD 指摘 → build.go 統一 / simplify-repair コメント追加 / `lookupEdgeTargetPath` コメント更新 / DB raw_link 検証追加 / `[[../escape]]` リライトで対応
- 2 周目: mdhop は LGTM。facts/design に SHOULD 残存（既存テスト linkType フィルタ + simplify/repair 述語化）→ いずれも今回スコープ外として backlog Later 化
- go は 1 周目で LGTM（fmt.Sprintf 定数埋め込みは安全と確認）

## Skipped tasks

なし

## Last verification

- `go test ./...`: PASS（既存 + 新規 6 ケース）
- `go vet ./...`: PASS
- `go build ./...`: PASS
- 手動 CLI sanity check: `mdhop build` → `mdhop move -from B.md -to NewB.md` で A.md frontmatter の quoted/bare/subpath 全形式が正しく書き換わるのを確認

## Next hint

次ループは A パスで残る v0.7.0 タスク「サンプルスキル更新（`examples/skills/` 配下を最新仕様に合わせる。リリース直前に実施）」に着手。これが最後の v0.7.0 タスクで、完了すれば goal-done.md を作成して全体ゴール達成。

着手時の手順:
1. `examples/skills/` 配下の現状を確認（`mdhop`, `mdhop-query`, `mdhop-workflow` の 3 サブディレクトリ）
2. v0.7.0 で追加された機能（frontmatter wikilink 対応）に応じてサンプル文言や例を更新
3. リリース直前タスクの位置付け通り、必要なら release_flow.md と整合
