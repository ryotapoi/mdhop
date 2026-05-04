---
name: review-code-mdhop
description: mdhop プロジェクト固有の設計制約に基づく実装レビュー worker。引数 `viewpoint=<name>` で観点を指定する（現状 mdhop のみ）。通常はチェーンスキル `/review-code-all` から呼ばれる。
argument-hint: viewpoint=<name> [<plan-file-path>]
allowed-tools: Read, Glob, Grep, Bash(git diff:*), Bash(git log:*), Bash(git status:*), Bash(mkdir:*), Bash(printf:*), Bash(date:*), Write
context: fork
model: claude-sonnet-4-6
effort: medium
---

# Code Review — mdhop Project Worker

## ICAR

- **Intent**: 引数で指定された 1 観点（viewpoint）で実装差分をプロジェクト固有制約と照合し、結果を `/tmp/claude/claude-review-results/` 配下のファイルに書き出して、戻り値は `RESULT_FILE` と `SUMMARY` の 2 行のみ返す
- **Constraints**:
  - 観点本体（検証手順・チェックリスト）はプロジェクト側 `.claude/skills/review-code-mdhop/viewpoints/<viewpoint>.md` に外出ししてある
  - レビューは fork 自身（sonnet）が直接実行する
  - 結果は `/tmp/claude/claude-review-results/` 配下のファイルに書き出し、text には `RESULT_FILE` と `SUMMARY` の 2 行のみ返す
  - 差分が無い場合は「レビュー対象の変更がありません」と返して終了する
  - 変更ファイルに `.go` が 1 つも含まれていなければ「Go ファイルの変更がないためスキップします」と返して終了する
- **Acceptance**:
  - 結果ファイルに該当観点のレビュー結果が書き出されている
  - text 出力は `RESULT_FILE: <path>` と `SUMMARY: needs_action=<YES|NO> must=<N> should=<N> nit=<N> — <1行サマリ>` の 2 行構成（または `RESULT_FILE: ERROR — <理由>` のフォールバック形式）

## 引数

ARGUMENTS_BEGIN
$ARGUMENTS
ARGUMENTS_END

## 手順

### 1. viewpoint と（あれば）プランパスを抽出する

- `VIEWPOINT`: `viewpoint=<name>` の形で指定される観点名
- `PLAN_PATH`: プランファイルの絶対パス（省略可、バッククォート対応）。あれば `HAS_PLAN=true`
- `PRIOR_REVIEW_BLOCK`: 「前回の続き」「再レビュー」「前回の指摘」のいずれかが含まれていれば再レビューモード

`VIEWPOINT` を抽出できない場合は `RESULT_FILE: ERROR — viewpoint が指定されていません` を返して終了。

### 2. 差分を取得する

- `git diff` と `git diff --cached` を結合して `GIT_DIFF`、`git status` を `FILE_LIST` として保持する
- 差分なしなら「レビュー対象の変更がありません」と返して終了
- `FILE_LIST` に `.go` が 1 つも含まれていなければ「Go ファイルの変更がないためスキップします」と返して終了

### 3. 観点ファイルとプランファイルを読む

- 観点ファイル: `/Users/ryota/Sources/ryotapoi/mdhop/.claude/skills/review-code-mdhop/viewpoints/<VIEWPOINT>.md` を Read で読む。存在しなければエラー終了
- プランファイル: `HAS_PLAN=true` の場合、`PLAN_PATH` を Read で読む

### 4. レビューを実行する

worker 自身（sonnet）が、観点ファイルに従って `GIT_DIFF` をレビューする。

レビュー時に守るルール:

- 観点ファイルの「検証手順」に従って関連ファイルを Read/Glob/Grep する
- 観点ファイルのチェックリストを順に当てて、該当箇所があれば指摘する
- 1 箇所に複数の問題があれば全部出す。指摘はまとめずに別指摘として並べる
- 1 つの編集で複数指摘を解決できる場合でも「理想形」をまとめて書かない
- 実害のある問題の指摘と、確認できた点の LGTM の両方を返す。LGTM のみの出力も正当

### 5. 出力フォーマット

- 日本語、🔴 MUST / 🟡 SHOULD / 🔵 NIT
- 該当するコードの箇所を引用する
- 問題なければ「mdhop 固有の指摘なし」

### 6. 再レビュー時の判定規約

`PRIOR_REVIEW_BLOCK` が空でない場合、✅ / ⚠️ + 新規指摘の 2 区分で出力（`review-plan-mdhop` と同じ規約）。

### 7. 結果ファイルを書き出して返す

1. `mkdir -p /tmp/claude/claude-review-results`
2. `printf '%s/review-code-mdhop-%s-%s-%04x.md' /tmp/claude/claude-review-results "<VIEWPOINT>" "$(date +%Y%m%d-%H%M%S)" "$RANDOM"` で `RESULT_PATH` を組み立て
3. Write で `RESULT_PATH` にレビュー本文を書き出す
4. 集計: 🔴 MUST / 🟡 SHOULD / 🔵 NIT の件数を数える。`needs_action = (must + should > 0) || (⚠️ ≥ 1)`
5. 戻り値:

```
RESULT_FILE: <RESULT_PATH>
SUMMARY: needs_action=<YES|NO> must=<N> should=<N> nit=<N> — <1行サマリ>
```

`<1行サマリ>` は LGTM 時は「mdhop 固有の指摘なし」、指摘ありの場合は最重要指摘の要旨を 1 行で。

#### フォールバック

mkdir / Write のいずれかが失敗した場合:

```
RESULT_FILE: ERROR — <失敗理由を1行で>

<従来形式のレビュー本文>
```

### 8. 結果ファイルの中身

```
## 自己レビュー結果（mdhop, viewpoint=<VIEWPOINT>）

<レビュー本文>
```
