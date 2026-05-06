# Goal State

status: running

## Current scope

`backlog/backlog.md` の v0.7.0 の全タスク完了。

## Last completed loop

2026-05-07 (B パス): frontmatter 内 wikilink 対応タスクを解析側 / 書き換え側に 2 分割。
- backlog v0.7.0「frontmatter 内 wikilink 対応」を以下に分解:
  - (1/2) 解析と edge 化（parse.go / build.go / resolve.go、新 linkType `frontmatter_wikilink` 追加、5 コマンド書き換えは触らない）
  - (2/2) 書き換えコマンド対応（add/update/move/disambiguate/convert、行ベース置換、quoted/bare style 維持）
- 設計判断は (1/2) 内で確定する（新 linkType / bare 検出戦略 / meta 両立 / alias 流用）
- 詳細は backlog エントリ参照
- このループは整理のみ（実装なし、A パスは次ループで実施）

## Skipped tasks

なし

## Last verification

検証なし（ドキュメント編集のみ）。

## Next hint

次ループは A パスで「frontmatter 内 wikilink 対応 (1/2): 解析と edge 化」に着手。

着手時の手順:
1. `internal/core/parse.go` の `parseFrontmatter` / `collectMeta` / `parseFrontmatterTags` の現状を再読
2. `gopkg.in/yaml.v3` の `Node.Line` / `Node.Column` を使い、val の行範囲を取得して生テキストから `[[...]]` を本文用正規表現で抽出する方針を確定
3. `linkOccur` の `linkType` に `"frontmatter_wikilink"` を追加し、build.go の edge 化、resolve.go の解決経路に流す
4. alias / subpath は本文 wikilink と同等（`splitAlias` / `splitSubpath` 流用）
5. `rules/03-data-model.md:126` の `link_type` 列挙に `frontmatter_wikilink` を追記
6. テスト: `testutil` フィクスチャに frontmatter wikilink を含む note を追加し、build → query で edge が辿れることを確認
7. 既存書き換え系コマンドは新 linkType を「未対応」のまま放置（次ループで対応）
8. レビュー: 解析側変更は parse/build/resolve に跨るため `/review-code-all` 並列レビュー想定
