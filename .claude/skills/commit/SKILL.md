---
name: commit
description: Conventional Commits 形式でコミットを作成する
disable-model-invocation: false
---

# Commit スキル

Conventional Commits ベースでコミットを作成する。

## コミットメッセージ形式

```
<type>: <summary>

<body（任意）>

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
```

- 言語: **英語**
- summary: 小文字始まり、末尾にピリオド不要、70文字以内
- body: 変更の背景や詳細が必要な場合のみ。箇条書き可

## Type 一覧

| type | 用途 |
|------|------|
| `feat` | 新機能 |
| `fix` | バグ修正 |
| `docs` | ドキュメントのみの変更 |
| `test` | テストのみの変更（プロダクションコード変更なし） |
| `refactor` | 機能変更なしのコード改善 |
| `chore` | ビルド、CI、依存関係、設定などの雑務 |
| `perf` | パフォーマンス改善 |

## Scope

現時点では scope は使わない。パッケージが増えて区別が必要になったら導入する。

## 手順

1. `/adr` を実行する（ADR スキルの判断基準に従い、不要なら作成しない）
2. `/change-note` を実行する（Change Note スキルの判断基準に従い、不要なら作成しない）
3. `git status` と `git diff`（staged + unstaged）で変更内容を把握する
4. `git log --oneline -5` で直近のコミットスタイルを確認する
5. `plans/` にこの実装に対応する plan ファイルがあれば削除する
6. コミットメッセージ案と変更サマリを**ユーザーに提示して承認を得る**
   - ADR / Change Note の作成有無も報告する
7. 承認後、ファイルを stage してコミットする
8. コミット後 `git status` で結果を確認する

## 注意

- コミットメッセージは必ず HEREDOC で渡す（改行の安全な扱いのため）
- `.env` や credentials を含むファイルをコミットしない
- `--amend` はユーザーが明示的に指示した場合のみ
