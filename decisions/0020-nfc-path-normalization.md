# ADR 0020: vault path を NFC へ正規化する

## Status

Accepted

## Context

APFS では Unicode 正規化の違う表記が同一ファイルに当たることがある。NFD の実ファイル名と NFC の参照値が混在すると、mdhop の index では path / node_key が文字列一致しないため、同一ファイルの二重登録、`meta-check` の偽 `not_found`、`delete` 不能が起きる。

Go 標準ライブラリには Unicode 正規化 API がない。独自実装は不完全になりやすく、path 解決・DB key・glob 比較の基盤として使うには危険が大きい。

## Considered Options

- **A: vault-relative path を NFC へ正規化し、`golang.org/x/text/unicode/norm` を使う**: Unicode 正規化を標準的なライブラリへ委ね、DB 保存値・比較 key・CLI 入力を同じ表現へ寄せる。
- **B: path 比較時だけ NFC/NFD の両方を試す**: 保存値は維持できるが、SQL 条件・resolve map・rewrite 系に二重の比較経路が増え、抜け漏れが起きやすい。
- **C: 依存を増やさず、ASCII / NFC-only vault のまま運用制約にする**: 既存の実 vault で発生したデータ整合性問題を解消できない。

## Decision

We will adopt Option A. `NormalizePath` を NFC 正規化の入口にし、DB node key / stored path / resolve map / path filter pattern / basename key を NFC 表現へ揃える。既存 index に NFD path が残っている場合も、incremental コマンドが DB から resolve map を作る時点で NFC に寄せて比較する。完全な永続移行は `build` による再生成で行う。

`golang.org/x/text/unicode/norm` は Unicode 正規化そのものを担う小さな依存であり、mdhop 固有の path 意味は `internal/core` 側に残す。新 package は作らず、既存の vault path helper に閉じ込める。

## Consequences

- 肯定的: NFD 実ファイル名と NFC 参照が混在しても、index 上は NFC path として 1 ノードに寄る。
- 肯定的: `resolve` / `meta-check` / `add` / `update` / `delete` / `move` の path 比較が同じ正規化前提になる。
- 否定的: `internal/core` の外部依存に `golang.org/x/text` が増える。
- 中立的: 既存 DB の NFD 保存値はマイグレーションせず、`build` 再実行で NFC 保存値へ移行する。
