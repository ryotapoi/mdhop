# ADR 0019: meta-check と meta-validate を別コマンドにする

## Status

Accepted

## Context

v0.11.0 で frontmatter の品質検査を 2 種類追加することになった。

- **meta-check**: 任意 key の値が vault 内の path / wikilink として実在するかを検査する（`sources: ./guide.md` が壊れていないか）。「値の参照先が実在するか」を見る。
- **meta-validate**: frontmatter が宣言済み schema に準拠するかを検査する（必須 key の欠落、enum 外の値、date 型 parse 不可、空値）。mdhop.yaml の `meta.types` を検査側でも使う。「key の存在と型・enum 妥当性」を見る。

backlog には「コマンドを分けるか統合するかは設計時に判断する」とあった。判断材料:

- 入力が違う。meta-check は `--key` + `--kind`（値をどう解釈するか）。meta-validate は `--require` + mdhop.yaml の `meta.types`（宣言済み schema）。
- 出力の意味が違う。meta-check は broken reference（not_found / ambiguous / vault_escape / not_wikilink）。meta-validate は schema 違反（missing / type mismatch / enum out / empty）。
- 既存コマンドは原則 1 コマンド 1 責務（resolve / query / search / reachable / graph）。`diagnose` だけが複数検査を `--fields` で束ねるが、これはいずれも「リンクグラフの破綻」という単一の意味境界に属する検査の集合だった。

## Considered Options

- **選択肢A**: 2 つを別コマンド（`meta-check` / `meta-validate`）にする。
- **選択肢B**: 1 コマンド（例 `meta-check` にサブモード）へ統合し、フラグで参照検査と schema 検査を切り替える。
- **選択肢C**: `diagnose` の `--fields` に meta 系検査を相乗りさせる。

## Decision

We will ship meta-check and meta-validate as two separate top-level commands.

参照実在検査（meta-check）と schema 準拠検査（meta-validate）は、ユーザーが説明できる価値の単位として別物であり、入力（`--kind` vs `meta.types`）と出力の意味（broken reference vs schema 違反）が独立している。将来の変更理由も別系統（解決ロジックの拡張 vs schema 表現の拡張）になる。`diagnose` への相乗り（選択肢C）は、diagnose が持つ「リンクグラフの破綻」という意味境界と異なるため不採用。1 コマンドへの統合（選択肢B）は、切り替えフラグが実質「別コマンドを 1 つのエントリに畳んだだけ」になり、help とフラグ体系を読みにくくするため不採用。

## Consequences

- 各コマンドの help・フラグ・出力が、それぞれ単一の検査目的に閉じる。ユーザーは「参照が壊れているか」「schema に従っているか」を別々に呼べる。
- 解決マップ構築（`newResolveMaps`）と meta テーブル読み出しは両コマンドで共有するが、これは「同じ目的の処理」ではなく「同じ下地データを別目的で使う」ため、コマンドは分けつつヘルパーだけ共有する形になる。
- コマンド数が増える。ただし 1 コマンド 1 責務という既存の設計思想と整合する範囲。
- 将来 frontmatter 検査をさらに増やす場合も、意味境界ごとにコマンドを足す方針が指針になる（本 ADR が将来判断の前例）。
