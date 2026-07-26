# ADR 0022: frontmatter wikilink を書き換え系コマンドへ展開する

## Status

Accepted

## Context

ADR 0013 は frontmatter 内の `[[...]]` を `frontmatter_wikilink` として検出・edge 化する設計を決めたが、リリースを 2 段階に分けていた。(1/2) は検出と edge 化のみを行い、書き換え系コマンドの SQL フィルタ（`link_type IN ('wikilink', 'markdown')`）と Go コードフィルタを意図的に拡張しないことで、マージ時に既存挙動を壊さない設計としていた。書き換え系の対応は (2/2) に先送りされ、ADR 0013 の Consequences には「(1/2) では検出と edge 化のみ行い、書き換え系は (2/2) で対応する」と記録されている。

先送りの理由は、frontmatter が本文と異なる制約を持つためだった。YAML 側の quoted / bare スタイルの保持、行範囲の制約、`add.go` / `update.go` の ambiguous / vault-escape 検証ガードとの整合など、本文 wikilink とは別に確認すべき点があった。

本 ADR は、この (2/2) を実装する判断を記録する。ADR 0013 が決めた抽出方式（YAML の `Node.Kind` を辿らず、frontmatter 行範囲の生テキストに本文 wikilink 抽出ロジックを当てる）は本 ADR の対象外であり、引き続き有効。

## Considered Options

- **A: 全書き換え系コマンドで frontmatter_wikilink を本文 wikilink と同等に扱う**: link type のフィルタを集約した 1 箇所に `frontmatter_wikilink` を加える。ADR 0013 が「抽出ロジックを `parseWikiLinks` で再利用したため alias / subpath / vault-relative path / phantom 解決は本文 wikilink と完全に同じ振る舞いになる」と記録した性質をそのまま書き換えにも適用する
- **B: コマンドごとに個別のフィルタで段階的に対応する**: move だけ先に対応する等、影響範囲を絞れるが、コマンドごとに link type の扱いが食い違い、どのコマンドが frontmatter を書き換えるかをユーザーが覚える必要が出る
- **C: (1/2) の状態を維持し、frontmatter wikilink は書き換え対象外のままとする**: 実装コストはゼロだが、move / disambiguate の後に frontmatter のリンクだけが古いパスを指したまま残り、厳密モード（曖昧リンク禁止・vault escape 禁止）の保証が edge の種類によって崩れる

## Decision

We will adopt option A: `frontmatter_wikilink` を本文 wikilink と同等の書き換え対象とする。add / update / move / move_dir / disambiguate / simplify が対象となる。

link type の判定は用途ごとに 2 つに分ける。書き換え対象かどうかは `rewriteLinkTypes`（`wikilink` / `markdown` / `frontmatter_wikilink`）が持ち、edge を SQL で絞る箇所はこれを使う。escape / 曖昧性の検証対象かどうかは `isPathLinkType` が持ち、こちらは `frontmatter_path` も含む（ADR 0014 により検証はするが書き換えない）。この 2 つを分けることで、「検証するが書き換えない」link type を表現できる。

ただし `repair` のみ例外とし、body link（`wikilink` / `markdown`）だけを書き換える。repair は壊れたパスリンク・vault-escape リンクを basename リンクへ書き換えて build 可能な状態へ復旧するツールであり、対象を本文に限ることで復旧操作の影響範囲を予測可能に保つ。判定は `isBodyPathLinkType` として別関数に分ける。

`add.go` / `update.go` の ambiguous / vault-escape 検証ガードも `build.go` と揃え、link type による検証の抜けを作らない。

## Consequences

- 肯定的: move / disambiguate 後に frontmatter のリンクが古いパスを指したまま残らない。厳密モードの保証が link type によらず一貫する
- 肯定的: 書き換え対象と検証対象の判定がそれぞれ 1 箇所に集約され、link type が増えたときの追従先が明確になる
- 否定的: `rewriteLinkTypes` と `isPathLinkType` の 2 つを取り違えると、書き換えない link type を書き換え対象に含める（またはその逆）誤りが起きうる。両者の違いはコードコメントで明示している
- 否定的: repair だけが別の判定関数（`isBodyPathLinkType`）を持つため、「なぜ repair は違うのか」を知るには本 ADR かコードコメントを読む必要がある
- 中立的: ADR 0014 の `frontmatter_path`（raw path 値）は本 ADR の対象外。リンク構文を持たないため文字列書き換えの機構に乗らず、rewrite 対象外という ADR 0014 の決定がそのまま有効
