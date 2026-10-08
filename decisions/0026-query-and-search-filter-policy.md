---
status: current
---

# ADR 0026: query と search のフィルタ方針

## 背景

daily note、template、広範な tag は agent の探索結果にノイズを生む。永続設定と呼び出し時の条件が必要であり、query では「結果を隠すこと」と「two-hop の経由点を選ぶこと」を区別する。ADR 0006 の設定形式の判断を引き継ぎ、query だけへの適用、旧フラグ、SQL だけで除外する方針を置き換える。

## 判断と理由

- 任意の設定ファイル `mdhop.yaml` を vault root に置き、YAML を使う。設定を発見しやすくし、生成データの `.mdhop/` と区別する。既存の `yaml.v3` を使い、CLI は標準 `flag` のままにする。TOML の新規依存や、コメントを持てない JSON は採らない。設定なしでも使える状態を保つ。
- path 条件の glob は `*` が `/` を含む任意の文字に一致する規則を使い、`[` を含む pattern はエラーとする。SQLite GLOB と Go の比較を揃え、利用上の利益が小さい character class の二重実装を避ける。tag 条件は大小文字を区別しない完全一致とする。
- query の hide は表示する note / tag target と via identifier を隠す条件であり、via 条件は two-hop の経由点の適格性を決める条件である。hidden via から target を発見することは妨げない。両者を一つの除外にまとめると、表示と探索の意味が混ざるため分離する。
- query は `query.hide`、`query.via.include`、`query.via.exclude` を使う。`query.via.exclude` が無指定の場合だけ、従来の top-level `exclude` を via 除外の fallback とする。config と呼び出し条件を合成し、via では包含に加えて除外を優先する。typed `--via` は type と完全な identifier で経由点を選び、glob や basename fallback として扱わない。
- query の呼び出し条件は `--hide-path` / `--hide-tag`、`--via-path` / `--via-tag`、`--exclude-via-path` / `--exclude-via-tag` とし、設定無効化は hide と via を別々に扱う。旧 query の `--exclude` / `--exclude-tag` / `--no-exclude` を現在の interface とはしない。
- search は top-level `exclude` と CLI の `--exclude` を合成し、`--no-exclude` で設定由来の除外を無効化する。top-level の query / search 除外設定は stats / diagnose の既定除外には使わない。各コマンド固有の明示的 path filter と、build の登録除外は別の用途である。
- filter は取得結果の LIMIT 後に適用せず、可視 target の集合を確定してからページを切る。search 等の SQL filter は parameterized query とし、query の hide / via 判定は Go 側でも行う。SQL だけに限定することより、除外で返却件数や target paging の意味を壊さないことを優先する。

## 帰結

config と CLI の責務が増えるが、表示だけ隠す操作と探索自体から外す操作を独立して指定できる。設定形式は YAML に依存し、将来変更するなら移行が必要になる。character class の拡張は現在の判断に含めない。
