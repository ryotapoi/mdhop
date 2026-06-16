# ADR 0002: resolve のリンク解決に DB クエリを使用する

## Status

Accepted

## Context

`build` コマンドはリンク解決時にインメモリマップ（`pathSet`, `basenameToPath`, `pathToID`）を使用する。`resolve` コマンドは build 済みの DB からリンクを解決する必要がある。

リンク解決ロジック（self-link → tag → relative → absolute → vault-relative → basename → markdown path）は build と resolve で同一だが、データソースが異なる（インメモリ vs DB）。

## Considered Options

- **A: DB クエリベースの別関数 `resolveLinkFromDB()`**: build の `resolveLink()` と同じ解決順序を DB クエリで再実装する。ロジックの重複が発生するが、build と resolve が独立して動作する
- **B: 共通インターフェースで抽象化**: データソースを interface で抽象化し、build はインメモリ実装、resolve は DB 実装を注入する。DRY だが、抽象化コストが高く、現時点では2つの実装しかない
- **C: resolve 時にも全ファイルを読み込んでインメモリマップを構築**: build のコードをそのまま再利用できるが、resolve は単一リンクの解決なのに全ファイルを読む必要があり非効率

## Decision

We will use option A: DB クエリベースの別関数 `resolveLinkFromDB()` を実装する。

解決順序は `build.go` の `resolveLink()` と同一に保ち、各ステップで DB クエリを使用する。

## Consequences

- **肯定的**: resolve は DB のみに依存し、ファイルシステムの読み込みが不要。単一リンクの解決が高速
- **肯定的**: build と resolve が独立しており、一方の変更が他方に波及しにくい
- **否定的**: リンク解決ロジックが2箇所に存在する。解決順序の変更時は両方を更新する必要がある
- **中立的**: 将来 interface 抽象化が必要になった場合、この2つの実装を元にリファクタリングできる
