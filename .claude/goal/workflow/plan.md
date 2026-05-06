# Plan (goal)

## Intent

実装前に、要求・制約・設計判断・検証方針を必要十分な粒度で揃える。

## Use When

`default.md` の Intake で「非 Small」と判定された場合に呼ばれる。具体的には：

- 複数ファイル変更
- 仕様・データモデル・アーキテクチャに影響する変更
- 実装方針が複数あり判断が必要
- リファクタを含む
- High-risk 領域に触れる変更（SQLite スキーマ、SQL 意味変更、リンク解決ロジック、CLI 破壊的変更、vault escape 等）

Small（typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正）は plan を省略してよい。

## No Plan Mode

`EnterPlanMode` / `ExitPlanMode` は使わない。**plan はコンテキスト上で立てる**（ファイルに書かなくてよい）。計画が大きく次フェーズで参照したい場合のみ `backlog/goal-plan.md` に書いてもよい。

ユーザー承認は取らない。ユーザー承認が絶対に必要だと判断した場合は `goal-blocked.md` を作成して停止する。

## Inputs

- goal-loop で選ばれた1タスク
- `backlog/backlog.md` の該当エントリ
- 関連する `rules/`, `specs/`（あれば）, `decisions/`, `references/knowledge.md`
- 関連コードと既存パターン

## UX シナリオ

CLI 出力に関わる変更なら、Before / After / 操作手順を 1 つの具体的な状態でコンテキスト上に整理する。
内部ロジックのみの変更なら「N/A — CLI 出力変更なし」と判断してスキップ。

具体的に：
- 抽象的な仕様ではなく、具体的な1つの状態で書く
- 操作の前後で出力がどう見えるかを明示する
- 複数のシナリオがある場合は主要なものを2-3個

UX シナリオの判断結果がユーザー意図と違いそうな場合は `goal-decisions.md` に記録する。

## 設計判断

- 設計判断の前に `design-principles` スキルを呼ぶ
- ルールに当てはめても決まらないときは自動判断し、ユーザー意図と違う可能性があるものは `goal-decisions.md` に記録する
- モジュール配置（`cmd/mdhop` と `internal/core` の依存方向）、共通化方針、型選択を判断する
- mdhop 固有制約（依存方向、SQL 安全性、`modernc.org/sqlite` の罠、リンク解決ルール、ルート優先ルール、CLI 仕様、vault escape）に触れる場合は意識的に確認する

## Refactor Guard

変更対象に明らかな構造の悪さがある場合のみ、`refactor-guard` でリファクタ要否を判定する。
小さい修正・ロジック追加だけの変更では呼ばない。
判定で「先にリファクタ」となれば、現タスク内に含めるか別タスクに分けるかを判断する。別タスクに分ける場合は goal-loop の B パスに移行する。

## Decision Criteria

- 原則 1 plan = 1 commit。独立した成果が混ざるなら plan を分ける（B パスに移行）
- 設計判断は採用案・却下案・理由をコンテキスト上の plan に記録
- 検証方針（自動 / CLI 動作確認）をコンテキスト上の plan に明記
- ユーザー意図と違う可能性がある判断は `goal-decisions.md` に記録

## Plan Review

Plan を立てたら、self-check または並列レビューで品質確認する。

- **self-check**: plan を省略する Small は plan review 自体スキップ。plan を立てた場合の単純なケースは self-check（plan を読み直して acceptance と照合する）のみ
- **並列レビュー**: 通常はこちら。手順 0〜3 を実施する

### 用語

- **指摘あり観点**: 前周で `needs_action=YES`（1 件以上の MUST/SHOULD 指摘）を返した観点
- **LGTM 観点**: 前周で `needs_action=NO`（指摘ゼロ）を返した観点

### 手順

#### 0. `/review-plan-split`

`/review-plan-split` を Skill tool で実行する。引数はコンテキスト上の plan の要旨を渡す（plan ファイルがある場合はそのパス）。

戻り値テキストから `^RESULT_FILE: ` / `^SUMMARY: ` 行を抽出する。

