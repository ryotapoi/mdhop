# mdhop

[![Test](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml/badge.svg)](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml)

Markdown リポジトリ内のリンク関係を SQLite にインデックス化する CLI ツール。Obsidian Vault 相当のディレクトリで wikilink / markdown link / tag / frontmatter を解析し、grep に頼らず関連ノートへ辿れるようにする。Coding Agent（Claude Code, Codex 等）と CLI ユーザーの両方で使える。

## 特徴

- **事前解析・即応答** — Vault 全体を SQLite にインデックス化。クエリは数ミリ秒で返る
- **Backlinks / 2-Hop Links / Tags** — 起点ノートから関連情報を一発取得
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
| `delete` | ファイルをインデックスから削除 |
| `move` | ファイル移動を反映しリンクを更新 |
| `disambiguate` | 曖昧な basename リンクをフルパスに書き換え |
| `resolve` | リンクの解決先を返す |
| `query` | 起点ノートの Backlinks / 2-Hop / Tags 等を返す |
| `stats` | ノート数・リンク数などの統計情報 |
| `diagnose` | basename 衝突・phantom ノードの検出 |

共通オプション: `--vault <path>`（省略時はカレントディレクトリ）、`--format json|text`、`--fields <comma-separated>`

各コマンドの詳細は `mdhop <command> --help` を参照。

## 設定（mdhop.yaml）

Vault 直下に `mdhop.yaml` を置くと、build 時・query 時の除外パターンを指定できる。

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
```

## ドキュメント

- [コマンド仕様・挙動の詳細](docs/external/overview.md)
- [ユースケース・使用フロー](docs/external/stories.md)
- [設計思想](docs/architecture/01-concept.md)
- [データモデル](docs/architecture/03-data-model.md)

## ライセンス

[MIT License](LICENSE)
