# mdhop

[![Test](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml/badge.svg)](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml)

Markdown リポジトリ内のリンク関係を SQLite にインデックス化する CLI ツール。Obsidian Vault 相当のディレクトリで wikilink / markdown link / tag / frontmatter を解析し、grep に頼らず関連ノートへ辿れるようにする。Coding Agent（Claude Code, Codex 等）と CLI ユーザーの両方で使える。

[English](README.md) · 日本語

frontmatter の wikilink は、引用符付き YAML scalar または配列の各文字列要素の値全体が単一の `[[foo]]` の場合だけ扱います。`related: ["[[Existing]]", "[[Missing]]"]` は二つのリンクですが、`related: "[[Existing]] [[Missing]]"` や `related: "説明 [[foo]]"` は通常の属性文字列です。内部のリンクは辿らず、書き換えません。`meta-check --kind auto` はこれらを skip し、明示 `--kind wikilink` は `not_wikilink` とします。本文の複数リンクと `meta.link_keys` の raw path は従来どおりです。旧索引は `mdhop build` で全量再生成してください。未変更ファイルも再解析し、旧 edge / phantom を除去します。旧索引の読み取り・部分更新はできません。

## 特徴

- **事前解析・即応答** — Vault 全体を SQLite にインデックス化。クエリは数ミリ秒で返る
- **Backlinks / Two-Hop Links / Tags** — 起点ノートから関連情報を一発取得
- **wikilink / markdown link / tag / frontmatter 対応** — Obsidian 互換のリンク解釈
- **Markdown 参照リンク対応** — 同一文書の定義を使って完全形・省略形の参照を解決し、関連検索に反映
- **ローカル完結** — 外部サービス不要。pure Go + SQLite
- **Coding Agent 向け最適化** — `query --relations`、`inspect --fields`、`--include-snippet` で必要最小限のコンテキストだけ返す

## インストール

```bash
go install github.com/ryotapoi/mdhop/cmd/mdhop@latest
```

