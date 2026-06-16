# ADR 0017: search の computed fields（lines / edge counts）

## Status

Accepted

## Context

v0.11.0 で「巨大すぎるページ」「リンク過疎な葉ページ」「backlinks 過密な hub」を機械的に検出するため、`search` に computed field（`lines` / `outgoing_count` / `incoming_count`）と `--fields meta.<key>` 出力を追加する。

これらの値をどう保持するかで、index スキーマと build/add/update の責務に影響が出る。`--sort -lines` のように全件ソートに使う前提があるため、実行時のファイル全読みは index キャッシュという mdhop の存在意義（grep に頼らず事前解析）と矛盾する。一方で edge count は edges テーブルから常に導出できる派生値である。

mdhop の DB は「変更時は DB を再生成する前提」（`docs/rules/03-data-model.md`）で、migration レイヤを持たない。スキーマ列追加は `mdhop build` の全再生成で吸収される。

## Considered Options

- **A. lines・counts をすべて nodes に永続列追加**: build 時に全部計算。sort は速いが、edge count を edges と nodes の二重管理にする。add/update/move のたびに同期が必要。
- **B. lines・counts をすべて実行時計算**: スキーマ変更なし。lines は search のたびに全件ファイル I/O が走り、index キャッシュの意味を失う。
- **C. lines は nodes に永続化、edge counts は edges から実行時 SQL 集計**: lines はファイル内容の事実（edges に存在しない情報）として build/add/update で確定保存。counts は edges（真実の源）からの派生として query 時に集計。

## Decision

We will adopt option C.

- `nodes.lines` 列を追加し、build / add / update でファイル全体の行数（frontmatter 含む）を保存する。move は内容を変えないため lines を触らない。asset / phantom は行数概念を持たないため NULL。
- `outgoing_count` / `incoming_count` は `search` の main クエリ内で edges を `COUNT` するサブクエリとして算出し、永続化しない。`--sort` でも同じ式を ORDER BY に使う。
- `--fields` は opt-in を維持する。`meta`（全 key）/ `meta.<key>`（特定 key）/ computed field を指定したときだけ追加出力する。

lines と counts の保持方法が非対称なのは、両者の意味境界が異なるため: lines は「ファイル内容の事実」で edges から導出できず、counts は「edges の派生」で常に edges と一致させたい。同じ事実を 2 箇所に持たないことを優先する。

## Consequences

- edge count が edges と食い違うことがない（build/add/update/move での同期コードが不要）。
- `nodes.lines` 列の追加により、過去の DB は次回 `mdhop build` で再生成が必要になる（migration なし、既存方針通り）。
- `--sort -outgoing_count` 等は相関サブクエリを全ノードに対して評価するため、超大規模 vault では meta key sort より重くなりうる。現状の規模（数千ノード）では問題にならない。必要なら将来 counts を集計テーブル化する余地は残る。
- `--fields meta.<key>` 記法の導入で、frontmatter の特定 key だけを平坦に取り出せる。全 meta 出力（`meta`）とは別フィールドとして共存する。
