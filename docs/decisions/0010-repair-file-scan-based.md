# ADR 0010: repair をファイル走査ベースに変更

## Status

Accepted

## Context

repair コマンドは壊れたパスリンクを basename リンクに書き換えるツールだが、従来は build 後の DB（nodes/edges テーブル）を前提としていた。しかし repair の主な用途は「build できない状態を修復する」ことであり、特に vault-escape リンク（build エラーの原因）が存在する場合に DB がないと repair が使えないという矛盾があった。

disambiguate コマンドは既に `--scan` フラグでファイル走査ベースの動作をサポートしており、DB なしでリンク書き換えを行うパターンが確立されていた。

## Considered Options

- **A: DB ベースを維持し、vault-escape のみ別途処理**: 既存アーキテクチャを保持できるが、build エラー時に repair が使えない問題は解決しない。2つの修復パスの保守コストが増える。
- **B: ファイル走査ベースに全面移行**: DB 依存を完全に除去し、DisambiguateScan と同じパターンで動作させる。build 前に実行可能になる。DB 更新（edge/mtime）は不要になる。
- **C: DB ベース + ファイル走査ベースの両モード**: `--scan` フラグで切り替え。柔軟だが保守コストが高い。repair で DB モードが必要なケースが見当たらない。

## Decision

We will adopt option B: repair をファイル走査ベースに全面移行する。DB 存在チェック、DB オープン、edge クエリ、stale チェック、DB トランザクションをすべて削除し、collectMarkdownFiles + parseLinks で直接ファイルを走査する。

## Consequences

- repair が build 前に実行可能になり、vault-escape リンクの修復で build → repair の順序依存が解消される
- DB 更新（edge raw_link、source mtime）が不要になり、実装が単純化される
- stale チェックが不要になる（DB の mtime と比較する必要がなくなった）
- repair 後は build を実行してインデックスを作成・更新する必要がある（従来と同じ）
- 大規模 vault では全ファイルの読み込みが必要になるが、build/disambiguate --scan と同等のコストであり許容範囲内
