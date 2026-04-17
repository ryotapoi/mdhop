---
name: review-plan-all
description: プランレビューの全チェーンを実行する。プランの記述が完了したら、ExitPlanMode を呼ぶ前に必ず実行すること。プランを書き終えた、レビューに進む、ExitPlanMode する、といった文脈で自動的にこのスキルを起動する。個別のレビュースキル（/review-plan, /review-plan-codex 等）を直接呼ばず、このスキルを使う。
argument-hint: [plan-file-path]
---

# Plan Review — Full Chain

プランレビューの全ステップを順次実行し、指摘の反映ループを回す。
**各ステップは前のステップの完了を待ってから実行すること。同時実行は禁止。**
**明示的に「ユーザーに確認」と記載されたステップ以外は、ユーザー確認なしで次のステップへ自動的に進む。**

ユーザーが codex スキップを指示している場合、手順5-6をスキップする。

## 手順

### 1. `/review-plan` を Skill tool で実行する

引数（`$ARGUMENTS`）があればそのまま渡す。

### 2. 新規の 🔴 MUST / 🟡 SHOULD 指摘をプランに反映する

- 前回対処済みの指摘の再表現（「もっと明示的に」「セクションに切り出せ」等）は新規とみなさない
- 判断が必要な指摘は AskUserQuestion でユーザーに確認する

### 3. 新規指摘があった場合 → 手順1に戻る

新規 MUST/SHOULD がゼロになるまでループする。

### 4. `/review-plan-mdhop` を Skill tool で実行する

引数（`$ARGUMENTS`）があればそのまま渡す。新規 MUST/SHOULD 指摘があれば反映し、手順1に戻る。

### 5. `/review-plan-codex` を Skill tool で実行する

**引数は自然言語で渡すこと**（harness 制約により `--resume` や `--flag` 形式のフラグは `$ARGUMENTS` 全体を空にしてしまう）:

- 初回: `args: "プランファイル <PLAN_PATH> をレビューしてください"`
- 2 回目以降: `args: "プランファイル <PLAN_PATH> を前回の続きで再レビューしてください"`

`<PLAN_PATH>` は手順 1 で受け取ったプランファイルの絶対パスに置換する。

Codex の出力に 🔴 MUST / 🟡 SHOULD / 🔵 NIT の指摘がある場合（LGTM でない場合）、指摘を `~/.local/share/codex-findings/codex-findings.md` に追記する。

**IMPORTANT: 必ず Edit ツール（末尾追記）を使うこと。Write ツールは既存内容を上書きするため絶対に使わない。**

```markdown
## YYYY-MM-DD plan [mdhop]: <変更の概要（1行）>

- 🔴/🟡/🔵 指摘内容の要約（1行）
  - スコープ: common / domain / project
  - self: Yes / No
- ...
```

### 6. 新規指摘があれば反映し、手順1に戻る

Codex レビューが2回目の場合、自動で反映せず指摘内容をユーザーに提示する。ユーザーがさらにループするか終了するか判断する。

### 7. 新規指摘なし → 完了

「プランレビュー完了。ExitPlanMode で承認を求めてください。」と報告する。
