---
status: current
---

# ADR 0031: リンク解決の共通 dispatcher と型付き backend

## 背景

build は in-memory map、resolve は DB query、dry 検証は実在 path を使う。独立した解決順序の実装は、self / tag / path / basename の分岐、vault escape 検査、link type 対応を各実装へ同期する負担と誤解決の危険を増やす。

ADR 0002 の独立実装は、二つしかない段階の抽象化コストを避ける判断だった。実装が三つに増えた段階で ADR 0021 が共通化した。共通化は維持するが、dry backend の一時 ID は利用側が必要とする path のためだけの変換になっていたため、本 ADR で現在の結果型を含めて判断を揃える。

## 判断と理由

- 解決順序・vault escape 検査・link type ごとの分岐を `resolveLinkWithBackend` に集約する。backend は順序を知らず、ストレージ固有の lookup だけを持つ。順序の変更を各実装へ同期する負担をなくすためである。
- `linkResolverBackend[T]` は self、tag、path、basename の四つの lookup を持つ。build は map backend、resolve は DB backend、dry 検証は dry backend を使う。resolve のために全ファイルを読み込む案や、dry 検証を既存 backend の flag 分岐へ押し込む案は採らない。
- 結果型だけを型パラメータとする。DB / build は実 node ID を返し、dry 検証は解決後の path を直接返す。dry は実在 note / asset だけを解決し、phantom / tag node を作らない。
- dry の一時 ID 採番と ID / path の双方向変換は持たない。利用側には path が必要で、ID の同一性や永続化は要求されていないためである。結果型を合わせるためだけの状態を増やさない。
- dry 検証の dispatcher を別に作る案は採らない。結果型の違いで解決順序を分けると、共通化で除いた同期負担が戻るためである。

## 帰結

backend の lookup を追加・変更する際に解決順序を複製しない。一方、dispatcher の変更は全 backend に波及し、build と resolve の完全な独立性は持たない。backend だけを読んでも解決順序は分からず、順序は dispatcher を読む必要がある。現在必要な三つの解決文脈のための構造であり、将来の backend 追加自体を決定するものではない。
