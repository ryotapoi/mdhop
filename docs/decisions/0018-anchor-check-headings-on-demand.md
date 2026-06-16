# ADR 0018: anchor 検査の heading を実行時抽出する

## Status

Accepted

## Context

v0.11.0 で `[[note#見出し]]` / `[text](note.md#fragment)` の fragment が target note の見出しに実在するかを検査する anchor 切れ検出（`diagnose --fields anchors`）を追加する。

リンクの fragment は既に edges テーブルの `subpath` に保存されている。不足しているのは「各 note の heading 一覧」だけである。これをどう保持するかで、index スキーマと build/add/update/move の責務に影響が出る。

直前の ADR 0017 では `search` の `lines` を nodes に永続化した。理由は `--sort -lines` が全件常時ソートを要求し、実行時の全件ファイル読みが index キャッシュの意味を壊すためだった。anchor 検査はこれとは頻度・対象が異なる: `diagnose --fields anchors` という opt-in 診断で、検査対象は「fragment を持つリンクの target note」に限られる。

Obsidian の anchor 正規化は句読点／記号除去＋空白畳み込みのみ（大小文字・アクセント保持、重複見出しの連番なし）で、kebab-case のような重い変換は不要であることを確認した。

## Considered Options

- **A. headings 専用テーブルを永続化**: build/add/update で各 note の正規化済み heading を保存。検査は SQL だけで済むが、build/add/update/move 全てに heading 同期コードが増え、その維持コストを opt-in 診断のためだけに常時払う。
- **B. 検査実行時に target note をディスクから読んで heading 抽出**: スキーマ変更なし。検査対象の target note だけを読み、heading セットを target path 単位でキャッシュする。

## Decision

We will adopt option B.

- anchor 検査時にのみ、fragment リンクの target note をディスクから読み、`collectHeadings` で heading を抽出して正規化セットを作る。同じ target path は 1 回だけ読みキャッシュする。
- heading 抽出は commit で切り出した本文走査骨格 `walkBodyLines` に載せる（frontmatter / fenced code を共通規則でスキップ）。
- anchor 検査は `--fields anchors` 明示時のみ実行する。`--fields` 未指定（全フィールド表示）でも anchors は走らせない。

lines（常時ソート対象 → 永続化）と heading（opt-in 診断 → 実行時抽出）で保持方法が異なるのは、使用頻度と対象範囲が異なるため。現在の要求（時々 anchor を点検したい）に対し、全 mutation への heading 同期コードは過剰。

## Consequences

- index スキーマを変更しないため、既存 DB の rebuild は不要。
- build/add/update/move に heading 同期コードを増やさない。
- 大量の fragment リンクがある vault では検査時に target note を読む I/O が発生するが、target path 単位でキャッシュするため同一 note は 1 回しか読まない。opt-in なので通常の diagnose 速度には影響しない。
- 正規化は Obsidian 互換（句読点／記号除去・空白畳み込み・大小／アクセント保持）。block reference（`#^id`）と setext heading は対象外（ATX heading のみ）。target が phantom / asset のリンクは anchor 検査の対象外（note 自体の不在は phantom 検出の領分）。
