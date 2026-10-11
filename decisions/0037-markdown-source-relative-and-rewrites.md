---
status: current
---
# ADR 0037: Markdown の元ノート相対解決とリンク書き換え

## 背景

一般的な Markdown 文書とリポジトリ内 Markdown に合わせ、Markdown の裸名とパスを元ノート相対に統一する。リンク生成・書き換えと索引も同じ意味に揃える。ADR 0014・0022・0025 を置き換え、変更しない frontmatter と wikilink の判断も本 ADR に再掲する。

## Markdown の解決と索引

- inline / reference 定義 / 画像 / asset の `to.md` と `./to.md` は同じ元ノート相対パスとする。`path/to.md`・`./path/to.md`・`../to.md` も元ノート相対、先頭 `/` だけ Vault ルート基準とする。basename 探索とルートへの fallback を行わない。
- 外部 URI、同一ノートの fragment、URL decode と escape、alias / embed / subpath、Vault 外拒否は維持する。全文法対応、互換モード、自動 wikilink 変換は追加しない。
- index の解釈 version を更新し、旧 index の参照・差分 mutation を拒否する。in-place migration せず全量 build で再生成し、旧新の解釈を混在させない。

## 生成・書き換えと wikilink

- wikilink と frontmatter wikilink は Vault 内 basename 探索を維持する。同名候補に root のファイルがあれば root 優先とし、root がなく subdirectory の候補だけで衝突する場合は明示的なパスを要求する。Obsidian の shortest-path 選択に変えない。大小文字だけが異なる root ファイルを共存させる運用は対象外とする。
- 通常の wikilink rewrite は Vault 相対パス、Markdown rewrite は先頭 `/` 付き root パスを生成する。短縮を通常更新へ組み込まず、衝突が解消した際の Git 差分の揺れを避ける。
- move / move_dir / template では元からある元ノート相対 outgoing link を移動後の起点で再計算する。両端移動時は移動後の target、未作成 target は元の path を維持する。
- convert は Markdown から明示相対 wikilink、wikilink から解決した target の root Markdown path へ変換する。曖昧な wikilink は推測せず不変とする。参照定義は自動書き換えず、意味を維持できない mutation は変更前に拒否する。
- simplify は wikilink の一意 basename / root 優先による短縮を維持する。Markdown は同じ target を指す短い元ノート相対パスだけに短縮する。
- repair は本文の wikilink / Markdown のみを対象とし、frontmatter には適用しない。wikilink は既存の basename 修復、Markdown は選択した実在候補の明示 root path に修復する。Markdown の複数候補は選ばず、候補なしは Vault 内の明示未作成 path とする。

## frontmatter の独立した契約

- `meta.link_keys` で宣言した key の raw path のみ `frontmatter_path` edge にする。空値・外部 URI・wikilink 値を除き、`tags` の指定は設定エラーとする。parse は config 非依存で edge 生成箇所のみ link keys を渡す。
- raw path は裸名を basename、`./`・`../` を元ノート相対、それ以外を Vault 相対として解決する。Markdown の新規則には追従させず、値全体を path として fragment を分離しない。曖昧 basename・Vault 外参照は build エラーとする。
- raw path はリンク構文を持たないため rewrite 対象外とする。add / move / move_dir 前に差分後の状態で再解決し、意味が変わる操作は実行前に拒否する。phantom の basename は一意解決時だけ promotion し、未解決 path は phantom を維持する。
- quoted frontmatter wikilink は本文 wikilink と同等の rewrite 対象とし、add / update / move / move_dir / disambiguate / simplify に適用する。bare YAML 値は edge 化せず rewrite しない。抽出方式は ADR 0023、YAML source の意味保存は ADR 0024 に従う。
- rewrite 対象と escape / 曖昧性の検証対象を独立して持つ。検証対象には書き換えない raw path・参照定義も含め、add / update と build の検証に抜けを作らない。

## 帰結

従来の Markdown basename / Vault 相対リンクは参照先が変わるため、利用者は必要なリンクを修正して build する。frontmatter raw path の曖昧性・escape や意味変更拒否は引き続き手動修正を必要とする。新しい YAML 書き換え機構や参照定義の自動書き換えは導入しない。
