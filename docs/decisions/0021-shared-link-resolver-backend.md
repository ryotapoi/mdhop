# ADR 0021: リンク解決を共通 backend interface へ統合する

## Status

Accepted

## Context

ADR 0002 は、build（インメモリマップ）と resolve（DB クエリ）でリンク解決のデータソースが異なる問題に対し、次の 3 案を検討していた。

- **A: DB クエリベースの別関数 `resolveLinkFromDB()`**: build の `resolveLink()` と同じ解決順序を DB クエリで再実装する。ロジックの重複が発生するが、build と resolve が独立して動作する
- **B: 共通インターフェースで抽象化**: データソースを interface で抽象化し、build はインメモリ実装、resolve は DB 実装を注入する。DRY だが、抽象化コストが高く、現時点では 2 つの実装しかない
- **C: resolve 時にも全ファイルを読み込んでインメモリマップを構築**: build のコードをそのまま再利用できるが、resolve は単一リンクの解決なのに全ファイルを読む必要があり非効率

ADR 0002 は **A を採用**した。当時の判断根拠は「実装が 2 つしかない段階で interface 抽象を入れるのは早すぎる」であり、重複の代償は Consequences に「否定的: リンク解決ロジックが 2 箇所に存在する。解決順序の変更時は両方を更新する必要がある」と明示して受け入れていた。

ADR 0015（reachable の走査設計、v0.10.0）でこの判断は一度再評価され、「reachable はリンク解決を一切行わず build 済みの edges を走査するだけなので、第三の実装を加えない。interface 抽象への移行は引き続き見送る」として A の維持が確認されている。

その後、前提が 2 点変わった。

1. **実装が 3 つになった**。dry-run 系の解決（実在 note / asset のみを解決し、phantom / tag ノードを作らない）が必要になり、`dryLinkResolver` が加わった。ADR 0002 が「2 つしかない」ことを抽象化見送りの根拠にしていた条件が崩れた。
2. **解決順序の同期が実害になった**。解決順序（self-link → tag → relative → absolute → vault-relative → basename → markdown path）に加え、vault escape 検査や `frontmatter_wikilink` を wikilink と同じ経路に載せる分岐など、順序ロジック自体が育った。ADR 0002 が「変更時は両方を更新する必要がある」と予告していたコストが、実装数に比例して増えた。

## Considered Options

- **A: ADR 0002 の構図を維持し、3 つ目も独立実装として書く**: 各実装の独立性は保たれるが、解決順序の変更が 3 箇所への同期を要求する。順序ロジックが育った現在、同期漏れが静かな誤解決を生むリスクが最も高い
- **B: 共通 dispatcher + backend interface へ統合**: 解決順序の分岐を 1 関数に集約し、backend はストレージ固有の lookup（self / tag / path / basename）だけを実装する。ADR 0002 が却下した Option B に相当する
- **C: dry-run 実装を既存のどちらかに寄せる**: 実装数を 2 に留められるが、dry-run は phantom / tag を作らない点で build とも resolve とも意味が異なり、フラグ分岐で潰すと各実装の可読性が落ちる

## Decision

We will adopt option B: 解決順序の dispatch を `resolveLinkWithBackend()` に集約し、ストレージ固有の lookup を `linkResolverBackend` interface に切り出す。

- `linkResolverBackend` は `resolveSelf` / `resolveTag` / `resolvePath` / `resolveBasename` の 4 メソッドを持つ
- backend 実装は `mapLinkResolver`（build、インメモリマップ）、`dbLinkResolver`（resolve、DB クエリ）、`dryLinkResolver`（dry-run、実在 path のみ・ephemeral ID）の 3 つ
- 解決順序・vault escape 検査・link type ごとの分岐は dispatcher が単独で所有し、backend は順序を知らない

ADR 0002 の Option A 採用は、これをもって現行実装と一致しなくなる。ADR 0002 の Consequences が「将来 interface 抽象化が必要になった場合、この 2 つの実装を元にリファクタリングできる」と残していた出口を、実装数 3 の時点で通ったことになる。

## Consequences

- 肯定的: 解決順序の変更が 1 箇所で済む。ADR 0002 が受け入れていた「2 箇所（後に 3 箇所）の同期」コストが消える
- 肯定的: backend の追加が順序ロジックに触れずに済むため、4 つ目の解決文脈が必要になっても構図が変わらない
- 否定的: build / resolve が完全独立でなくなり、dispatcher の変更は全 backend に波及する。ADR 0002 が肯定的に評価していた「一方の変更が他方に波及しにくい」性質は失われた
- 中立的: 解決順序は dispatcher が単独で持つため、backend 実装だけを読んでも解決順序は分からない。順序を知りたい場合は `resolveLinkWithBackend()` を読む
