---
status: current
---

# ADR 0035: 行番号による resolve の索引出現選択

## 判断

クリックした本文リンクの出現文脈を指定するため、resolve に任意の `--line` を追加する方針を採用する。列位置・文字オフセットは追加しない。判断は有効だが、`--line` は未実装である。詳細契約・実装範囲・受入条件の正本は [backlog の実装タスク](../backlog/backlog.md#今後の検討)とする。

位置を指定する経路は、元ノート・索引の原文・元ファイルの物理行が一致する索引出現だけを選ぶ。同じ行で異なる解決先や subpath が残れば拒否し、一致しない場合も別行や原文再解釈へフォールバックしない。位置を省略する既存の解決契約と出力形は維持する。行指定は任意の出現を一意に識別する API ではなく、既存索引の文脈を選択するための条件である。

## 理由

同じノートの `[[x\|表示文字列]]` は、表内で `x.md`、表外で `x\.md` を指し得る。現状維持では元ノートと原文だけでクリックした側を選べない。呼び出し元で原文を変換する案は、表構文の意味を再実装し、索引の原文との対応を失うため採らない。

`parse.go` の `walkBodyLines` は frontmatter・空行・コードフェンスを含む元ファイルの 1 始まりの物理行番号を保ち、`parse_table.go` の表判定は行単位である。[TestTableWikilinkContext](../internal/core/link_interpretation_test.go)は表内外の同一 raw、異なる target、元の行番号を確認している。`db.go` の edges は raw_link・target_id・subpath・line_start・line_end・in_table を保持する。この要求は既存の行情報で区別でき、列・オフセットのために parser・schema・呼び出し元の座標定義を広げる必要がない。同一行の異なる意味を選び分ける保証は付けず、曖昧として拒否する。

[TestExactRawTableResolveSnapshotAndAmbiguity](../internal/core/link_interpretation_test.go)は、本文編集後も更新前は旧索引の結果を返すこと、表内外の意味が混在すれば曖昧となることを確認している。本文を再解析してクリック文脈を推定するのでなく、この索引 snapshot を利用する方針を維持する。

## 責任境界と限界

CLI は行指定の解析・検証・help、core は索引の出現選択と feature SQL を所有する（[ADR 0032](0032-module-and-sql-ownership.md)）。schema・parser・[既存 dispatcher](0031-typed-link-resolver-backends.md)の変更は予定しない。未指定時の再解析経路はそのまま維持する。

呼び出し元は同じ Vault / DB と索引作成時の保存済み本文から原文・物理行を取得し、位置取得から解決まで本文と索引を変更しない。保存・索引更新後は位置を取得し直す。参照定義や表境界の編集も原文が同じまま意味を変え得る。未保存 buffer、原文へ逆対応できない renderer、古い索引との組み合わせに正しいクリック先を保証しない。

行と原文の一致は同一世代の証明にならない。resolve に本文再読・mtime/hash 検査・世代 token・自動更新・排他制御を追加せず、並行編集・更新をまたぐ snapshot 保証は付けない（[要件](../docs/requirements.md)、[ADR 0034](0034-cache-index-and-location-policy.md)）。呼び出し元の UI / renderer の実装はこの判断に含まない。

主対象は現在対応する単一行の本文リンクである。参照リンクの位置は定義行でなく使用箇所とする。frontmatter の座標は YAML scalar に由来し、タグの raw_link には生成された prefix もあるため、すべての edge の raw_link が本文の逐語的切り出しであるとは保証しない。既存の raw_link 表現を変えず、frontmatter や未対応の複数行文法の保証を広げない。
