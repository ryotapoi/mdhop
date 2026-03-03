# CLAUDE.md

このファイルは Claude Code (claude.ai/code) がこのリポジトリで作業する際のガイダンスを提供します。

## プロジェクト概要

mdhop は Coding Agent（Claude Code, Codex）向けの CLI ツール。Obsidian Vault 相当の Markdown リポジトリ内のリンク関係（wikilink / markdown link / tag / frontmatter）を事前解析して SQLite にインデックス化し、grep に頼らず関連ノートへ辿れるようにする。

## ビルド・テストコマンド

```bash
go test ./...                          # 全テスト実行
go test ./internal/core/               # 特定パッケージのテスト実行
go test ./internal/core/ -run TestBuild # 特定テストのみ実行
go build -o bin/mdhop ./cmd/mdhop      # バイナリビルド
```

## アーキテクチャ

### 設計方針

- **厳密モード**（デフォルト）: basename が重複するリンク（曖昧リンク）はエラー。basename 衝突（同名ファイルの複数存在）自体はエラーにしない。曖昧な*リンク*のみ拒否する。
- **basename 比較は case-insensitive**: `[[note]]` は `Note.md` にマッチする。
- **アトミックな DB 作成**: `.tmp` ファイルに書き込み後 `os.Rename` で本番パスへ移動。失敗時は既存 DB を保持する。
- **DB に本文テキストは保存しない**: 位置情報（行番号）のみ保持し、スニペットはクエリ時にファイルから読み出す。

### パッケージ構成

- `internal/core/` — コアロジック（build, parse, DB スキーマ, ユーティリティ）。現在の実装はすべてここにある。
- `internal/testutil/` — テストヘルパー（`CopyDir` でテスト用 Vault を一時ディレクトリにコピーする等）。
- `cmd/mdhop/` — CLI エントリポイント。サブコマンドごとにファイル分割。
- `testdata/` — テストで使用する Vault フィクスチャ。各 `vault_*` ディレクトリが独立したテストシナリオ。

### `internal/core/` の主要ファイル

- `build.go` — `Build()` エントリポイント: `.md` ファイル収集、曖昧リンク検出、SQLite DB にノード/エッジを作成。
- `parse.go` — リンクパーサー: wikilink (`[[...]]`) と markdown link (`[...](...)`) を抽出。コードフェンス・インラインコードを除外。
- `db.go` — SQLite スキーマ定義（`nodes` + `edges` テーブル）、DB パス管理、upsert 関数。
- `util.go` — パス正規化（`normalizePath`）と basename 抽出。

### データモデル（SQLite）

`.mdhop/index.sqlite` に2テーブル:
- **nodes**: `id, node_key (UNIQUE), type (note|phantom|tag|url), name, path, exists_flag, mtime`
- **edges**: `id, source_id, target_id, link_type (wikilink|markdown|tag|frontmatter|url), raw_link, subpath, line_start, line_end`

ノードキーの形式: note の場合 `note:path:<Vault相対パス>`

### テストパターン

テストは `testdata/` の Vault フィクスチャを `testutil.CopyDir` で `t.TempDir()` にコピーしてから操作する。これによりテストが分離され、共有フィクスチャを変更しない。

## 仕様ドキュメント

詳細な仕様は `rules/` にある。新しいコマンドを実装する前に必ず参照すること:
- `rules/overview.md` — ユーザー視点のコマンド仕様と挙動（主要リファレンス）
- `rules/03-data-model.md` — DB スキーマとクエリ設計
- `derived/test-plan.md` — コマンドごとの必要テスト一覧
- `rules/01-concept.md` — コア思想と設計根拠

## ドキュメント構成

| ディレクトリ | 内容 |
|---|---|
| `rules/` | 方針・スコープ・拘束的制約 |
| `decisions/` | ADR |
| `derived/` | 派生ビュー（stories, test-plan 等） |
| `references/` | 技術的知見（knowledge.md） |
| `backlog/` | バックログ・実装計画 |

## 開発スタイル

### サブエージェント活用

メインコンテキストを汚さないために、skill 以外の場面でもサブエージェント（Task ツール）を積極的に使う。

- 調査・比較・コード探索は Explore サブエージェントに委譲する
- 独立した作業は並列でサブエージェントを起動する
- 1 サブエージェント = 1 タスクに絞り、焦点を明確にする

## 開発ワークフロー

IMPORTANT: 以下のフローを必ずこの順番で実行すること。ステップを飛ばしてはならない。

### Step 1: 計画（プランモード）

EnterPlanMode でプランを作成する。

### Step 2: プランレビュー

プランの記述が完了したら、**ExitPlanMode を直接呼んではならない**。
必ず先に `/review-plan-all` スキルを Skill ツールで実行する。
レビュー完了後に ExitPlanMode を呼ぶ。

### Step 3: 実装

プラン承認後、`references/knowledge.md` を事前確認してから実装・テストを行う。

### Step 4: 実装レビュー

実装・テストが完了したら、**コミットしてはならない**。
必ず先に `/review-code-all` スキルを Skill ツールで実行する。

### Step 5: コミット

レビュー完了後、`/commit` スキルでコミットする。

### Codex 指摘の蓄積

`/review-plan-codex` や `/review-code-codex` で MUST / SHOULD の指摘が出たら、`tmp/codex-findings.md` に追記する。

追記形式:
```
## <セッションで何を実装したかの1行要約>
- [plan|impl] 🔴/🟡 <指摘内容の要約>
  - self で防げたか: Yes/No
  - スコープ: project / common
  - 詳細: <具体的な指摘を1-2行で>
```

20〜30件溜まったら一括分類し、skill への反映を検討する。

## ドキュメント管理

- 同じ情報を複数のドキュメントに書かない。各情報の置き場所は1箇所に限定する
- 新しいスキルやファイルを作成したら、同じステップで settings.json 等への登録も行う

技術的な知見・ハマりどころは以下の基準で振り分ける:

- **CLAUDE.md**: 常に意識すべきルール・制約（毎回読み込まれる）
- **references/knowledge.md**: 特定の状況で役立つ知見（該当する実装のときに読みに行く）

実装前やバグ調査時は `references/knowledge.md` を確認すること。

## 言語

コミットメッセージは英語（Conventional Commits）。ドキュメントは日本語の場合がある。コード（変数名、コメント）は英語で書く。
