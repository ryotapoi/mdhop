---
name: review-plan-all
description: プランレビューを実行する司令塔。ExitPlanMode を呼ぶ前に、リスクに応じた深さでレビューを通す（L0 self-check / L1 領域固有 / L2 別視点）。プランを書き終えた、レビューに進む、ExitPlanMode する、といった文脈で起動する。個別のレビュースキル（/review-plan, /review-plan-codex 等）を直接呼ばず、このスキルを使う。Small な plan は L0 で済ませてよい。
argument-hint: [plan-file-path]
---

# Plan Review — Risk-Based

プランレビューを、作業のリスクに応じた深さで実行する。
**準備手順と並列レビューは順次実行。同一手順内の個別レビューは並列起動する。**
**明示的に「ユーザーに確認」と記載されたステップ以外は、ユーザー確認なしで自動進行する。**

ユーザーが codex スキップを指示している場合、`/review-plan-codex` を並列リストから除外する。

## Review Depth の選択

最初に main 側で plan の risk profile を判定し、Depth を選ぶ。判定が揺れたら一段深い側に倒す。

- **L0 self-check**: Small 変更（typo、docs、テスト追加だけ、1 ファイルの明確な修正）。main で plan を読み直し、acceptance と照合するだけ。手順 0〜5 はスキップして「self-check 完了。ExitPlanMode で承認を求めてください。」と報告
- **L1 targeted**: 通常の変更。手順 0（split） + 領域固有レビューを実施
- **L2 external**: High-risk、設計判断が重い、曖昧、または L1 で重大な指摘が出た。L1 に加えて Codex を入れる

判定例:
- typo / docs / test 追加だけ → L0
- 通常の機能追加・バグ修正・複数ファイル変更 → L1
- SQLite スキーマ・マイグレーション・`delete --rm` / `move` 等の破壊的処理・SQL 集約意味変更・リンク解決 / ルート優先ルール変更・CLI 破壊的変更・vault escape・並行性・公開 API 削除・外部連携 → L2

## 用語

以下の用語を使う（個別レビュースキルが返す `needs_action=YES/NO` の言い換え）:

- **指摘あり観点**: 前周で `needs_action=YES`（1 件以上の MUST/SHOULD 指摘）を返した観点
- **LGTM 観点**: 前周で `needs_action=NO`（指摘ゼロ）を返した観点

## 手順

### 0. `/review-plan-split` を Skill tool で実行する（L1 / L2）

引数（`$ARGUMENTS`）があればそのまま渡す。プランの粒度を判定する。

戻り値テキストから `^RESULT_FILE: ` / `^SUMMARY: ` 行を抽出する。

- **`SUMMARY: ... needs_action=NO ...`（✅ 分割不要）**: 結果ファイルは Read せず、次の手順 1 に進む
- **`SUMMARY: ... needs_action=YES ...`（⛔ 分割推奨）**: `RESULT_FILE` のパスを Read で読み込み（`/tmp/claude/claude-review-results/` 配下であることを確認）、検出シグナルをユーザーに提示し、`AskUserQuestion` で以下を尋ねる:
  - 選択肢 1: 「backlog でタスク分割する（ExitPlanMode → backlog 更新 → 再計画）」
  - 選択肢 2: 「このまま 1 プランで進める（分割しない理由をプランに明記する）」

  選択肢 1 の場合、以降のレビュー（手順 1 以降）は**全てスキップ**し、ExitPlanMode をユーザーに依頼する旨を伝えて終了する。選択肢 2 の場合は次の手順 1 に進む（プラン本文に「分割しないと判断した理由」を追記してから）。

`RESULT_FILE:` の値が `ERROR` で始まる場合、本文がそのまま戻り値内に含まれているのでフォールバックとして扱う。

split は手順 0 でのみ実施する。再実施ループには戻らない。

### 1. リスクに応じた観点を並列で実施する（1 周目）

