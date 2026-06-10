# ADR 0016: graph export のスキーマと範囲

## Status

Accepted

## Context

v0.10.0 の subgraph export（`mdhop graph`）で、何を node / edge として出力するかを決める必要があった。
インデックスには note / asset / phantom / tag の 4 種の node と、tag 系を含む全 link 出現の edge が入っている。
一方、graph の用途は外部ツール（可視化、類似判定、クラスタリング）への入力であり、解釈はツール側の責務とする方針（backlog: 「類似判定等は呼び出し側」）。

## Considered Options

- **A: navigation link のみの誘導部分グラフ**: node は実在 note / asset（path glob でフィルタ）、edge は両端が node 集合内の link 出現。tag node / tag edge は出力しない。phantom は opt-in
- **B: tag を含む全 node / edge**: インデックスの全構造をそのまま dump する
- **C: tag を「note 間の共有属性」として edge に畳み込む**: tag node を消し、同一 tag を持つ note ペアに edge を張る

## Decision

We will export the induced subgraph of existing notes and assets (Option A): nodes are
`type IN ('note','asset') AND exists_flag=1` filtered by `--path` / `--exclude` globs, and
edges are link occurrences whose both endpoints are in the node set. Tag nodes are never
exported, so tag edges drop out naturally. Phantom targets are included only with
`--include-phantoms`. Output formats are `json` and Graphviz `dot` (no `text`, no `--fields`).
Node `id` is an export-scoped reference key for `edges[].source/target`, not stable across builds.

- B は tag node が hub になりグラフ構造を支配するため、リンク構造の可視化・分析という用途に合わない
- C は「解釈はツール側」の方針に反する（tag 共有を関連とみなすかはツール側の判断。reachable が tag edge を辿らないのと同じ整理 = ADR 0015）
- tag を含めた export が必要になったら、`--include-tags` のような opt-in で将来拡張する

## Consequences

- 出力が navigation link の構造そのものになり、外部ツールはエッジ解釈の前処理なしで使える
- 誘導部分グラフのため、`--path` で絞ると集合外への edge は消える（リンク存在の検査は diagnose / reachable の責務）
- 同一ペア間でも link 出現ごとに 1 edge 出るため、重み付けはツール側で集計できる（情報を落とさない）
- tag ベースの関連を見たい場合は現状 query / search を使う必要がある（将来拡張余地として記録）
