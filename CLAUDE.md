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

詳細な仕様は `docs/` にある。新しいコマンドを実装する前に必ず参照すること:
- `docs/external/overview.md` — ユーザー視点のコマンド仕様と挙動（主要リファレンス）
- `docs/architecture/03-data-model.md` — DB スキーマとクエリ設計
- `docs/test-plan.md` — コマンドごとの必要テスト一覧
- `docs/architecture/01-concept.md` — コア思想と設計根拠

## 開発スタイル

### サブエージェント活用

メインコンテキストを汚さないために、skill 以外の場面でもサブエージェント（Task ツール）を積極的に使う。

- 調査・比較・コード探索は Explore サブエージェントに委譲する
- 独立した作業は並列でサブエージェントを起動する
- 1 サブエージェント = 1 タスクに絞り、焦点を明確にする

## プランレビュー

プランモードで実装計画を書き終えたら、ExitPlanMode の前にレビューループを実行する。
**各ステップは前のステップの完了を待ってから実行すること。同時実行は禁止。**

1. `/self-plan-review` を実行する（5観点並列レビュー）
2. **新規の** 🔴 MUST / 🟡 SHOULD の指摘をプランに反映する
3. 新規指摘があった場合 → 手順1に戻る（新規 MUST/SHOULD がゼロになるまでループ）
4. `/codex-plan-review` を実行する（Codex セカンドオピニオン。**2回目以降は `--resume` をつけて呼ぶ**）
5. 指摘があれば反映し、手順1に戻る
6. 指摘なし → ExitPlanMode する

収束判定: 前回対処済みの指摘の再表現（「もっと明示的に」「セクションに切り出せ」等）は新規とみなさない。
判断が必要な指摘は AskUserQuestion でユーザーに確認する。

## 実装レビュー

実装・テストが完了したら、コミット前にレビューループを実行する。
**各ステップは前のステップの完了を待ってから実行すること。同時実行は禁止。**

1. プランから意図的に変更した箇所がある場合、`tmp/plans/` のプランファイルを更新する（該当 Step に変更内容と理由を追記）
2. `/self-impl-review` を実行する（5観点並列レビュー）
3. **新規の** 🔴 MUST / 🟡 SHOULD の指摘を実装に反映する
4. 新規指摘があった場合 → 手順2に戻る（新規 MUST/SHOULD がゼロになるまでループ）
5. `/codex-impl-review` を実行する（Codex セカンドオピニオン。**2回目以降は `--resume` をつけて呼ぶ**）
6. 指摘があれば反映し、手順2に戻る
7. 指摘なし → `/commit` する

収束判定: 前回対処済みの指摘の再表現は新規とみなさない。
判断が必要な指摘は AskUserQuestion でユーザーに確認する。

## ドキュメント管理

- 同じ情報を複数のドキュメントに書かない。各情報の置き場所は1箇所に限定する
- 新しいスキルやファイルを作成したら、同じステップで settings.json 等への登録も行う

技術的な知見・ハマりどころは以下の基準で振り分ける:

- **CLAUDE.md**: 常に意識すべきルール・制約（毎回読み込まれる）
- **docs/knowledge.md**: 特定の状況で役立つ知見（該当する実装のときに読みに行く）

実装前やバグ調査時は `docs/knowledge.md` を確認すること。

## コミット

コミットは `/commit` スキルを使う。Conventional Commits 形式、英語。
`/commit` スキルが ADR → knowledge.md 追記 → コミットメッセージ承認 → コミットの全手順を含む。

## 言語

コミットメッセージは英語（Conventional Commits）。ドキュメントは日本語の場合がある。コード（変数名、コメント）は英語で書く。
