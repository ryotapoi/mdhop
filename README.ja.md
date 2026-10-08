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

既存の Vault を v0.20.0 または v0.21.0 にアップグレードした場合は、`mdhop build` を実行してください。v0.20.0 では参照リンクの定義先を保存し、v0.19.5 で変更された number 型 frontmatter metadata の sort 形式も再生成します。v0.21.0 では表内のリンク文脈と index の解釈 version を保存するため、旧 index の再生成が必要です。

## クイックスタート

```bash
# Vault ディレクトリに移動
cd /path/to/vault

# インデックスを作成（.mdhop/index.sqlite が生成される）
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
| `init-meta` | `mdhop.yaml` の frontmatter 型定義を生成 |

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

## 設定（mdhop.yaml）

Vault 直下に `mdhop.yaml` を置くと、build 除外、query の表示・経由先選択、search 除外、frontmatter の扱いを指定できる。

```yaml
build:
  exclude_paths:
    - "daily/*"
    - "templates/*"

exclude:
  paths:
    - "daily/*"
  tags:
    - "#daily"

query:
  hide:
    paths: ["archive/*"]
  via:
    exclude: {paths: [], tags: []}

meta:
  link_keys:        # raw path 値をリンク edge にする frontmatter key
    - related
    - sources
```

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
