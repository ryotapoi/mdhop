---
status: current
---

# ADR 0030: frontmatter の参照検査と schema 検査を分ける

## 背景

frontmatter の品質には、値が vault 内の path / wikilink として実在するかと、key の存在・型・enum が宣言した schema に従うかの異なる問いがある。ADR 0019 のコマンド分離を維持し、両コマンドが解決 map を共有するという記述を置き換える。

## 判断と理由

- 参照実在検査は meta-check、schema 準拠検査は meta-validate という独立した top-level command にする。入力、出力の意味、変更理由が別であるためである。
- meta-check は key と値の解釈方法を受け取り、not_found / ambiguous / vault_escape / not_wikilink 等の参照問題を返す。meta-validate は require 条件や config の schema 宣言を用い、missing / type / enum 等の問題を返す。参照解決の拡張と schema 表現の拡張を同じ command の mode 切替にしない。
- 一つの command にサブモードとして統合する案は採らない。独立した価値を一つの入口に畳むと help と flag 体系が読みにくくなる。
- diagnose へ相乗りさせない。diagnose の複数検査はリンクグラフの破綻という意味境界に属するが、frontmatter schema の準拠は別の目的である。
- 共通の下地データや処理を使う場合も、目的の独立性を理由に command は分ける。解決 map は参照検査側が必要とし、schema 検査側に共有を強制しない。単に同じ meta データを読むことは command 統合の理由にならない。

## 帰結

command 数は増えるが、利用者は「参照が壊れているか」と「schema に従っているか」を別々に呼び、各 help・flag・出力を一つの検査目的として理解できる。今後の検査も意味境界ごとに分ける判断の前例とするが、将来の command 追加を約束するものではない。