Depth と plan の内容に応じて並列起動するスキル + viewpoint を選ぶ。**選んだものを 1 つのメッセージで Skill tool 並列起動** する。

`/review-plan` と `/review-plan-mdhop` は viewpoint 指定 worker。引数は `viewpoint=<name> <PLAN_PATH>` 形式（`<PLAN_PATH>` は `$ARGUMENTS` から抽出した絶対パス。バッククォートで囲ってよい）。

| 呼び出し | 引数 | いつ含めるか |
| --- | --- | --- |
| `/review-plan` | `viewpoint=facts <PLAN_PATH>` | L1 / L2 で常時 |
| `/review-plan` | `viewpoint=design <PLAN_PATH>` | L1 / L2 で常時 |
| `/review-plan` | `viewpoint=go <PLAN_PATH>` | L1 / L2 で常時（Go プロジェクト） |
| `/review-plan-mdhop` | `viewpoint=mdhop <PLAN_PATH>` | L1 / L2 で常時（mdhop 制約は常に確認） |
| `/review-plan-codex` | 自然言語形式（後述） | L2 のみ（codex スキップ指示があれば除外） |

各スキルの戻り値テキストから `^RESULT_FILE: ` 行と `^SUMMARY: ` 行を抽出する。
- `RESULT_FILE:` の値が `ERROR` で始まる場合、本文がそのまま戻り値内に含まれているのでフォールバックとして扱う（戻り値本文を読む）

`/review-plan-codex` は viewpoint 指定なし（codex は別構造）。自然言語で渡す（harness 制約により `--resume` や `--flag` 形式のフラグは `$ARGUMENTS` 全体を空にしてしまう）:

```text
プランファイル <PLAN_PATH> をレビューしてください
```

`<PLAN_PATH>` は呼び出し元から受け取ったプランファイルの絶対パスに置換する。

`/review-plan-codex` の戻り値テキスト 1 行目は `plan mode: ...` のカナリア。続く行から `^RESULT_FILE: ` / `^SUMMARY: ` を抽出する。

### 2. 1 周目の結果を main で統合し、まとめて反映する

**指摘反映の進め方**:

1. 全スキル実行が完了するまで結果ファイルは Read しない（戻り値の `RESULT_FILE` / `SUMMARY` 行のみ受け取る）
2. 全スキル完了後、`SUMMARY: ... needs_action=YES ...` のものについて、`RESULT_FILE` のパスを Read で読み込む
   - パスが `/tmp/claude/claude-review-results/` 配下であることを確認してから Read する（パス検証）
   - `needs_action=NO` のスキルの結果ファイルは Read しない
3. 全指摘を一覧し、🔴 MUST / 🟡 SHOULD 指摘の対応方針を決定する。対応方針の判断は呼び出し元 workflow（`.claude/workflow/plan.md`）に従う
4. 対応方針が決まったら Edit に入る。隣接セクションへの修正は 1 つの Edit にまとめる。離れたセクションへの修正は別 Edit のまま（diff レビュー粒度を保つため）
5. 反映完了後、結果ファイルは再 Read しない（古い Read 結果は履歴から自然に流れる）

**main の責務**: A/A' 判定（同根拠の磨き込みかどうか）は main では行わない。main は受け取った指摘ごとに「採用 / 却下」だけ判断する。A/A' 判定は再レビュー時に fork 側で行う（手順 4 参照）。

判断が必要な指摘は `AskUserQuestion` でユーザーに確認する。

`/review-plan-codex` が `SUMMARY: ... needs_action=YES ...` を返した場合、結果ファイルを Read した上で `/codex-findings-append` を実行する。引数: `plan mdhop "<変更概要>"`。`needs_action=NO` の場合は呼ばない。

### 3. ループ前のハードリミット判定（再実施ループに入る前に必ず実行）

これが何周目のレビューかを main 側で数える（手順 1 が 1 周目、再実施が 2 周目以降）。

3 周目以降に入ろうとしている場合、ユーザーに状況を簡潔に報告して指示を仰ぐ:

