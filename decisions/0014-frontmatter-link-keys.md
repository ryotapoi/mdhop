# ADR 0014: meta.link_keys による frontmatter raw path の edge 化

## Status

Accepted

## Context

`related:` / `sources:` のような frontmatter key に raw path（`../topics/foo.md`、`docs/foo.md`）を書く運用が実 vault で一般的だが、これらは edge にならずグラフから見えなかった（frontmatter 値内の `[[...]]` のみ `frontmatter_wikilink` として edge 化済み）。raw path が edge にならないと、到達性チェック（reachable）の false positive と孤立検出の見落としが起きる。

mdhop は厳密モード（曖昧リンク禁止・vault escape 禁止）を保証し、move / disambiguate 等の rewrite 系はリンク構文（`[[...]]` / `[text](path)`）の文字列書き換えで実装されている。raw path 値はリンク構文を持たないため、既存の rewrite 機構では書き換えられない。

## Considered Options

- **A: opt-in 設定 + markdown link と同一の解決規則 + 厳密モード検証 + rewrite 対象外**: `meta.link_keys` で宣言した key のみ edge 化。escape / 曖昧 basename は build エラー。rewrite 系は対象外とし、raw path の解決先が変わる add / move は操作前にエラーで止める。
- **B: 解決規則を vault 相対のみに限定**: 実装は簡単だが、`../topics/foo.md` のような note 起点の相対 path 運用を取りこぼす。
- **C: rewrite 系も raw path 値を書き換える**: move 後も raw path が追従するが、YAML 値の文字列書き換えという新機構が必要で、quoted / list / block scalar の全形式を安全に書き換えるコストが高い。
- **D: 解決失敗やエラーを警告に留める（検証しない）**: link_keys 有効化で build が壊れないが、「曖昧リンクは存在しない」という厳密モードの保証が edge の種類によって崩れる。

## Decision

We will adopt Option A: edge 化は `meta.link_keys` による opt-in とし、解決規則は markdown link と同一、厳密モードの検証（vault escape / 曖昧 basename エラー）を適用し、rewrite 系の対象外とする。URL 値（`://` を含む）と wikilink 値はスキップし、`tags` キーの指定は設定エラーとする。link_type は `frontmatter_path` を新設する。

rewrite 対象外の帰結として、raw path 値の解決先を変えてしまう操作（add / move / move_dir）は実行前に全 `frontmatter_path` edge を差分後の状態で再解決し、解決先が変わるならエラーで止める（incremental DB と full build の不一致を作らない）。phantom 化している raw path は、basename 値が新ファイルに一意解決する場合のみ promotion し、path 値が解決しないままなら phantom に残す。

## Consequences

- 肯定的: backlinks / twohop / 孤立検出 / 到達性チェックに raw path 参照が反映される。未設定 vault の挙動は完全に従来どおり。edge 種別で raw path 由来を識別できる。
- 肯定的: 厳密モードの保証（曖昧リンクなし・escape なし）が全 edge 種別で一貫する。
- 否定的: link_keys を有効化すると、これまで見えていなかった raw path の曖昧・escape が build エラーとして顕在化する（修正は手動）。
- 否定的: raw path 値の解決先が変わる add / move はエラーになり、ユーザーが frontmatter 値を手で直すまで操作できない。書き換え対応（Option C）は将来の要望次第。
- 中立的: parse 層は config 非依存のまま（`parseLinksWithLinkKeys` を edge 生成箇所でのみ使用）。
