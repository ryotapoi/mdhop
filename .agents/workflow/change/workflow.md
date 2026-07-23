# Change Workflow

この workflow は、単発の実装依頼、または Goal 内で切り出された 1 commit 分の作業を完了させるための手順。
Goal 経由の場合は `goal-workflow` skill を入口とし、各 commit でこの workflow を繰り返す。単発依頼の場合はこの workflow を直接の入口とする。

## ICAR

- **Intent**: 単発依頼、または Goal 内で切り出された 1 commit 分の作業を、必要十分な調査・計画・実装・検証・記録で完了させる。
- **Constraints**:
  - Goal 経由では Conductor → Implementer → Gatekeeper → Conductor と担当する。Implementer は plan・実装・検証だけ、Gatekeeper は full diff・brief / plan・test 再実行・review lane・acceptance だけ、Conductor は機械照合と commit だけを担う。
  - workflow は 1 つの commit 単位で回す。1 commit に独立した複数作業を混ぜない。Goal が複数 commit 単位を含む場合は、`goal-workflow` skill に戻って 1 commit に収まる単位へ切り直す。
  - 手続きの重さは作業の大きさとリスクに合わせる。
  - 判断に影響する `docs/rules/`, `docs/specs/`, `backlog/backlog.md`, `docs/decisions/`, `llm-wiki/`（作業地図）は推測で済ませず実物を確認する。
  - 仕様・UX の不明点は、現在の要求、正本、既存コード、調査・検証結果から採用案を選んで進める。可逆で影響が小さい選択は、Product Decision Ledger の対象なら ledger に残す。複数の妥当案が残り、かつ選択が非可逆（データ保持・削除・マイグレーション・外部公開契約）またはやり直しコストが大きい場合、または正本と矛盾する場合は Stop Conditions に従う。
  - Product Decision Ledger の対象・Alternative Check・報告基準（対象は振る舞い仕様が変わる判断。定義は同ファイル）は `.agents/workflow/design-decision-record.md` を唯一の正本とする。
- **Acceptance**:
  - ユーザーの要求が満たされている。
  - 必要な情報源が同期されている。
  - 選んだ検証とレビューの深さを説明できる。
  - コミット済み、またはユーザーが明示的にコミット不要とした状態。
- **Relevant**:
  - `goal-workflow` skill
  - `.agents/workflow/change/investigate.md`
  - `.agents/workflow/change/plan.md`
  - `.agents/workflow/change/implement.md`
  - `.agents/workflow/change/verify.md`
  - `.agents/workflow/change/review.md`
  - `.agents/workflow/change/finish.md`
  - `.agents/workflow/design-decision-record.md`

## Intake

最初に作業を分類する。判定が揺れたら High-risk 寄りに倒す。Small / Normal の境界は迷ったら Normal で進めてよい。

- **Small**: typo、文書、テスト期待値、1 ファイルの明確な修正
- **Normal**: 通常の機能追加・バグ修正・複数ファイル変更
- **High-risk**: データ永続化、マイグレーション、並行性、公開 API、削除、広い UI 挙動、外部連携
- **Exploratory**: 原因不明、仕様不明、技術検証が先に必要

## Flow

- Exploratory → `change/investigate.md` で事実を揃えてから Intake をやり直す。
- Goal で選ばれた今回扱う 1 commit 分だけを進める。
- 実行中に 1 commit として不自然だと分かった場合 → 作業を広げず、Goal 実行中は `goal-workflow` skill に戻って commit 単位を切り直す。単発 Change では今回扱う単位を切り直す。
- Plan が必要な変更 → `change/plan.md` で実装前の ICAR を揃える。
- Plan 省略可な変更 → `change/implement.md` へ進み、局所 ICAR を満たす。
- 実装 → `change/implement.md`（Goal では Implementer）
- 検証 → `change/verify.md`（Implementer 後に Gatekeeper が再実行）
- レビュー → `change/review.md`（Normal 以上は Gatekeeper が差配・採否。Small は Conductor の限定 self-check）
- 完了 → `change/finish.md`（Goal の commit は Conductor）

## Source Resolution

現在のユーザー依頼は作業の目的を定める。`docs/rules/`、`docs/decisions/`、`docs/specs/`、tests は正本と根拠として照合する。矛盾した場合は依頼を理由に正本を黙って上書きせず、Stop Conditions に従ってどの情報源が古いかを確定してから同期する。

## Execution Notes

互いに独立した read-only 調査・レビューは並列化してよい。同一 worktree の実装 writer は 1 体に限る。worker は会話履歴を引き継がない fresh な subagent とし、探索は `scout` とする。running agent には追加連絡し、completed / idle agent は同じ agent を再開する。領域固有の判断は各 phase の workflow に従って skill を使う。

fresh は現行 subagent ツールの `fork_turns: "none"` を明示指定して実現する（既定に任せない）。

既存 worktree 差分向けの特別な snapshot / staging / clean check フローは作らない。通常の差分確認と commit discipline で巻き込みを防ぐ。

Change 中の一時 artifact（plan file、列挙メモ、検証用 checkout、review lane のログ等）は `tmp/` 直下に平置きせず、`tmp/workflow/<scope>/` 配下に置く。`<scope>` は短い slug（backlog の版番号やタスクの短縮名。例: `v0110_1`）。Goal 経由では Conductor が Goal 開始時に 1 回決めて Change brief で全 Change に同じ値を渡し、Implementer / Gatekeeper は決め直さない（brief に置き場がなければ change 識別子で代用し、停止しない）。単発 Change では現在の agent が change の短い識別子を使う。`tmp/` 直下は人が手で置くファイルに残し、掃除は scope フォルダ単位でまとめて消せる状態を保つ。

Product Decision Ledger は新しい正本ではない。Goal、長い Change、委任、review 指摘対応をまたぐ判断候補がある場合は、必要に応じて `tmp/workflow/<scope>/product-decision-ledger.md` に残す。finish では記憶ではなく ledger、review 結果、同期済み docs から `ユーザー判断が必要` を判断する。

横断のスコープ判定は `boundary-control` を正本とし、全 phase に適用する。隣接作業は現在の commit に広げず、project-relevant なら workflow が認める正本へ capture するか最終報告で report する。

## Phase Handoff

固定テンプレートは要求しないが、phase を移る時は次に必要な事実を欠落させない。

- 扱った scope と result
- commit SHA、または未 commit / stop の理由
- 検証コマンド・結果と review status
- docs / backlog 同期、Product Decision Ledger、follow-up、残存リスクの有無

## Stop Conditions

- 上記の判断境界で Stop に該当する仕様・UX・データ保持・削除方針が残っている。
- 要求と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している。
- High-risk 変更で必須の検証を代替手段でも裏付けられない。
- ユーザーが停止・相談・計画のみを指示している。
