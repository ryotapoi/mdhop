# Knowledge Base

このファイルはプロジェクト固有の技術的な知見・ハマりどころを蓄積する場所。

## このファイルの使い方

- **いつ読むか**: 新しい機能の実装前、バグ調査時に関連セクションを確認する
- **何を書くか**: 特定の状況で役立つ知見（罠、回避策、仕様の癖など）
- **CLAUDE.md との違い**: CLAUDE.md は毎回読み込まれる「常に守るルール」。ここは「該当する実装のときだけ必要な知見」
- **書き方**: セクション見出しでテーマ分け。各項目は簡潔に。症状・原因・対処がわかるように書く

---

## modernc.org/sqlite

- `LastInsertId()` は `ON CONFLICT DO NOTHING` 時に 0 ではなく直前の INSERT の rowid を返す。`RowsAffected()` で実際に挿入されたか確認してから `LastInsertId()` を使うこと。同一タグが複数ファイルから参照される場合などで不正な target_id を持つエッジが作られる
- `ON CONFLICT ... DO UPDATE` 時は `LastInsertId()` が 0 を返す。その場合は SELECT で ID を取得する
- `openDBAt(path + "?mode=ro")` はクエリパラメータをファイル名の一部として扱うため、read-only オープンには使えない

## リンクパーサー (parse.go)

- `parseMarkdownLinks` で `[[` を `[` と誤認しないよう、`[` の次が `[` ならスキップする処理が必要
- frontmatter の行番号: yaml.v3 の `Node.Line`（YAML 内 1-based）+ offset 1（`---` 行分）= ファイル全体の行番号
- Tag parser はブラックリスト方式。ハイフン・アンダースコア・Unicode 文字は許可。先頭数字は Obsidian 準拠で不可
- **frontmatter タグと inline タグの文字種差異**: inline は Unicode 対応済みだが、frontmatter は YAML からそのまま取り込むためブラックリスト対象の句読点も含み得る。この差異は実用上問題にならない
- `isBasenameRawLink` は self-link（`[[#Heading]]`, `[text](#heading)`）で false を返す必要がある。fragment 除去後に target が空なら self-link

## ルート優先ルール

- `pathSet` のキー構造が前提: ルート `A.md` は `"a"` キー、サブディレクトリ `sub/A.md` は `"sub/a"` キー。`pathSet["a"]` はルートファイル専用のため `hasRootInPathSet` が機能する
- move の Phase 2（incoming rewrite）が先に basename リンクを rewrite するため、Phase 2.5 ではそのエッジは処理済み。テストで「Phase 2.5 でエラー」を期待するなら、incoming edge を持たない第三者ファイルのリンクを使う
- update でファイル削除すると `basenameCounts` が減る。ルート削除後に残りが 1 つなら basename 一意→非曖昧

## リライト (rewrite.go)

- `buildRewritePath` は vault-relative パスを返す（ルートもサブディレクトリも統一）。発リンクの相対パスリライトには `filepath.Rel` ベースの `rewriteOutgoingRelativeLink` を別途使う
- `applyFileRewrites()` の `sourceID=0` 固定は `newMtimes` を無視する前提でのみ安全。mtime を使う拡張時は要注意
- `applyFileRewrites` はバックアップを返す設計にし、呼び出し元がリライト失敗時に復元できるようにする
- `os.WriteFile` は新規作成時に umask でパーミッションがマスクされる。既存ファイルの上書きでもファイルが削除→再作成される可能性があるため、パーミッションを保持するには `os.WriteFile` 後に `os.Chmod` を併用する（`writeFilePreservePerm`）
- 曖昧リンク判定で `basenameKey(link.target)` を使うと `.md` 以外の拡張子も削られる。`strings.ToLower(link.target)` を使う（update.go と一貫性を保つ）

## パス操作

- `filepath.Rel(dir, ".")` は `"../."` を返す（`".."` ではない）。`filepath.Clean` を適用すること

## move コマンド

- 移動ファイルが着リンクの書き換え対象でもある場合（自身への参照）、着リンク収集時に除外して発リンクフェーズでまとめて処理する
- stale チェック: `os.Rename` は mtime を保持するので、移動先の mtime と DB 上の移動元の mtime を比較する
- 外部ファイル（incoming/collateral rewrite 対象）の stale チェックは不要。`replaceOutsideInlineCode` は文字列マッチで安全に動作し、Obsidian 等による mtime 変更で連続 move が失敗する問題を回避する
- 既知の制限: move 中に外部ツール（Obsidian 等）が書き換え対象ファイルの**内容**を変更すると、行ズレにより置換が空振りするが DB の edge は更新される（DB とディスクの不整合）。次の `build` で回復する
- コラテラル書き換え: Phase 2.5 の SQL で `tn.path` を取得する際、phantom ノード（path が NULL）を JOIN 条件で除外する（`tn.type = 'note' AND tn.exists_flag = 1`）
- outgoing basename の DB クエリ: `COALESCE(tn.path, '')` で phantom ノードの NULL を空文字列に変換。`preMoveTargetPath == ""` のケース（phantom）は書き換え不要
- `allExternalRewrites := append(incomingRewrites, collateralRewrites...)` はスライスエイリアシングの危険があるため、明示的に新スライスを作って append する

