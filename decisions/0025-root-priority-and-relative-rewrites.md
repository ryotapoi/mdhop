---
status: current
---

# ADR 0025: basename の root 優先と相対リンクの書き換え

## 背景

root note の vault-relative path は basename と同じになる。root と subdirectory に同名 note があるだけで basename を曖昧として拒否すると、root を明示する有効な表記まで使えなくなる。一方、移動する note の既存 source-relative outgoing link は、その表記を維持しながら移動後の起点に合わせる必要がある。

ADR 0004 の root 優先は維持するが、「全書き換えを vault-relative に統一する」という判断には既存相対 outgoing link の例外があるため、全体を本 ADR で置き換える。

## 判断と理由

- 同じ basename の候補に vault root のファイルがある場合、basename link は root のファイルへ解決する。root の vault-relative path と basename が同じであることを根拠とし、衝突時も短い root link を使えるようにする。
- subdirectory のファイルだけで衝突する場合は明示的な path が必要である。Obsidian の shortest-path 選択へは変更しない。
- 通常のリンク書き換えは vault-relative path を使う。root だけに `./` や `../` を付ける特別処理は設けず、path 生成を一貫させる。
- move / move_dir における、移動 note が元から持っていた source-relative outgoing link は例外とする。元の解決先を維持し、移動後の note 起点で相対 path を計算して書き換える。両端が移動する場合も移動後の target を使う。既存の相対表記を保ちつつ意味を維持するためである。

## 帰結と範囲

root に特別な優先順位があるため、全ての basename 衝突が等しく扱われるわけではない。case-sensitive filesystem 上で大小文字だけが異なる root ファイルを共存させる運用は対象外とする。既存相対 outgoing link の例外があるので、書き換え結果全てに vault-relative path を要求しない。
