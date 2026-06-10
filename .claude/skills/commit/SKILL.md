---
name: commit
description: Conventional Commits 形式でコミットを作成する
disable-model-invocation: false
---

# Commit スキル

## ICAR

- **Intent**: 変更をレビュー済みの状態で、Conventional Commits 形式の英語コミットメッセージとしてリポジトリに記録する
- **Constraints**:
  - **Conventional Commits 形式に従う**（type は `feat` / `fix` / `docs` / `test` / `refactor` / `chore` / `perf` から選ぶ。scope は Conventional Commits の規約に従って必要なら使う）
  - メッセージ言語は英語、summary は小文字始まり・末尾ピリオドなし・70 文字以内
  - `.env` や credentials を含むファイルはコミット対象から除外する
- **Acceptance**:
  - 該当があれば ADR 作成・`references/knowledge.md` 追記・`backlog/backlog.md` の `[x]` 更新が済んでいる
  - コミットが作成され、`git status` でクリーンな状態を確認済み
- **Relevant**: 下記「コミットメッセージ形式」「Type 一覧」「ADR テンプレート」, `references/knowledge.md`, `backlog/backlog.md`

## コミットメッセージ形式

```
<type>: <summary>

<body（任意）>

Co-Authored-By: Claude <noreply@anthropic.com>
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

## 手順

1. 判断記録（ADR）が必要か判断する（下記「ADR 判断基準」参照。不要ならスキップ）
2. 実装中にハマった点・注意事項があれば `references/knowledge.md` に追記する（該当なければスキップ）
3. `git status` と `git diff`（staged + unstaged）で変更内容を把握する
4. `git log --oneline -5` で直近のコミットスタイルを確認する
5. `backlog/backlog.md` に今回のコミットで完了した項目があれば `[x]` に更新する
6. ファイルを stage してコミットする
7. コミット後 `git status` で結果を確認する

## ADR 判断基準

以下に該当する判断があった場合のみ ADR を作成する:
- 技術選定（ライブラリ、フレームワーク、プロトコル等）
- データモデルやスキーマの設計
- アーキテクチャパターンの選択
- 複数の選択肢から意図的に一つを選んだ判断
- 将来の実装に制約を与える判断

該当あり → `decisions/` に ADR を作成（下記「ADR テンプレート」参照）
該当なし → スキップ

## ADR テンプレート

`decisions/` の既存ファイルを確認し、次の連番を決定する。
判断ごとに1つの ADR を作成（1つの ADR に複数の判断を混ぜない）。
ファイル名: `decisions/NNNN-タイトル.md`（NNNN は0埋め4桁、タイトルはケバブケース）。
連番は再利用しない（Superseded でも番号は残す）。1〜2ページに収める。

```markdown
# ADR NNNN: タイトル（短い名詞句）

## Status

Proposed | Accepted | Deprecated | Superseded by ADR NNNN

## Context

判断が必要になった背景と課題を事実ベースで書く。
技術的・プロジェクト的な力学や制約を含める。

## Considered Options

- **選択肢A**: 概要
- **選択肢B**: 概要
- **選択肢C**: 概要（検討したが却下した案も含める）

## Decision

「We will ...」の形式で、採用した方針を能動態で書く。

## Consequences

この判断によって何が容易になり、何が困難になるか。
肯定的・否定的・中立的な影響をすべて列挙する。
```
