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
- `cmd/mdhop/` — CLI エントリポイント（未実装）。
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

詳細な仕様は `docs/` にある。新しいコマンドを実装する前に必ず参照すること:
- `docs/external/overview.md` — ユーザー視点のコマンド仕様と挙動（主要リファレンス）
- `docs/architecture/03-data-model.md` — DB スキーマとクエリ設計
- `docs/test-plan.md` — コマンドごとの必要テスト一覧
- `docs/architecture/01-concept.md` — コア思想と設計根拠

## プランレビュー

プランモードで実装計画を書き終えたら、ExitPlanMode の前に以下の順序でレビューを実行する:

1. `/self-plan-review` を実行する（Claude 自身による4観点並列レビュー）
2. レビュー結果をプランに反映する
3. `/codex-plan-review` を実行する（Codex によるセカンドオピニオン）
4. 指摘を反映して ExitPlanMode する

レビュー結果の処理:
- 解決可能な指摘（🔴 MUST / 🟡 SHOULD）はプランに反映する
- 判断が必要な指摘は AskUserQuestion でユーザーに確認する

## 実装レビュー

実装・テストが完了したら、コミット前に以下の順序でレビューを実行する:

1. `/self-impl-review` を実行する（Claude 自身による4観点並列レビュー）
2. 🔴 MUST / 🟡 SHOULD の指摘は実装に反映する
3. `/codex-impl-review` を実行する（Codex によるセカンドオピニオン）
4. 指摘を反映する
5. 反映が完了してから `/commit` する

判断が必要な指摘は AskUserQuestion でユーザーに確認する。

## Codex MCP 連携

Codex（GPT）が MCP ツール（`mcp__codex__codex`, `mcp__codex__codex-reply`）で利用可能。

使う場面:
- ユーザーに「Codex も使って」と言われたとき
- デバッグで行き詰まったとき（同じアプローチを2回以上失敗したら検討）
- 別の視点が欲しいとき

状況に応じたスキルを呼ぶ:
- `/codex-debug-session` — デバッグでセカンドオピニオンが欲しいとき
- `/codex-consult` — 設計・実装の相談がしたいとき

## コミット

コミットは `/commit` スキルを使う。Conventional Commits 形式、英語。
`/commit` スキルが ADR → change-note → コミットメッセージ承認 → コミットの全手順を含む。

## 言語

コミットメッセージは英語（Conventional Commits）。ドキュメントは日本語の場合がある。コード（変数名、コメント）は英語で書く。
