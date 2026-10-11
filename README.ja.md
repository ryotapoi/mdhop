# mdhop

[![Test](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml/badge.svg)](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml)

Markdown リポジトリ内のリンク関係を SQLite にインデックス化する CLI ツール。Obsidian Vault 相当のディレクトリで wikilink / markdown link / tag / frontmatter を解析し、grep に頼らず関連ノートへ辿れるようにする。Coding Agent（Claude Code, Codex 等）と CLI ユーザーの両方で使える。

[English](README.md) · 日本語

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

## コマンド一覧

| コマンド | 説明 |
|---------|------|
| `build` | Vault 全体を解析しインデックスを作成 |
| `add` | 新規ファイルをインデックスに追加 |
| `update` | 既存ファイルのインデックスを更新 |
| `set` | frontmatter の単一 key または相対日付を設定しインデックスを更新 |
| `delete` | ファイルをインデックスから削除 |
| `move` | ファイル移動を反映しリンクを更新（frontmatter 由来の移動先テンプレートにも対応） |
| `disambiguate` | 曖昧な basename リンクをフルパスに書き換え |
| `simplify` | 冗長なパスリンクを basename 形式に短縮（disambiguate の逆） |
| `convert` | リンク形式を wikilink ↔ markdown で変換 |
| `repair` | 壊れた・vault 外を指すパスリンクを basename 形式に修復 |
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
| `paths` | 実効的な Vault・設定・索引の配置を表示 |
| `migrate` | 旧 YAML 設定と Vault 内索引を既定配置へ移行 |
| `init-meta` | `mdhop.toml` の frontmatter 型定義を生成 |

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

`mdhop paths --vault <path>`（JSON は `--format json`）で実効的な `vault`・`config`・`db` の絶対パスを確認できる。設定や DB が未作成でも表示でき、ファイルを読み込まず、何も作成しない。キャッシュを削除した場合や Vault を移動した場合は `mdhop build` で再生成する。通常操作は旧 `<vault>/.mdhop/` を利用・移行・削除しない。

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