v0.22.0 では `mdhop migrate` で旧設定・索引を新しい配置へ移行してください。[旧配置からの移行](#旧配置からの移行)を参照してください。

既存の Vault を v0.20.0 または v0.21.0 にアップグレードした場合は、`mdhop build` を実行してください。v0.20.0 では参照リンクの定義先を保存し、v0.19.5 で変更された number 型 frontmatter metadata の sort 形式も再生成します。v0.21.0 では表内のリンク文脈と index の解釈 version を保存するため、旧 index の再生成が必要です。

Markdown link の `to.md` と `./to.md` は元ノートと同じディレクトリを指します。`path/to.md`・`./path/to.md`・`../to.md` も元ノート相対、先頭 `/` の `/path/to.md` は Vault ルート基準です。参照リンクの定義と画像・asset も同じ規則で解決し、Vault 全体の basename 探索やルートへのフォールバックは行いません。wikilink と frontmatter の raw path 値は従来どおりです。

従来の Markdown basename / Vault 相対リンクは解釈が変わります。意図する参照先に合わせてリンクを直し、`mdhop build` で索引を全量再生成してください。旧索引は参照・差分更新に利用できません。自動変換や互換モードはありません。

## クイックスタート

```bash
# Vault ディレクトリに移動
cd /path/to/vault

# インデックスを作成（ユーザーのキャッシュ領域に生成される）
mdhop build

# ノートの関連情報を取得
mdhop query --file Notes/Design.md

# タグ起点で探索
mdhop query --tag '#project'

# リンクを解決
mdhop resolve --from Notes/A.md --link '[[B]]'
```

クリックした索引出現を選ぶには `mdhop resolve --from Notes/A.md --link '[[B]]' --line 12` を使います。任意の `--line` は正の整数だけを受け付け、`line_start` に完全一致させます。行番号はファイル先頭を 1 とする元の物理行で frontmatter も数え、表示行・折返し行ではありません。参照リンクは定義行でなく使用行を指定します。escape・alias・embed・fragment・空白を含む索引の raw 原文を変更せず渡してください。行指定時は一致しなければ再解析や別行へのフォールバックをせず、同じ行に異なる解決先・fragment があれば曖昧エラーになります。省略時は既存の完全一致優先とフォールバックを維持します。

呼び出し元は同じ Vault / DB と索引作成時の保存済み本文から元ノート・raw・物理行を取得します。保存後は `build` または適切な `update` / `add` を完了し、位置を取得し直してください。参照定義・表境界の編集も索引更新が必要で、設定・解析規則の変更時は `build` します。位置取得から解決まで本文・索引を変更しないことが利用条件です。未保存 buffer、原文へ逆対応できない renderer、行移動後の古い索引には正しいクリック先を保証しません。行と原文の一致だけでは本文と索引の同一世代を証明できません。本文再読・mtime/hash 検査・自動更新・排他制御や、並行編集・索引更新をまたぐ snapshot 保証は追加しません。主対象は現在索引化できる単一行本文リンクで、frontmatter の座標や複数行文法の保証を広げず、タグ等を含む既存の raw 表現を変えません。

## コマンド一覧

| コマンド | 説明 |
|---------|------|
| `build` | 追加・変更・削除を自動検出して索引を更新（`--full` で全量再生成） |
| `add` | 新規 Markdown ノートをインデックスに追加 |
| `update` | 既存ファイルのインデックスを更新 |
| `set` | frontmatter の単一 key または相対日付を設定しインデックスを更新 |
| `delete` | ファイルをインデックスから削除 |
| `move` | ファイル移動を反映しリンクを更新（frontmatter 由来の移動先テンプレートにも対応） |
| `disambiguate` | 曖昧な basename リンクをフルパスに書き換え |
| `simplify` | 参照先を保ってパスを短縮（Markdown は元ノート相対） |
| `convert` | リンク形式を wikilink ↔ markdown で変換 |
| `repair` | 壊れた・vault 外を指すパスリンクを安全な Vault 内リンクに修復 |
| `resolve` | リンクの解決先を返す |
| `query` | 一つの入口の backlinks / outgoing / twohop を返す |
| `inspect` | 索引済み note 一件の tags / meta と任意 head を返す |
| `search` | frontmatter メタデータ・パス・孤立検出条件で Vault 全体からノートを検索 |
| `reachable` | 入口 note からリンクで到達できる / できない note を列挙 |
| `graph` | リンクグラフを JSON / Graphviz dot で出力 |
| `stats` | ノート数・リンク数などの統計情報 |
| `diagnose` | basename 衝突・phantom ノード・見出し anchor 切れの検出 |
| `meta-check` | frontmatter の path / wikilink 値が実在する対象に解決するか検査 |
| `meta-validate` | frontmatter を必須 key・profiles・`meta.types` 宣言に照らして検査 |
| `migrate` | 旧 YAML 設定と Vault 内索引を既定配置へ移行 |
| `init-meta` | `mdhop.toml` の frontmatter 型定義を生成 |

`add` は大小文字を区別しない `.md` パスのみ受け付けます。非 Markdown パスを含むバッチは変更前に全体を拒否します。asset の登録には `build` を使います。

`--vault <path>`（省略時はカレントディレクトリ）は各コマンドに共通。出力・field 系のフラグはコマンドごとに異なる。

正確なコマンド別フラグ・出力フィールド・使用例は `mdhop <command> --help` を参照。導入済みのバージョンは `mdhop --version` で確認できる。

## Agent Skill の例

最新の Codex / Claude 形式の skill 例は [`examples/skills/mdhop`](examples/skills/mdhop) にある。適切なコマンドを選ぶための薄い agent 入口で、正確なフラグ・出力フィールド・例は `mdhop <command> --help` に寄せている。

```bash
mdhop stats --format json
mdhop search --where "status=active || status=review" --fields meta --format json
mdhop query --file Notes/Design.md --relations backlinks,outgoing --format json
mdhop set --file Notes/Design.md --key reviewed --date today-90d --format json
mdhop move --from Notes/ --to-template "99-Archive/{client|others}/{updated:year}/{basename}" --dry-run --format json
```

## Vault・DB・設定の配置

専用の `migrate` を除き、DB または設定を扱う全コマンドで `--vault`・`--db`・`--config` を使える。`--vault` はカレントディレクトリ、`--db` は `<cache>/mdhop/vaults/<vault-hash>/index.sqlite` が既定。配置引数は絶対パスとカレントディレクトリ基準の相対パスを受け付ける。ノートのパス・リンク解釈・本文プレビュー・出力パス・設定内 glob の基準は常に Vault とし、DB や設定の場所から推定しない。

`<cache>` は空でない絶対パスの `XDG_CACHE_HOME` を使い、未設定・空・相対パスなら対応 OS によらず `~/.cache` を使う。`<vault-hash>` は絶対パス化・symlink 解決後の Vault root の SHA-256 全長・小文字 hex とする。同じ実体への相対パス・symlink は同じ索引を使い、Vault を移動すると別の索引になる。設定はハッシュに含めないため、異なる索引設定を併用する場合は `--db` で保存先を分ける。

`mdhop cache-list`（JSON は `mdhop cache-list --format json`）は Vault の指定なしで、現在の `<cache>/mdhop/vaults/` 直下にある全長 64 桁・小文字 hex の実ディレクトリを、キャッシュの絶対パス昇順で一覧する。各項目はフォルダの絶対パス・現在のディスク使用量・生成元 Vault を返す。text は IEC 単位と、生成元不明の場合は `unknown` を使う。JSON は `cache_path`・丸め前の `size_bytes`・`vault`（不明は null）の配列とし、空一覧は空 text または `[]`。一覧操作はディレクトリ・DB・記録を作成・更新せず、不正・読取不能な記録はエラーにする。

使用量はフォルダ自身と全子孫の file・directory・symlink entry の割当ブロック数 × 512 bytes の合計で、SQLite 補助ファイル・一時ファイル・生成元記録も含む。symlink の参照先は辿らず、hardlink は entry ごとに数える。現在内容を測定するが同時点の snapshot は保証せず、共有 extent や圧縮によって削除時の回収容量とは異なる場合がある。

`--db` を省略した build は、hash に使う canonical Vault root を `{"vault":"/absolute/vault/path"}` として既定索引と同じフォルダの `origin.json` に保存する。`migrate` も記録する。明示 `--db` は既定位置を指定した場合も記録を作成・更新しない。記録のない旧キャッシュは既定再 build まで生成元不明とし、Vault の移動・削除後も生成時のパスを保持する。外部 DB や過去の cache root は追跡しない。キャッシュは再生成可能なので、mdhop が使用していないときにフォルダごと削除し、必要時に build で再生成できる。

キャッシュを削除した場合や Vault を移動した場合は `mdhop build` で再生成する。通常操作は旧 `<vault>/.mdhop/` を利用・移行・削除しない。

`--config` があれば任意名の指定ファイルだけを読む。未指定時は `<vault>/mdhop.toml` だけを読み、既定ファイルがなければ設定なしで動作する。明示ファイルの欠落、読み取り失敗、不正設定は、設定フィルタを無効化していてもエラー。自動検出・併読・マージはしない。`init-meta --write` は選択した設定先を更新し、明示指定ファイルは存在している必要がある。

```sh
mdhop build --vault ./Notes --db ./indexes/notes.sqlite --config ./settings/notes.toml
mdhop query --vault ./Notes --db ./indexes/notes.sqlite --config ./settings/notes.toml --file Index.md
```

外部 DB を使う build・参照系は Vault の書込権限を必要とせず、Vault 内に索引や一時ファイルを作らない。build は選択 DB と同じディレクトリで固有の一時 DB を完成させ、commit と close の成功後に置換する。失敗時は旧 DB を保持する。Vault 内の選択 DB は status の入力とノートの directory mutation 対象から除外し、ノートを削除・移動しても DB 保存先を維持する。選択 DB と直接の補助ファイルは `add` で登録できない。旧版等で登録済みの場合、それらを対象に含む `delete`・`move`（template の実行・計画を含む）は副作用前に操作全体を拒否する。対応する Ubuntu・macOS のローカルファイルシステムでは、接続済み reader は旧索引を保持し、置換後に接続する reader は新索引を見る。再生成に重なる参照も完成済みの索引を見る。複数 CLI 呼び出し間の同一世代や変更中の本文との同時点は保証せず、本文プレビューの stale 検査は維持する。build と update 等の同時書込は未サポートのため直列化する。DB を使わない scan・設定生成は `--db` を受け付けても DB を開かない。

## 旧配置からの移行

`mdhop migrate` を Vault 内で実行するか、`mdhop migrate --vault ./Notes` で対象を指定する。対象は Vault 直下の `mdhop.yaml`・`mdhop.toml`・`.mdhop/` と、その Vault の既定キャッシュ DB に固定され、`--db`・`--config`・位置引数は受け付けない。

YAML があれば全設定を TOML に変換・検証・保存し、その設定で索引を全量再生成する。TOML だけがある場合は内容を変更せず使用し、両方なければ既定設定で再生成する。再生成成功後だけ旧 YAML と `.mdhop/` を削除する。削除予定の YAML は新索引の asset に含めず、リンク先は削除後の状態で解決する。成功時の stdout は空で、警告は stderr に出る。

YAML と TOML が共存する場合は無変更でエラーになる。先に両方を保管して使用する設定を確認し、片方を Vault 外へ移してから再実行する。未知の項目・不正値・複数 YAML 文書・YAML merge key は黙って捨てずエラーにする。固定対象の symlink や、既定キャッシュが削除対象内に入る配置も拒否する。

変換・保存・再生成に失敗した場合は旧 YAML・`.mdhop/`・既存キャッシュ DB を保持し、新しく作成した TOML は削除して戻す。復元に失敗すると残った TOML のパスを報告するため、旧 YAML を保管したうえで生成 TOML を確認・退避して再実行する。後始末に失敗した場合はエラーとなり、新キャッシュは公開済みで、旧ファイルの一部が残る可能性がある。報告された残パスを確認して権限等を直し、TOML と YAML が共存していれば旧 YAML を保管・退避してから再実行する。通常参照は新 TOML と既定キャッシュを使える。build・update・migrate 等の書き込み操作は直列に実行する。

## build の自動差分更新

`mdhop build` は対象一覧と保存済みのファイル別 nanosecond mtime・size を等値比較し、追加・変更・削除を反映する。移動は削除＋追加として扱い、未変更ノートの本文は読み取り・解析しない。参照先の実在化・削除や wikilink / frontmatter raw basename の衝突・root 優先の変化に影響される既存リンクも再解決する。曖昧リンクは全量方式と同様に失敗する。

`mdhop build --full` は既存索引を再利用せず全ノートを読み取り・解析して再生成する。初回、旧形式、必要保存情報の欠落、canonical Vault root または索引に影響する実効設定（`build.exclude_paths`、`meta.link_keys`、metadata 型）の変更、成功した index mutation (`update` / `add` / `delete` / `move` / `set` / `disambiguate` 等) 後は自動的に全量方式へ戻る。設定のコメント・キー順・設定ファイルの場所だけの変更と query / search 専用設定は再生成条件にしない。設定 parse error は旧 DB を残して失敗する。

同一秒でも nanosecond mtime または size が変われば検出し、mtime を保持しても size の変更を検出する。同じ mtime・size に保った編集・復元や、filesystem の時刻分解能以下の同サイズ変更は検出しないため `--full` を使う。`status` は従来の秒精度 mtime 比較を維持し、build の変更検出と一致するとは限らない。

build 中に読み取り前後や最終一覧・署名の確認で検知した入力変動はエラーとなり、旧 DB を保持する。filesystem snapshot や lock は使わず、最終確認後の編集や同一署名への復元の完全検知は保証しない。無変更時も警告と既定 origin の契約を保つ。差分更新の性能比較と条件は [計測記録](cache/build-incremental-performance.md) を参照。

## 設定（mdhop.toml）

Vault 直下に `mdhop.toml` を置くと、build 除外、query の表示・経由先選択、search 除外、frontmatter の扱いを指定できる。

```toml
[build]
exclude_paths = ["daily/*", "templates/*"]

[exclude]
paths = ["daily/*"]
tags = ["#daily"]

[query.hide]
paths = ["archive/*"]

[query.via]
exclude = { paths = [], tags = [] }

[meta]
link_keys = ["related", "sources"]

[meta.types]
date = "date"
priority = { ordered = ["low", "high"] }

[[meta.profiles]]
path = "notes/*"
require = ["date"]
```

設定がない場合や空の場合は既定値を使う。通常のコマンドは旧 `mdhop.yaml` を読み込まず、変換・削除もしない。ノートの YAML frontmatter は引き続き扱う。

## v0.21.0 への移行

- query の `--fields` は削除。関係は `--relations backlinks,outgoing,twohop` で選び、入口の tags / meta / head は `mdhop inspect --file Notes/Design.md --include-head 5 --format json` へ移す。query の head は関連 note、snippet は各関係の根拠行を返す。
- `--max-backlinks` / `--max-twohop` / `--max-via-per-target` は削除し、既定は全件・全経由先。一対一の置換ではなく、必要な関係を一つ選び `--relations backlinks --limit 20 --offset 20` を使う。旧 flags の alias はない。
- query の `--exclude` / `--exclude-tag` / `--no-exclude` は削除。表示は `--hide-path` / `--hide-tag`、twohop の経由先選択は `--via*` / `--exclude-via*`、設定無効化は `--no-config-hide` / `--no-config-via` へ分ける。
- 旧 config `exclude` は `query.via.exclude` **キー不在時だけ**経由先除外へ fallback する。上例の明示空は fallback を無効にする。`query.hide` には流用せず、via include の有無は fallback と独立。CLI 値は no-config 指定時も残る。search の旧 exclude / fields / head 契約は変わらない。
- JSON は `entry`、選択した `backlinks` / `outgoing` / `2hoplink` 配列、`page` を返す。twohop は対象ごとの全 `relation` と `hidden_relation` を持つ。独立 tags、入口 preview / meta、旧 via→targets 構造を読む処理は更新が必要。選択済み空と未選択を区別する。
- tag / asset / phantom 入口の backlinks は維持する。outgoing / twohop は空。query outgoing には親タグも含み、inspect tags は葉タグを返す。

フラグと使用例は `mdhop <command> --help` を参照。

## ドキュメント

- [変更履歴](CHANGELOG.ja.md)
- [目的・要件](docs/requirements.md)
- [設計判断](decisions/)
- [検証手順](docs/verification.md)
- [Versioning・release 手順](docs/release.md)

## ライセンス

[MIT License](LICENSE)

依存ライブラリの著作権表記とライセンス全文は [THIRD-PARTY-NOTICES.txt](THIRD-PARTY-NOTICES.txt) に掲載。バイナリの再配布時は両ファイルを同梱する。[更新・配布手順](docs/licensing.md)を参照。
