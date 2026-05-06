# Backlog

## v0.6.1

- [x] ノード型文字列リテラル（`"note"` `"asset"` `"phantom"` `"tag"`）を定数化
  - **問題**: 内部コア全体に 35 箇所以上散在（`db.go` `move_dir.go` `resolve.go` `query.go` 等）。SQL の WHERE 句と Go の switch 文で型安全性なく重複しており、タイポしてもコンパイル時に検出できない暗黙結合
  - **対応**: `db.go` に `NodeTypeNote = "note"` 等 4 件の exported untyped string const を定義し、Go コード側のリテラル参照を全置換。SQL 内シングルクォート、test ファイル、linkType 用 `"tag"`/`"frontmatter"` は対象外（`backlog/goal-decisions.md` 2026-05-06 参照）
- [x] `cmd/mdhop/format.go`（838 行）をコマンド別に分割
  - **問題**: 13 コマンド分の `printXText` / `printXJSON` ペアが 1 ファイルに集約。コマンド追加のたびに肥大化が確定している構造
  - **対応**: `format_query.go` `format_stats.go` ... のようにコマンド別分離。共通の `encodeJSON` `printStringListText` `parseFields` `validateFormat` `validateFields` `fieldSet` は `format.go` に残す
- [ ] `internal/core/query.go`（797 行）をエントリ解決とデータフェッチに分離
  - **問題**: `Query` エントリポイント、エントリ解決（`findEntryNode` `findEntryByKey` 等 6 関数）、データフェッチ（`queryBacklinks` `queryOutgoing` `queryTwoHop` `fetchNodeInfoBatch`）、ファイル読み取り（`readHead` `readSnippets` `readFileLines`）が同居
  - **対応**: `query_entry.go`（エントリ解決）と `query_fetch.go`（データフェッチ＋ファイル読み取り）に分割。`query.go` はエントリポイントと型定義のみに縮小

## v0.7.0

- [ ] `--where` NOT EXISTS 演算子（特定キーを持たないノートの検索。現状 EXISTS の逆がない）
- [ ] サンプルスキル更新（`examples/skills/` 配下を最新仕様に合わせる）
- [ ] frontmatter 内 wikilink 対応
  - **目的**: frontmatter で `key: "[[note]]"` / `key: [[note]]` / 配列形式が実際に使われている。これらを本文 wikilink と同等に解析し、edge 化して `mdhop query` で辿れるようにする。`""` の有無は Obsidian 都合（囲わないと wikilink 認識されない）なので、mdhop はどちらでも認識する
  - **現状（2026-05-04 時点）**:
    - `internal/core/parse.go:322` の `parseFrontmatter` は `tags` キーだけ特別扱いし、それ以外は `collectMeta` (`parse.go:418`) で文字列値として meta テーブルに格納するのみ。wikilink 記法は無視されている（edge 生成なし）
    - 既存 `linkType` は `wikilink | markdown | tag | frontmatter`（`frontmatter` は frontmatter tags 用、`rules/03-data-model.md:126`）
    - 書き換え系（`add.go` `update.go` `move_helpers.go` `disambiguate.go` `convert.go`）はほぼ全て `linkType == "wikilink" | "markdown"` で分岐し、`rewriteRawLink` (`rewrite.go:36`) を共通利用
  - **YAML パース調査結果**（`gopkg.in/yaml.v3` で実測、検証コードは捨てた）:
    - `key: "[[note]]"` → Scalar `"[[note]]"` （普通の文字列、想定どおり）
    - `key: [[note]]` → **ネストした flow sequence**として解釈される（`[[note]]` = `[ [note] ]`、要素1個の seq の seq）。エラーにならず構造が崩れる
    - `key:\n  - "[[a]]"` → block seq of scalar string ✅
    - `key:\n  - [[a]]` → block seq の中で各要素が flow seq of flow seq になる
    - `[[a|alias]]` `[[a#h]]` の `|` `#` は scalar 値の一部として保持される（YAML コメント扱いされない）
  - **設計論点**:
    - 新 linkType（例: `frontmatter_wikilink`）を追加するか、既存 `wikilink` を流用してロケーションを別カラム or rawLink から判別するか。書き換え時に YAML 文字列の `""` 維持が必要なため type 分けが安全
    - bare `[[note]]` の検出は YAML 後段で walk し「flow style の 1要素ネスト seq・全要素 scalar」のパターンを bare wikilink として再解釈する。`key: [a, b]` のような正当な flow seq との誤検出に注意
    - 書き換えは YAML 再シリアライズだと quoted/bare の style が変わる恐れ。行ベース文字列置換（`replaceOutsideInlineCode` 系）に倒すのが無難。yaml.v3 の `Node.Line` で行範囲は取れる
    - meta テーブルとの両立: frontmatter wikilink を edge と meta の両方に書くか edge のみか（`--where key=value` での絞り込みニーズと整合させる）
    - alias/subpath（`[[a|alias]]` `[[a#h]]`）は本文 wikilink と同様にサポート
  - **影響範囲**: 書き換え系コマンド（add/update/move/disambiguate/convert）への横断対応が必須。詳細は計画フェーズで詰める

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] `internal/core/move_dir.go` の `MoveDir`（758 行 1 関数）を分解
  - **問題**: ファイル列挙・DB 読み込み・リンク書き換え計画・ファイルリネーム・DB 更新・ロールバックが単一関数に同居。「実行段階」と「エラー時復元段階」が混在し、ロールバックロジックが散在（585, 607, 616 行）
  - **対応**: move_helpers.go 側に段階別ヘルパーを追加し、MoveDir はオーケストレーションのみに縮小。ロールバックは defer + 状態フラグで集約
  - **注意**: 挙動変更リスクが高い。テスト充実（特にロールバックパス）を先行させる
- [ ] `internal/core/init_meta.go`（578 行）の責務分離
  - **問題**: YAML 生成・型推論・既存設定マージの 3 責務が同居。新メタ型追加のたびに肥大化するリスク
  - **対応**: `init_meta_yaml.go`（YAML 生成）/ `init_meta_infer.go`（型推論）/ `init_meta.go`（マージとエントリポイント）に分割
- [ ] `internal/core` の sentinel error 化
  - **問題**: `fmt.Errorf` 94 箇所のうち `%w` でのラップは 8 箇所のみ。`sql.ErrNoRows` 比較は `==` を 13 箇所で使っており `errors.Is` 不在。エラー識別を文字列マッチに頼る箇所が壊れやすい
  - **対応**: `errIndexNotFound` 等の sentinel を定義し、呼び出し元は `errors.Is` で分岐。既存 `fmt.Errorf` を `%w` ラップに置換