## MoveDir（ディレクトリ move）

- ディレクトリ move では basename が変わらない（パスのディレクトリ部分のみ変更）。したがって collateral rewrite（basename 曖昧化による第三者リンク書き換え）は発生しない。Phase 2.5 のコードは存在するが、`affectedBasenames` のカウントが変動しないためスキップされる
- `rewriteOutgoingRelativeLinkBatch` は移動セット内ファイル間の相対リンクを扱う。`movedFromTo` マップのキーは `.md` 付きパスだが、wikilink の resolve 結果は `.md` なし。両方で lookup する必要がある

## update コマンド

- 同時 update+delete: ファイル A が B を参照、両方更新で B がディスク削除 → A の `resolveLink` が phantom B を作成 → B は incoming edge が 0（edge は phantom B を指す）→ note B は完全削除される（phantom 変換ではない）。これは正しい挙動
- `basenameCounts` 調整は `pathToID` の存在チェックが必要。`exists_flag=0` のノートなど maps にないファイルを decrement しない

## add コマンド

- Phantom promotion（step 13）は note insertion（step 12）の後に実行する。`pathToID` に新ノート ID が入っていないと edge reassignment が失敗する
- 既存曖昧チェック（step 8）にはパターン A（oldCount==1 → newCount>1）と B（oldCount==0、同 basename ファイル 2+ 追加で既存 phantom link あり）の両方が必要

## テスト

- `vault_build_conflict` は `[[A]]` を含むため build がエラーになる。basename 衝突のみ（曖昧リンクなし）をテストするには専用フィクスチャを作る
- build はファイル存在時に phantom を自動解決するため、既存 phantom edge のテストには DB に直接 phantom ノードを挿入する
- mtime stale テスト: `os.WriteFile` は同一秒内だと同じ Unix mtime を返す。DB の mtime を直接書き換えてテストする
- disambiguate テスト: build が曖昧 basename link を拒否するため、フィクスチャではパスリンク（`[[sub/A]]`）を使い、テスト内でルート `A.md` を追加して複数候補状態を作る

## query コマンド (query.go)

- `*sql.DB` → `dbExecer` 変更は安全。query.go の内部関数は `Exec`, `QueryRow`, `Query` しか使わず、`dbExecer` インターフェースがこれらを全てカバーしている。公開関数 `Query()` は `*sql.DB` のまま維持（`openDBAt` の戻り値型）
- 除外フィルタの SQL WHERE で `n.path` が NULL（phantom/tag）のノードを誤除外しないよう `n.path IS NULL OR NOT (n.path GLOB ?)` が必要。`NOT (NULL GLOB ?)` は NULL を返し WHERE で偽になるため
- タグの `name` 列は `upsertTag` で元の大文字小文字のまま保存される（`node_key` のみ小文字化）。タグの比較クエリでは `LOWER(n.name)` が必要

## CLI 出力 (format.go)

- `writeNodeInfoText` では firstIndent と restIndent を分離する（リスト項目 `- type: note` vs 継続行 `  name: ...`）
- JSON の `Exists` フィールドは `*bool` + `omitempty` にしないと、false が JSON から落ちる
- JSON output で `map[string]any` を使うとリクエストされたフィールドが空でも常に出力される（struct + `omitempty` だと空配列が消える）
- フィールドバリデーションは DB オープンの前に配置する。DB がない状態で unknown field エラーを返すため

## asset 管理

- note と asset の basename キー空間は完全に分離: `basenameKey` は拡張子除去、`assetBasenameKey` は拡張子込み。クロスチェック不要
- `diskOnlyFiles`（MoveDir）や delete `--rm` のディスク Walk では `.md` ファイルをスキップする（D5: disk-based operation は非 .md のみ）
- `findEntryByFile` の note → asset フォールバックは `sql.ErrNoRows` のみで発動。DB エラーを握りつぶさない
- outgoing rewrite で asset maps を参照する必要はない。note 移動では asset 解決先は変わらず、asset 移動時は `isAsset=true` で outgoing rewrite 自体がスキップされる
