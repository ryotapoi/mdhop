---
name: review-code-all
description: 実装レビューの全チェーンを実行する。実装・テスト完了後、コミット前に必ず実行すること。実装が終わった、レビューに進む、コミットする、といった文脈で自動的にこのスキルを起動する。個別のレビュースキル（/review-code, /review-code-go, /review-code-codex 等）を直接呼ばず、このスキルを使う。
argument-hint: [plan-file-path]
---

# Implementation Review — Full Chain

実装レビューの全ステップを順次実行し、指摘の反映ループを回す。
**各ステップは前のステップの完了を待ってから実行すること。同時実行は禁止。**
**明示的に「ユーザーに確認」と記載されたステップ以外は、ユーザー確認なしで次のステップへ自動的に進む。**

ユーザーが codex スキップを指示している場合、手順9-10をスキップする。

## 手順

### 0. ビルドとテストを通す

```bash
go build ./...
go test ./...
```

失敗したら修正してから次へ進む。

### 1. diff サイズを確認する

`git diff --stat` で変更行数を確認する。合計が 1000 行を超える場合、AskUserQuestion で「diff が大きいため Codex レビュー（/review-code-codex）でレート制限に当たる可能性があります。Codex レビューも実行しますか？」と確認する。ユーザーが No なら手順9（/review-code-codex）をスキップして手順12に進む。

### 2. プランファイルの更新（必要な場合のみ）

プランから意図的に変更した箇所がある場合、`backlog/plans/` のプランファイルを更新する（該当 Step に変更内容と理由を追記）。

### 3. `/simplify` を Skill tool で実行する

結果を簡潔に報告したら、ユーザー確認を待たずに次の手順へ自動的に進む。

### 4. `/review-code` を Skill tool で実行する

引数（`$ARGUMENTS`）があればそのまま渡す。

### 5. `/review-code-go` を Skill tool で実行する

引数があればそのまま渡す。

### 6. 新規の 🔴 MUST / 🟡 SHOULD 指摘を実装に反映する

- 前回対処済みの指摘の再表現は新規とみなさない
- 判断が必要な指摘は AskUserQuestion でユーザーに確認する

### 7. 新規指摘があった場合 → 手順4に戻る

新規 MUST/SHOULD がゼロになるまでループする。

### 8. `/review-code-mdhop` を Skill tool で実行する

引数（`$ARGUMENTS`）があればそのまま渡す。新規 MUST/SHOULD 指摘があれば反映し、手順4に戻る。

### 9. `/review-code-codex` を Skill tool で実行する

**引数は自然言語で渡すこと**（harness 制約により `--resume` や `--flag` 形式のフラグは `$ARGUMENTS` 全体を空にしてしまう）:

- 初回（プランあり）: `args: "プランファイル <PLAN_PATH> に対する実装をレビューしてください"`
- 初回（プランなし）: `args: "プランなしで git diff をレビューしてください"`
- 2 回目以降: `args: "<上記初回文> を前回の続きで再レビューしてください"`

手順1で Codex スキップが決定している場合、このステップを飛ばして手順12に進む。

Codex の出力に 🔴 MUST / 🟡 SHOULD / 🔵 NIT の指摘がある場合（LGTM でない場合）、指摘を `~/.local/share/codex-findings/codex-findings.md` に追記する。

**IMPORTANT: 必ず Edit ツール（末尾追記）を使うこと。Write ツールは既存内容を上書きするため絶対に使わない。**

```markdown
## YYYY-MM-DD impl [mdhop]: <変更の概要（1行）>

- 🔴/🟡/🔵 指摘内容の要約（1行）
  - スコープ: common / domain / project
  - self: Yes / No
- ...
```

### 10. 新規指摘があれば反映し、手順4に戻る

Codex レビューが2回目の場合、自動で反映せず指摘内容をユーザーに提示する。ユーザーがさらにループするか終了するか判断する。

### 11. 新規指摘なし → 以降の完了ステップへ進む

### 12. 手動確認（必要な場合のみ）

UI や操作感など自動テストではカバーできない変更がある場合、手動確認の手順をユーザーに提示してレスポンスを待つ。
自動テストで十分にカバーできている変更のみの場合はスキップする。

### 13. `/commit` を Skill tool で実行する
