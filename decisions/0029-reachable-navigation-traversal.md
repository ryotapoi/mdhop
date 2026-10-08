---
status: current
---

# ADR 0029: reachable は navigation edge を BFS で辿る

## 背景

reachable は入口 note からリンクで辿れない note を検出する read-only コマンドである。ADR 0015 の走査判断を維持するが、リンク解決 interface を導入しない判断は ADR 0031 に置き換わったため、現在の走査判断を本 ADR に揃える。

## 判断と理由

- note 間の navigation link を辿り、tag / frontmatter tag edge は辿らない。wikilink、markdown、markdown_reference、frontmatter_wikilink、frontmatter_path を navigation とする。同じ tag を共有するだけでは、入口からリンクで到達できることにはならないためである。
- build 済み edges を読み込んで in-memory BFS を行う。reachable 自体はリンクを再解決せず、解決ロジックの別実装を増やさない。リンク解決側が共通 backend を使うかどうかと、走査側が解決済み edge を使うことは別の判断である。
- route は BFS の parent を記録して最短経路として構成する。SQLite 再帰 CTE は、route 構築と Go 側処理の分担が複雑になるため採らない。
- 実在 note を走査の起点となる source とし、asset / phantom target は outgoing を持たない葉として扱う。検査対象 note を絞る path filter は、経路の connector note を除く条件とはしない。

## 帰結と範囲

tag の共有による到達の誤判定を避け、raw path edge も到達性に反映できる。adjacency のメモリ読込を必要とするが、既存の実測 3476 notes / 12309 edges では問題がなく、極端に大きい vault の追加対応は決めていない。最大深さ指定は用途が未確定のため現在の対象外とし、将来の機能を約束しない。
