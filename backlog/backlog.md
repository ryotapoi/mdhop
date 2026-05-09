# Backlog

## v0.8.0

- [ ] `--where` NOT EXISTS 演算子（特定キーを持たないノートの検索）
  - **問題**: 現状 EXISTS の逆がない。マイグレーション後の取りこぼし検出（「`priority` がまだ付いてない既存ノート」など）やテンプレート逸脱の検出に必要
  - **対応**: `internal/core/where.go` の文法に NOT EXISTS を追加。`--where "priority NOT EXISTS"` のような形を想定
  - **想定ユースケース**: PKM 整理（未整理ノートの洗い出し）、メタキー追加後の取りこぼし検出
  - **影響範囲**: `where.go` のみ（query/search 側の呼び出し点は変更不要）
- [ ] PKM 整理向け孤立検出フラグ（`search` への 3 フラグ追加、まとめて 1 タスク）
  - **問題**: 「タグ一切なし」「outgoing link なし」「incoming link なし」のノートを抽出する手段がない。tags は edges テーブル経由なので `--where` の文法では届かない（meta テーブルにエントリを持たない）
  - **対応**: `mdhop search` に専用フラグを追加
    - `--no-tags` : タグ edge ゼロの note（最優先・PKM 整理で一番欲しい）
    - `--no-outgoing` : outgoing link ゼロの note
    - `--no-incoming` : backlink ゼロの note
  - **設計判断**: `--where` 拡張ではなく専用フラグにする。tags が meta テーブル外で `--where` の対象外なため、文法を歪める方が筋悪
  - **分割しない理由**: 3 フラグとも `internal/core/search.go`（222 行）と `search_test.go` で完結。SQL は `LEFT JOIN edges ... WHERE NULL` のヘルパーを共有できるため、分割すると共通部分の重複ロードが発生する
  - **影響範囲**: search コマンドのフラグ追加 + SQL クエリ拡張（`LEFT JOIN edges ... WHERE edges.id IS NULL` 系）+ fixture と 3 ケースのテスト追加

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] `parseFrontmatter` の責務分離検討
  - **問題**: 現状 `parseFrontmatter` は tags / meta / frontmatter_wikilink の 3 系統を返す。今後さらに追加する種別があると肥大化する
  - **対応**: 戻り値が 4 系統以上になるタイミングで `parseResult` 構造体ベースへ移行する判断
  - **由来**: design レビュー（v0.7.0 frontmatter 内 wikilink 対応 (1/2)）で予兆として記録
