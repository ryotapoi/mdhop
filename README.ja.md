# mdhop

[![Test](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml/badge.svg)](https://github.com/ryotapoi/mdhop/actions/workflows/test.yml)

Markdown リポジトリ内のリンク関係を SQLite にインデックス化する CLI ツール。Obsidian Vault 相当のディレクトリで wikilink / markdown link / tag / frontmatter を解析し、grep に頼らず関連ノートへ辿れるようにする。Coding Agent（Claude Code, Codex 等）と CLI ユーザーの両方で使える。

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
| `delete` | ファイルをインデックスから削除 |
| `move` | ファイル移動を反映しリンクを更新 |
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
| `meta-validate` | frontmatter を必須 key と `meta.types` 宣言に照らして検査 |
| `init-meta` | `mdhop.yaml` の frontmatter 型定義を生成 |

共通オプション: `--vault <path>`（省略時はカレントディレクトリ）、`--format json|text`、`--fields <comma-separated>`

各コマンドの詳細は `mdhop <command> --help` を参照。

## Agent Skill の例

最新の Codex / Claude 形式の skill 例は [`examples/skills/mdhop`](examples/skills/mdhop) にある。構造的なノート探索、メタデータフィルタ、Vault 全体検索、ファイル操作 workflow をまとめている。

最近追加された例（path filter、到達性チェック、グラフ出力、frontmatter 検査、範囲指定 repair）:

```bash
# frontmatter key を持たないノート
mdhop search --where "priority NOT EXISTS" --format json

# 相対日付比較で更新の古いノート（stale 候補）
mdhop search --where "updated<today-90d" --format json

# 行数が多い順。computed fields と meta key を出力
mdhop search --sort -lines --limit 10 --fields lines,outgoing_count,meta.status --format json

# 見出し anchor 切れ検出を有効化（--fields anchors は anchors のみ出力）
mdhop diagnose --path "projects/*" --fields anchors --format json

# frontmatter の参照値が解決するかを検査（末尾 / のディレクトリ参照も対応）
mdhop meta-check --key sources --kind path --format json

# 指定範囲の source note だけを repair 対象として preview
mdhop repair --path "docs/*" --dry-run --format json

# frontmatter が schema に準拠するかを検査
mdhop meta-validate --require status --require updated --format json

# 入口 note から到達できる / できない note と最短経路
mdhop reachable --from index.md --path "docs/*" --route --format json

# 可視化用にリンクグラフを出力
mdhop graph --path "docs/*" --format dot
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

- [コマンド仕様・挙動の詳細](rules/overview.md)
- [ユースケース・使用フロー](derived/stories.md)
- [設計思想](rules/01-concept.md)
- [データモデル](rules/03-data-model.md)

## ライセンス

[MIT License](LICENSE)
