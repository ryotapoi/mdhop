# mdhop

[![Test](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml/badge.svg)](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml)

Markdown リポジトリ内のリンク関係を SQLite にインデックス化する CLI ツール。Obsidian Vault 相当のディレクトリで wikilink / markdown link / tag / frontmatter を解析し、grep に頼らず関連ノートへ辿れるようにする。Coding Agent（Claude Code, Codex 等）と CLI ユーザーの両方で使える。

[English README](README.md) · [English changelog](CHANGELOG.md) · [日本語の変更履歴](CHANGELOG.ja.md)

## 特徴

- **事前解析・即応答** — Vault 全体を SQLite にインデックス化。クエリは数ミリ秒で返る
- **Backlinks / Two-Hop Links / Tags** — 起点ノートから関連情報を一発取得
- **wikilink / markdown link / tag / frontmatter 対応** — Obsidian 互換のリンク解釈
- **ローカル完結** — 外部サービス不要。pure Go + SQLite
- **Coding Agent 向け最適化** — `--fields` や `--include-snippet` で必要最小限のコンテキストだけ返す

## インストール

```bash
go install github.com/ryotapoi/mdhop/cmd/mdhop@latest
```

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
| `query` | 起点ノートの Backlinks / Two-Hop / Tags 等を返す |
| `search` | frontmatter メタデータ・パス・孤立検出条件で Vault 全体からノートを検索 |
| `reachable` | 入口 note からリンクで到達できる / できない note を列挙 |
| `graph` | リンクグラフを JSON / Graphviz dot で出力 |
| `stats` | ノート数・リンク数などの統計情報 |
| `diagnose` | basename 衝突・phantom ノード・見出し anchor 切れの検出 |
| `meta-check` | frontmatter の path / wikilink 値が実在する対象に解決するか検査 |
| `meta-validate` | frontmatter を必須 key・profiles・`meta.types` 宣言に照らして検査 |
| `init-meta` | `mdhop.yaml` の frontmatter 型定義を生成 |

共通オプション: `--vault <path>`（省略時はカレントディレクトリ）、`--format json|text`、`--fields <comma-separated>`

各コマンドの詳細は `mdhop <command> --help` を参照。

## Agent Skill の例

最新の Codex / Claude 形式の skill 例は [`examples/skills/mdhop`](examples/skills/mdhop) にある。適切なコマンドを選ぶための薄い agent 入口で、正確なフラグ・出力フィールド・例は `mdhop <command> --help` に寄せている。

```bash
mdhop stats --format json
mdhop search --where "status=active || status=review" --fields meta --format json
mdhop query --file Notes/Design.md --fields backlinks,outgoing --format json
mdhop set --file Notes/Design.md --key reviewed --date today-90d --format json
mdhop move --from Notes/ --to-template "99-Archive/{client|others}/{updated:year}/{basename}" --dry-run --format json
```

## 設定（mdhop.yaml）

Vault 直下に `mdhop.yaml` を置くと、build 時・query 時の除外パターンと frontmatter の扱いを指定できる。

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

meta:
  link_keys:        # raw path 値をリンク edge にする frontmatter key
    - related
    - sources
```

## ドキュメント

- [コマンド仕様・挙動の詳細](docs/specs/overview.md)
- [ユースケース・使用フロー](docs/specs/stories.md)
- [設計思想](docs/rules/01-concept.md)
- [データモデル](docs/rules/03-data-model.md)

## ライセンス

[MIT License](LICENSE)
