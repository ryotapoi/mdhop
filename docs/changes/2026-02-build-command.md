# build コマンド: Vault 解析と SQLite インデックス生成の実装

## Touches
- `go.mod` / `go.sum` — modernc.org/sqlite, gopkg.in/yaml.v3
- `internal/core/parse.go` — リンクパーサー（wikilink, markdown link, tags, frontmatter tags）。linkOccur 構造体、行番号追跡、subpath 抽出、コードフェンス・インラインコード除外
- `internal/core/parse_test.go` — 23 テスト
- `internal/core/build.go` — Build() エントリポイント。ファイル収集、曖昧リンク検出、2パス構成（notes upsert → link resolution + edges）、vault escape 検出
- `internal/core/build_test.go` — 22 テスト + ヘルパー（queryEdges, queryNodes, countEdges）
- `internal/core/db.go` — SQLite スキーマ（nodes + edges）、upsertNote, upsertPhantom, upsertTag, insertEdge, getNodeID
- `internal/core/util.go` — normalizePath, basename, basenameKey
- `internal/testutil/copy.go` — テスト用 CopyDir ヘルパー
- `testdata/` — 11 vault フィクスチャ（basic, empty, conflict, existing_db, case_insensitive, case_ambiguous, edges, relative, phantom, tags, tag_codefence, full）

## Verification
- `go test ./...` — 45 テスト全パス
- `go vet ./...` — 警告なし

## Pitfalls
- `upsertNote` の `LastInsertId()` は ON CONFLICT（UPDATE）時に 0 を返すため、その場合は SELECT で ID を取得する必要がある
- frontmatter の行番号は yaml.v3 の `Node.Line`（YAML 内 1-based）+ offset 1（`---` 行分）= ファイル全体の行番号
- `parseMarkdownLinks` で `[[` を `[` と誤認しないよう、`[` の次が `[` ならスキップする処理を追加
