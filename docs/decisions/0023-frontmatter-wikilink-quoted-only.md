# ADR 0023: frontmatter wikilink は引用符付き YAML 値のみから抽出する

## Status

Accepted

## Context

ADR 0013 は frontmatter 行範囲の生テキスト走査で `[[...]]` を `frontmatter_wikilink` として拾う設計を採用した。これにより bare `key: [[Note]]` や bare list item `- [[Note]]` も edge 化されていた。

Obsidian の property link は、frontmatter 値が **引用符で囲まれた YAML scalar / list item** として書かれたときだけ `[[...]]` をリンクとして扱う。bare `key: [[Note]]` は YAML 上 nested flow sequence（`[ [Note] ]`）として解釈され、Obsidian も property link にならない。

mdhop の frontmatter wikilink 抽出を Obsidian と揃えないと、bare 記法が phantom や graph / reachable / 書き換え系コマンドに混入し、property link として意図していないリンクが辿れる・書き換わる。

## Considered Options

- **A: ADR 0013 の生テキスト走査を維持し bare も拾い続ける**: 実装変更は小さいが Obsidian 互換でなく、bare が phantom や mutation 検証に入り続ける
- **B: `yaml.Node` を辿り、double/single-quoted scalar と quoted list item の値だけ `parseWikiLinks` に渡す**: Obsidian property link と一致。bare scalar / bare list / block scalar は対象外
- **C: meta テーブルに載った値だけから wikilink を判定する**: meta は quoted 値のみ載るため bare は自然に除外されるが、抽出と meta 格納の責務が混ざり、graph 構築時の link 抽出ロジックが meta 依存になる

## Decision

We will adopt option B: frontmatter wikilink は `yaml.Node` 上で double/single-quoted scalar と、それらを含む sequence（list item）だけから抽出する。値文字列は YAML パーサが引用符を外した後の `val.Value` を `parseWikiLinks` に渡し、`linkType="frontmatter_wikilink"` を付ける。

対象外（edge・phantom・meta-check issue・graph / reachable / 書き換え系 / mutation 検証に含めない）:

- bare scalar `key: [[Note]]`
- bare list item `- [[Note]]`
- block scalar（`key: |` / `key: >`）内の `[[...]]` テキスト

`tags` キーは従来どおり frontmatter wikilink 抽出から除外する（`parseFrontmatterTags` が担当）。

本文内 `[[Note]]` は変更しない。

## Consequences

肯定的:

- Obsidian property link セマンティクスと一致し、bare 記法による false positive が消える
- 抽出は YAML 構造と引用符スタイルに依存するため、行範囲推定や block scalar 用の特別扱いが不要になる
- quoted 値は meta テーブルにも載るため、frontmatter wikilink edge と meta 検索の対象が揃う

否定的:

- ADR 0013 で「block scalar も行範囲走査で自動カバー」としていた挙動は打ち切る（block scalar 内の `[[...]]` は edge にならない）
- bare `[[Note]]` を frontmatter リンクとして使っていた vault では edge が減る（意図した互換性変更）

中立的:

- ADR 0022 の書き換え展開は quoted frontmatter wikilink に対して引き続き有効。bare 行は書き換え対象外のまま残る
