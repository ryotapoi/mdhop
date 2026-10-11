---
status: current
---

# ADR 0033: TOML 設定と build / query / search のフィルタ方針

## 背景

設定の生成・編集に使う形式を TOML に統一する。build の登録除外と探索時のフィルタは用途が異なり、query では表示を隠す条件と two-hop の経由点を選ぶ条件も分ける。ADR 0007 と 0026 の形式以外の判断を引き継ぐ。

## 判断と理由

- 既定の設定ファイルは Vault root の `mdhop.toml` とし、`--config` があれば指定した一ファイルだけを読む。任意名の明示指定を許可し、既定との併読・マージ・自動検出はしない。既定欠落のみ設定なしとし、明示欠落・読取失敗・不正内容はエラー。配置引数の相対パスは cwd 基準、設定内 glob は設定場所によらず Vault 基準とする。`github.com/pelletier/go-toml/v2` を使い、CLI は標準 `flag` のままにする。設定なしでも使える状態を保つ。通常操作は旧 `mdhop.yaml` を読まず、変換・削除もしない。ノートの YAML frontmatter は `yaml.v3` で引き続き扱う。
- `init-meta` は TOML を生成・更新し、既存の設定値と明示的な型宣言を優先する。通常の table / inline table と quoted key は同じ値として扱い、再生成時の書式や任意のコメントの完全保存は約束しない。型宣言は文字列、ordered は `ordered` 配列を持つ table、profiles は table array とする。設定に Vault / DB のパス指定は設けない。
- path 条件の glob は `*` が `/` を含む任意の文字に一致する規則を使い、`[` を含む pattern はエラーとする。SQLite GLOB と Go の比較を揃え、利用上の利益が小さい character class の二重実装を避ける。tag 条件は大小文字を区別しない完全一致とする。
- build は設定の `build.exclude_paths` に従い、索引登録前にファイルを除外する。探索用の `exclude.paths` と独立して指定する。build の除外を呼び出しごとの CLI flag や別の ignore 形式には分散させない。除外先へのリンクは phantom にし、除外ノートの tag を登録しない。`DisambiguateScan` も同じ除外を適用する。mutation commands (`add` / `update` / `delete` / `move`) は DB state を操作し、build 除外による不整合は次の build で解消する。
- query の hide は表示する note / tag target と via identifier を隠す条件であり、via 条件は two-hop の経由点の適格性を決める条件である。hidden via から target を発見することは妨げない。両者を一つの除外にまとめると、表示と探索の意味が混ざるため分離する。
- query は `query.hide`、`query.via.include`、`query.via.exclude` を使う。`query.via.exclude` が無指定の場合だけ、top-level `exclude` を via 除外の fallback とする。明示空 table は fallback を無効にする。config と呼び出し条件を合成し、via では包含に加えて除外を優先する。typed `--via` は type と完全な identifier で経由点を選び、glob や basename fallback として扱わない。
- query の呼び出し条件は `--hide-path` / `--hide-tag`、`--via-path` / `--via-tag`、`--exclude-via-path` / `--exclude-via-tag` とし、設定無効化は hide と via を別々に扱う。旧 query の `--exclude` / `--exclude-tag` / `--no-exclude` を現在の interface とはしない。
- search は top-level `exclude` と CLI の `--exclude` を合成し、`--no-exclude` で設定由来の除外を無効化する。top-level の query / search 除外設定は stats / diagnose の既定除外には使わない。各コマンド固有の明示的 path filter と、build の登録除外は別の用途である。
- filter は取得結果の LIMIT 後に適用せず、可視 target の集合を確定してからページを切る。search 等の SQL filter は parameterized query とし、query の hide / via 判定は Go 側でも行う。SQL だけに限定することより、除外で返却件数や target paging の意味を壊さないことを優先する。

## 帰結

設定の parse に失敗すれば build を含む利用コマンドは失敗する。build の除外先を含む basename collision は診断の対象外になる。表示だけ隠す操作と探索自体から外す操作を独立して指定できる。character class の拡張や旧設定の移行操作はこの判断に含めない。

## 索引・設定の独立配置と公開

`--vault`・`--db`・`--config` は独立した配置引数とする。Vault は未指定時 cwd、DB は未指定時 `<vault>/.mdhop/index.sqlite`。core へ配置値を明示的に渡し、Vault を DB・設定から推定しない。DB を使わない scan・設定生成では指定 DB を開かない。`init-meta` の設定読込・raw TOML 読込・`--write` は同じ設定先を使い、書出しの一時ファイルもそのディレクトリに置く。

build は指定 DB の親ディレクトリで固有の一時 DB を作り、transaction の commit と接続 close の成功後に同一ディレクトリの rename で公開する。旧 DB は失敗時に保持し、外部 DB の利用では Vault 内へ書き込まない。選択 DB・SQLite 補助ファイル・build 一時 DB は索引入力・status の走査候補・directory delete/move のディスク走査対象から除く。ノート操作で DB 保存先を削除・移動しない。

Ubuntu・macOS のローカル filesystem の rename では、置換前から接続済みの reader は旧ファイルを、置換後の新規 reader は新ファイルを参照する。一つの操作中に追加接続が別世代を開かないよう、DB pool は最大一接続・一 idle 接続を保持し、接続の有効期限を設けない。SQLite URI の予約文字は filesystem path として escape する。並行参照と build を実 SQLite integration test で確認する。複数 CLI 呼出し間の同一世代、DB と編集中本文の同時点は保証せず、既存 preview stale 検査を維持する。build と update 等の同時書込は未サポートであり、利用側で直列化する。ネットワーク filesystem・非 Unix OS の原子的置換は保証しない。

判断は [SQLite URI filename](https://www.sqlite.org/uri.html)、[Go database/sql connection pool](https://pkg.go.dev/database/sql#DB.SetMaxOpenConns)、[Go os.Rename](https://pkg.go.dev/os#Rename) の仕様と実装・test に照合した。
