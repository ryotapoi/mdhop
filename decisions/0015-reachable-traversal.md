# ADR 0015: reachable の走査設計

## Status

Accepted

## Context

v0.10.0 で到達性チェック（`reachable --from <note> --path <glob>`）を追加する。入口 note からリンクで辿れない note を検出する read-only コマンドで、設計上の論点は (1) どの edge を辿るか、(2) リンク解決ロジックとの関係（ADR 0002 の二重化を増やすか）の 2 つ。

backlog には「reachable 設計時に ADR 0002（map ベースと DB ベースの解決ロジック二重化の許容）を再評価する」という項目があり、グラフ走査が増えるタイミングで interface 抽象へ進むかの判断が求められていた。

## Considered Options

走査対象 edge:

- **A: ナビゲーションリンクのみ（`wikilink` / `markdown` / `frontmatter_wikilink` / `frontmatter_path`）**: tag 系 link_type（`tag` / `frontmatter`）は辿らない
- **B: 全 link_type**: tag ノードも経由する。tag ノードに outgoing edge はないため現状の結果は A と同じだが、「同じ tag を持つだけの note」を将来 reachable 扱いする拡張（tag → note の逆辿り）に道を開く

走査の実装:

- **C: edges テーブルを一括ロードして in-memory BFS**: 解決済み edge の走査のみで、リンク解決ロジックを使わない
- **D: SQLite 再帰 CTE**: SQL 1 本で済むが、route 構築（parent 追跡）と Go 側ロジックの分担が複雑になる

## Decision

We will traverse only navigation link types (Option A) using an in-memory BFS over the edges table (Option C).

到達性は「リンクで辿れるか」の検査であり、tag の共有は到達と見なさない。この境界を link_type の除外として明示する。

ADR 0002 の再評価: reachable はリンク解決を一切行わず、build 済みの edges を走査するだけなので、map ベース / DB ベースの解決ロジック二重化に第三の実装を加えない。interface 抽象への移行は引き続き見送る（ADR 0002 は現状のまま有効）。

## Consequences

- 肯定的: tag を共有するだけの note が reachable と誤判定されない。`meta.link_keys`（ADR 0014）の raw path edge も自然に走査対象になる
- 肯定的: 解決ロジックの二重化が増えず、ADR 0002 の構図が変わらない
- 肯定的: in-memory BFS は最短経路（`--route`）の構築が素直で、NULL 三値論理の罠も踏まない
- 否定的: vault 全体の edge をメモリにロードする（実測 3476 notes / 12309 edges で問題なし。極端に大きい vault では将来要検討）
- 中立的: 最大深さ指定（`--max-depth`）は用途未確定のため非対象とした