- 周回数
- 残っている指摘の要旨
- 同根拠の指摘（前周以前で skip した指摘と why が同じ）が残っているか

報告は状況説明にとどめ、選択肢は提示しない。ユーザーの指示があるまでループに戻らない。

### 4. 部分再実施ループ（2 周目以降）

前周で「指摘あり観点」だった個別レビューのみを **同一ターンで並列起動** する。LGTM 観点はスキップ。

**再レビュー時の引数渡し**: 自然言語で前回指摘 + main 側の対処を渡す。

`/review-plan` `/review-plan-mdhop` の場合は **`viewpoint=<name>` を必ず先頭に置き**、続けて自然言語の再レビュー指示を渡す（worker は `viewpoint=` の抽出と「前回の続き」キーワード検出を両立する）:

```text
viewpoint=<name>
プランファイル `<PLAN_PATH>` を前回の続きで再レビューしてください。
前回の指摘への対処:
- 🔴 <指摘1の要旨> → <対応内容を1行 or 対応しない理由を1行>
- 🟡 <指摘2の要旨> → ...
新規問題と、対処の妥当性を確認してください。
```

`/review-plan-codex` の場合は viewpoint 指定なし（codex は別構造）:

```text
プランファイル <PLAN_PATH> を前回の続きで再レビューしてください。
前回の指摘への対処:
- 🔴 <指摘1の要旨> → ...
- 🟡 <指摘2の要旨> → ...
新規問題と、対処の妥当性を確認してください。
```

引数本文に「Claude 側の見解」を混ぜず、事実（何を変えたか・どの指摘を残したか）だけ渡す。Codex の場合、Codex 自身が前回ログを覚えているので指摘の全文を貼り直す必要はない。**要旨 1 行 + 対処 1 行**で十分。

`/review-plan-codex` の再実施では、前回 Codex レビューの結果ファイルを Read で読んでから引数を組み立てる。**ファイルパスは前回 `/review-plan-codex` の戻り値テキストから抽出した `RESULT_FILE:` の値をそのまま使う**（main の会話履歴に残っている）。タイムスタンプや glob で「直近のもの」を選ぶのは禁止 — 他セッションが並行稼働している時に他人のファイルを掴む。`/tmp/claude/claude-review-results/` 配下であることを確認する。

戻りを全部受け取ったら手順 2 と同じ要領で main で統合・反映する。再実施でも `/review-plan-codex` が `needs_action=YES` を返したら `/codex-findings-append` を呼ぶ。

全観点 LGTM になるまで手順 3 → 4 をループする。3 周目に入る前は必ず手順 3 のハードリミット判定を通す。

### 5. 最終念押し判定（全観点 LGTM 達成時）

全観点が LGTM になった時点で、main は **直近周までの累積修正** を見て「方向性が変わる規模」かを判定する。

直近周の修正規模で最終念押しの要否を決める。

- **大（最終 1 周回す）**: 設計の方向反転、新セクション追加、新しい型・状態の導入、プラン構造を根本的に書き直し
- **中（最終 1 周回す）**: 既存セクション内の規約追加、テストケース追加、文言の段落単位追加
- **小（回さない）**: typo、コメント追加、パス修正、1 行レベルの言い換え

中と小の境界で迷ったら中扱い（最終 1 周回す）に倒す。

「最終 1 周回す」と判定した場合、**直近周で回さなかった観点**（=前周で LGTM だった観点）を 1 度だけ並列で回す。引数は手順 4 と同じ自然言語形式。新規指摘が出たら手順 2 と同じ要領で main で反映し、その指摘あり観点だけをもう一度回す（このとき手順 3 の周回カウントは継続する）。

### 6. 完了

全観点 LGTM かつ最終念押し判定で「回さない」または最終念押しの 1 周も LGTM なら完了。

「プランレビュー完了。ExitPlanMode で承認を求めてください。」と報告する。
