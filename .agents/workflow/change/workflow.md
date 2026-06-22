# Change Workflow

この workflow は、単発の実装依頼、または Goal 内で切り出された 1 commit 分の作業を完了させるための手順。
Goal 経由の場合は `goal-workflow` skill を入口とし、各 commit でこの workflow を繰り返す。単発依頼の場合はこの workflow を直接の入口とする。

## ICAR

- **Intent**: 単発依頼、または Goal 内で切り出された 1 commit 分の作業を、必要十分な調査・計画・実装・検証・記録で完了させる。
- **Constraints**:
  - workflow は 1 つの commit 単位で回す。1 commit に独立した複数作業を混ぜない。Goal が複数 commit 単位を含む場合は、`goal-workflow` skill に戻って 1 commit に収まる単位へ切り直す。
  - 手続きの重さは作業の大きさとリスクに合わせる。
  - 判断に影響する `docs/rules/`, `docs/specs/`, `backlog/backlog.md`, `docs/decisions/`, `llm-wiki/`（作業地図）は推測で済ませず実物を確認する。
  - 仕様・UX に関わる判断は、現在の要求、`docs/rules/` / `docs/specs/` / `docs/decisions/`、既存コード、調査・検証結果から最善案を選ぶ。ユーザーが別方針を選ぶ可能性がある重要な判断は、進められるなら採用案で進め、Goal 完了報告の `ユーザー判断が必要` に残す。
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
  - `.agents/workflow/maintenance.md`

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
- 実装 → `change/implement.md`
- 検証 → `change/verify.md`
- レビュー → `change/review.md`
- 完了 → `change/finish.md`
- 節目で構造を見る → `maintenance.md`

## Source Priority

複数情報源が矛盾した場合、新しい順で照合する。古い方を直す。

1. 現在のユーザー依頼
2. `docs/rules/`
3. `docs/decisions/`
4. `docs/specs/`
5. tests

## Execution Notes

独立した調査・レビュー・実装は並列化してよい。領域固有の判断は各 phase の workflow に従って skill を使う。

既存 worktree 差分向けの特別な snapshot / staging / clean check フローは作らない。通常の差分確認と commit discipline で巻き込みを防ぐ。

横断のスコープ制御を全 phase に効かせる。今回の要求の外へ作業を広げそうな時、隣接作業が見つかった時、scope を変える編集の前に、その行為が active scope 内か判定する。active scope は「ユーザーの明示指示 + 起動 workflow / skill の Intent・Acceptance + phase の要件 + workflow が要求する review 対応・同期・記録」で構成し、ユーザーの一文だけで決めない。判定順は workflow-required（手順が要求）→ incidental-required（やらないと Acceptance を満たせない最小行為）→ adjacent-candidate（関連・有益だが達成には不要、実行しない）→ blocked（進められない、止めて報告）。adjacent-candidate は実行せず、project-relevant なら backlog / decision log 等へ capture するか最終報告で report する。この制御で自動進行する workflow を細切れに止めない。

## Stop Conditions

- その時点の情報では適切な仕様・UX・データ保持・削除方針を決められず、ユーザー判断や不足情報なしに進めること自体が不適切。
- 要求と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している。
- High-risk 変更で必須の検証を代替手段でも裏付けられない。
- ユーザーが停止・相談・計画のみを指示している。
