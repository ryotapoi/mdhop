---
status: current
---

# ADR 0023: frontmatter wikilink は引用符付き YAML の単一リンク値から抽出する

## Context

ADR 0013 は frontmatter 行範囲の生テキスト走査で `[[...]]` を `frontmatter_wikilink` として拾う設計を採用した。これにより bare `key: [[Note]]` や bare list item `- [[Note]]` も edge 化されていた。

Obsidian の property link は、frontmatter 値が **引用符で囲まれた YAML scalar / list item** として書かれたときだけ `[[...]]` をリンクとして扱う。bare `key: [[Note]]` は YAML 上 nested flow sequence（`[ [Note] ]`）として解釈され、Obsidian も property link にならない。

mdhop の frontmatter wikilink 抽出を Obsidian と揃えないと、bare 記法が phantom や graph / reachable / 書き換え系コマンドに混入し、property link として意図していないリンクが辿れる・書き換わる。

## Considered Options

- **A: ADR 0013 の生テキスト走査を維持し bare も拾い続ける**: 実装変更は小さいが Obsidian 互換でなく、bare が phantom や mutation 検証に入り続ける
- **B: `yaml.Node` を辿り、double/single-quoted scalar と quoted list item の値だけ `parseWikiLinks` に渡す**: Obsidian property link と一致。bare scalar / bare list / block scalar は対象外
- **C: meta テーブルに載った値だけから wikilink を判定する**: meta は quoted 値のみ載るため bare は自然に除外されるが、抽出と meta 格納の責務が混ざり、graph 構築時の link 抽出ロジックが meta 依存になる

## Decision

We will adopt option B: frontmatter wikilink は `yaml.Node` 上で double/single-quoted scalar と、それらを含む sequence（list item）だけから抽出する。YAML 復号後の値全体が単一の `[[foo]]` である場合だけ `linkType="frontmatter_wikilink"` を付ける。配列は各要素を独立に判定する。alias / subpath / self-heading は既存の意味を保ち、外側の余白や embed prefix は取り除かない。

`related: ["[[Existing]]", "[[Missing]]"]` は二つのリンクになる。`related: "[[Existing]] [[Missing]]"`、`related: "説明 [[foo]]"`、`related: "[[foo]] 説明"` は通常文字列として meta に保持し、edge・phantom・書き換え・mutation のリンク検証対象にしない。同じ全体判定を抽出・meta-check・rewrite で共有する。meta-check の `auto` は通常文字列内部の完全な wikilink を検査せず、明示 `wikilink` は `not_wikilink` を返す。純粋な raw path、URL/空値の skip、壊れた先頭 `[[broken` の検査は維持する。`meta.link_keys` の raw path 契約は変更しない。

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
- 通常文字列も meta 属性として保持し、値の一部だけをリンクに昇格させない

否定的:

- ADR 0013 で「block scalar も行範囲走査で自動カバー」としていた挙動は打ち切る（block scalar 内の `[[...]]` は edge にならない）
- bare `[[Note]]` を frontmatter リンクとして使っていた vault では edge が減る（意図した互換性変更）

中立的:

- ADR 0022 の書き換え展開は quoted frontmatter wikilink に対して引き続き有効。bare 行は書き換え対象外のまま残る

- 複数リンクや説明文付きの quoted 値を従来 edge 化していた索引は互換ではない。解釈 version を 6 から 7 に更新し、旧索引の read / partial mutation は拒否する。`mdhop build` は mtime・size 未変更でも全量再解析し、旧 edge / phantom を除去する。in-place migration は行わない。
