---
name: review-plan-mdhop
description: mdhop プロジェクト固有の設計制約に基づくプランレビュー worker。引数 `viewpoint=<name>` で観点を指定する（現状 mdhop のみ）。通常は `review-plan-all` から呼ばれる。
argument-hint: viewpoint=<name> `<plan-file-path>`
---

# Plan Review — mdhop Project Worker

## ICAR

- **Intent**: 引数で指定された 1 観点（viewpoint）でプランをプロジェクト固有制約と照合し、結果を `/tmp/Codex/Codex-review-results/` 配下のファイルに書き出して、戻り値は `RESULT_FILE` と `SUMMARY` の 2 行のみ返す
- **Constraints**:
  - 観点本体（検証手順・チェックリスト）はプロジェクト側 `.agents/skills/review-plan-mdhop/viewpoints/<viewpoint>.md` に外出ししてある。worker はこれを Read してレビュー方針として使う
  - レビューはこの skill の実行コンテキストで直接実行する
  - 結果は `/tmp/Codex/Codex-review-results/` 配下のファイルに書き出し、text には `RESULT_FILE` と `SUMMARY` の 2 行のみ返す
- **Acceptance**:
  - 結果ファイルに該当観点のレビュー結果が書き出されている
  - text 出力は `RESULT_FILE: <path>` と `SUMMARY: needs_action=<YES|NO> must=<N> should=<N> nit=<N> — <1行サマリ>` の 2 行構成（または `RESULT_FILE: ERROR — <理由>` のフォールバック形式）
  - `needs_action` は `must + should > 0` のとき `YES`、それ以外は `NO`。再レビュー時に ⚠️（前回対処の問題）が 1 件以上ある場合も `needs_action=YES` 扱いとする（対処側の重要度に揃えて `must` または `should` にカウント）
- **Relevant**:
  - `$ARGUMENTS`: `viewpoint=<name> <plan-path>` 形式の文字列
  - `<repo-root>/.agents/skills/review-plan-mdhop/viewpoints/<name>.md`: 観点本体（worker が Read する）

## 引数

ARGUMENTS_BEGIN
$ARGUMENTS
ARGUMENTS_END

## 手順

### 1. viewpoint とプランパスを抽出する

`$ARGUMENTS` から以下を抽出する:

- **`VIEWPOINT`**: `viewpoint=<name>` の形で指定される観点名
- **`PLAN_PATH`**: プランファイルの絶対パス。バッククォートで囲まれていれば取り除く
- **`PRIOR_REVIEW_BLOCK`**: `$ARGUMENTS` に「前回の続き」「再レビュー」「前回の指摘」のいずれかが含まれていたら再レビューモード

`VIEWPOINT` を抽出できない、または `PLAN_PATH` を抽出できない場合は以下を返して終了する:

- viewpoint なし: `RESULT_FILE: ERROR — viewpoint が指定されていません。例: viewpoint=mdhop \`/path/to/plan.md\``
- プランパスなし: `RESULT_FILE: ERROR — プランファイルパスが指定されていません`

### 2. 観点ファイルとプランファイルを読む

- 観点ファイル: プロジェクトルートの `.agents/skills/review-plan-mdhop/viewpoints/<VIEWPOINT>.md` を Read で読む。プロジェクトルートは `$ARGUMENTS` 内のパスから推定するか、または絶対パスとして `/Users/ryota/Sources/ryotapoi/mdhop/.agents/skills/review-plan-mdhop/viewpoints/<VIEWPOINT>.md` を使う。存在しなければ `RESULT_FILE: ERROR — viewpoint=<VIEWPOINT> に対応する観点ファイルがありません` を返して終了
- プランファイル: `PLAN_PATH` を Read で読む

### 3. レビューを実行する

この skill の実行コンテキストで、観点ファイルに書かれた検証手順と検証観点に従ってプランをレビューする。

レビュー時に守るルール:

- 観点ファイルの「検証手順」に従って関連ファイルを Read/Glob/Grep する（仮定で書かない）
- 観点ファイルの「検証観点」（あるいは設計制約リスト）のチェックリストを順に当てて、該当箇所があれば指摘する
- 1 箇所に複数の問題があれば全部出す。指摘はまとめずに別指摘として並べる
- 1 つの編集で複数指摘を解決できる場合でも「理想形」をまとめて書かない
- 実害のある問題の指摘と、確認できた点の LGTM の両方を返す。LGTM のみの出力も正当

### 4. 出力フォーマット（指摘本文）

- 日本語で出力
- 指摘ごとに重要度を付ける: 🔴 MUST / 🟡 SHOULD / 🔵 NIT
- 該当する仕様書・コードの箇所を引用する
- 問題がなければ「mdhop 固有の指摘なし」と記載する

### 5. 再レビュー時の判定規約

`PRIOR_REVIEW_BLOCK` が空でない場合、以下の 2 区分で出力する:

**前回指摘への対処レビュー:**
- ✅ <指摘要旨>: 対処適切 / LGTM
- ⚠️ <指摘要旨>: 対処の問題: <具体的に>

**新規指摘:**
- 🔴/🟡/🔵 <指摘要旨>: ...

対処レビュー（✅/⚠️）は前回指摘 1 件につき 1 行返す。⚠️ は対処に明確な問題がある場合のみ。新規指摘は前回指摘で触れていない別観点のみ。前回指摘の言い換え・派生・磨き込みは ✅ 扱い。

### 6. 結果ファイルを書き出して返り値を返す

1. `mkdir -p /tmp/Codex/Codex-review-results`
2. ファイル名を組み立てる: `printf '%s/review-plan-mdhop-%s-%s-%04x.md' /tmp/Codex/Codex-review-results "<VIEWPOINT>" "$(date +%Y%m%d-%H%M%S)" "$RANDOM"`。出力を `RESULT_PATH` とする
3. `RESULT_PATH` にレビュー本文を書き出す（フォーマットは下の「結果ファイルの中身」参照）
4. 集計: 🔴 MUST / 🟡 SHOULD / 🔵 NIT の件数を数える。`needs_action = (must + should > 0) || (⚠️ ≥ 1)`
5. ユーザーに返す text:

```
RESULT_FILE: <RESULT_PATH>
SUMMARY: needs_action=<YES|NO> must=<N> should=<N> nit=<N> — <1行サマリ>
```

`<1行サマリ>` は LGTM 時は「mdhop 固有の指摘なし」、指摘ありの場合は最重要指摘の要旨を 1 行で。

#### フォールバック

結果ファイルの作成に失敗した場合:

```
RESULT_FILE: ERROR — <失敗理由を1行で>

<従来形式のレビュー本文>
```

### 7. 結果ファイルの中身

```
## 自己レビュー結果（mdhop, viewpoint=<VIEWPOINT>）

<レビュー本文>
```