- **`SUMMARY: ... needs_action=NO ...`（✅ 分割不要）**: 結果ファイルは Read せず、次の手順 1 に進む
- **`SUMMARY: ... needs_action=YES ...`（⛔ 分割推奨）**: `RESULT_FILE` のパスを Read で読み込み（`/tmp/claude/claude-review-results/` 配下であることを確認）、検出シグナルを `goal-decisions.md` に記録した上で、現タスクを分割するか今回1プランで進めるかを自動判断する。
  - 分割する場合: 現ループは A パスを中断し、`backlog/backlog.md` を更新して終了（B パス相当）。`goal-state.md` に「分割のため次ループで A 再開」を書く
  - 分割しない場合: 分割しない理由を `goal-decisions.md` に書き、次の手順 1 に進む

`RESULT_FILE:` の値が `ERROR` で始まる場合、本文がそのまま戻り値内に含まれているのでフォールバックとして扱う。

split は手順 0 でのみ実施する。再実施では呼ばない。

#### 1. 観点を並列で実施する（1 周目）

並列起動するスキル + viewpoint を選ぶ。**1 つのメッセージで Skill tool 並列起動** する。

`/review-plan` と `/review-plan-mdhop` は viewpoint 指定 worker。引数は `viewpoint=<name> <PLAN_PATH>` 形式（plan がコンテキスト上の場合は自然言語で内容を渡す）。

| 呼び出し | 引数 |
| --- | --- |
| `/review-plan` | `viewpoint=facts <PLAN>` |
| `/review-plan` | `viewpoint=design <PLAN>` |
| `/review-plan` | `viewpoint=go <PLAN>` |
| `/review-plan-mdhop` | `viewpoint=mdhop <PLAN>` |

**`/review-plan-codex` は呼ばない**（goal では Codex 系を使わない）。

各スキルの戻り値テキストから `^RESULT_FILE: ` 行と `^SUMMARY: ` 行を抽出する。
- `RESULT_FILE:` の値が `ERROR` で始まる場合、本文がそのまま戻り値内に含まれているのでフォールバックとして扱う

#### 2. 1 周目の結果を統合し、まとめて反映する

1. 全スキル実行が完了するまで結果ファイルは Read しない（戻り値の `RESULT_FILE` / `SUMMARY` 行のみ受け取る）
2. 全スキル完了後、`SUMMARY: ... needs_action=YES ...` のものについて、`RESULT_FILE` のパスを Read で読み込む
   - パスが `/tmp/claude/claude-review-results/` 配下であることを確認してから Read する
   - `needs_action=NO` のスキルの結果ファイルは Read しない
3. 全指摘を一覧し、🔴 MUST / 🟡 SHOULD 指摘の対応方針を自動判断する
4. 対応方針が決まったら plan を更新する（コンテキスト上または `goal-plan.md`）
5. 反映完了後、結果ファイルは再 Read しない

判断が必要な指摘で自動判断できないものは `goal-blocked.md` を作成して停止する。判断したがユーザー意図と違う可能性があるものは `goal-decisions.md` に記録する。

#### 3. 2 周目（最終）

前周で「指摘あり観点」だった個別レビューのみを **同一ターンで並列起動** する。LGTM 観点はスキップ。

**再レビュー時の引数**: 自然言語で前回指摘 + 対処を渡す。

```text
viewpoint=<name>
<plan> を前回の続きで再レビューしてください。
前回の指摘への対処:
- 🔴 <指摘1の要旨> → <対応内容を1行 or 対応しない理由を1行>
- 🟡 <指摘2の要旨> → ...
新規問題と、対処の妥当性を確認してください。
```

戻りを全部受け取ったら手順 2 と同じ要領で反映する。

#### 4. 完了判定

2 周目の結果を見て：

- 全観点 LGTM → 完了。`implement.md` へ進む
- 重大な指摘が残っている → `goal-blocked.md` を作成して停止する

**3 周目以降に入らない**。レビューループは最大2周で終了する。

## Acceptance

- 実装対象、非対象、検証方針が明確
- 必要な仕様・backlog・decision の更新方針が明確
- レビュー指摘への対応が済んでいる、または対応しない理由がコンテキスト上の plan に記録されている

## Stop Conditions

次の場合は `goal-blocked.md` を作成して停止する。

- 仕様・CLI 挙動・設計方針に複数の妥当な選択肢があり自動判断不能
- High-risk 領域に触れる変更で検証方針がない
- レビュー 2 周しても重大な指摘が残る
