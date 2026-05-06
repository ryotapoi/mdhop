# Goal State

status: running

## Current scope

`backlog/backlog.md` の v0.7.0 の全タスク完了。

## Last completed loop

2026-05-07 (A パス): backlog v0.7.0「frontmatter 内 wikilink 対応 (1/2): 解析と edge 化」を完了。

実装内容:
- `internal/core/parse.go`: `parseFrontmatter` を拡張し、新ヘルパー `frontmatterEntryRange` / `parseFrontmatterWikilinks` を追加。frontmatter mapping の各 (key, val) について行範囲を計算し、生テキストに本文 wikilink 抽出を当てて `linkType="frontmatter_wikilink"` の `linkOccur` を生成
- `internal/core/build.go`: 検証ループ（line 87）と vault-relative path 判定（line 258）の linkType フィルタに `frontmatter_wikilink` を追加
- `internal/core/resolve.go`: vault-relative path 判定（line 129）の linkType フィルタに `frontmatter_wikilink` を追加
- `testdata/vault_build_frontmatter_wikilink/`: 新規 fixture（A.md / B.md / Sub/C.md / Sub/D.md）。quoted / bare / array / alias / subpath / phantom のバリエーション
- `internal/core/parse_test.go`: TestParseFrontmatterWikilink* 8 件を追加
- `internal/core/build_test.go`: TestBuildFrontmatterWikilink* 3 件を追加
- `rules/03-data-model.md` 3.1 link_type 列挙に `frontmatter_wikilink` を追記
- `rules/overview.md` 「リンク解釈（互換性）」セクションを更新

設計判断（goal-decisions.md 記録済み）:
- frontmatter wikilink を build 時の vault-escape / ambiguous 検証対象に含める（add.go / update.go の同型ガードは (2/2) で揃える）
- `parseFrontmatterWikilinks` のシグネチャは YAGNI で `fileLineOffset` を削除
- `linkType` 文字列リテラル直書きの増殖は別タスク化（backlog Later に named type 昇格を追加）

派生タスク（backlog 追記済み）:
- backlog Later: `linkType` を `type LinkType string` に named type 昇格
- backlog Later: `parseFrontmatter` の責務分離検討
- backlog v0.7.0 (2/2) タスク説明に「`add.go:249` / `update.go:136` の検証ガードも (2/2) で揃える」を追記

レビュー: `/review-plan-all` 系は overload で 2 周目スキップ（self-check で代替）。`/review-code-all` 系は 2 周完了（facts / design / mdhop は 2 周目で MUST/SHOULD ゼロ、go は 1 周目で LGTM）。

## Skipped tasks

なし

## Last verification

- `go test ./...`: PASS（既存 388+ + 新規 11 ケース）
- `go vet ./...`: PASS
- `go build ./...`: PASS
- 手動 CLI sanity check: `mdhop build` → `mdhop query --file B.md` で A.md が backlink として返るのを確認

## Next hint

次ループは A パスで「frontmatter 内 wikilink 対応 (2/2): 書き換えコマンド対応」に着手。

着手時の手順:
1. `internal/core/rewrite.go` の `rewriteRawLink` / `applyFileRewrites` の現状を再読
2. (2/2) backlog 説明に基づき、`add.go` / `update.go` / `move_helpers.go` / `disambiguate.go` / `convert.go` の `linkType` 分岐に `frontmatter_wikilink` を追加
3. `add.go:249` / `update.go:136` の検証ガードも `frontmatter_wikilink` を含むように拡張（(1/2) で意図的に保留した分）
4. 行ベース置換の実装: `val.Line` → `lineStart` を起点に `[[old]]` → `[[new]]` を生置換（`replaceOutsideInlineCode` 相当）。quoted/bare style 保持
5. テスト: 5 コマンド × バリエーション（rawLink 一意マッチ、alias 保持、subpath 保持、quoted 保持、bare 保持）
6. `convert` は `frontmatter_wikilink` ↔ markdown 変換しない方針を維持
