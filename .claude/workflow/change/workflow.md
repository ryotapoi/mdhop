# Change Workflow

この workflow は、単発の実装依頼、または Goal 内で切り出された 1 commit 分の作業を完了させるための手順。
Goal 経由の場合は `goal-workflow` skill を入口とし、各 commit でこの workflow を繰り返す。単発依頼の場合はこの workflow を直接の入口とする。

## Intent

単発依頼、または Goal 内で切り出された 1 commit 分の作業を、必要十分な調査・計画・実装・検証・記録で完了させる。手続きの重さは、作業の大きさとリスクに合わせる。

## Inputs

- ユーザー依頼
- 関連する `docs/rules/`, `docs/specs/`, `backlog/backlog.md`, `docs/decisions/`, `llm-wiki/`（作業地図）
- 既存コードと git history

## Intake 分類

最初に作業を分類する。判定が揺れたら High-risk 寄りに倒す。Small / Normal の境界は迷ったら Normal で進めてよい。

- **Small**: typo、文書、テスト期待値、1 ファイルの明確な修正
- **Normal**: 通常の機能追加・バグ修正・複数ファイル変更
- **High-risk**: データ永続化、マイグレーション、並行性、公開 API、削除、広い UI 挙動、外部連携
- **Exploratory**: 原因不明、仕様不明、技術検証が先に必要

## Routing

- Exploratory → `change/investigate.md` で事実を揃えてから判断し直す
- Plan が必要な変更 → `change/plan.md`（plan mode は使わず、内部で計画を立ててそのまま `change/implement.md` へ進む。詳細は `change/plan.md`）
- Plan 省略可な変更 → そのまま `change/implement.md`
- 検証 → `change/verify.md`
- レビュー → `change/review.md`
- 完了 → `change/finish.md`
- 節目で構造を見る → `maintenance.md`

## Decision Criteria

- workflow は 1 つの commit 単位で回す。1 commit に独立した複数作業を混ぜない。
- 実行中に 1 commit として不自然だと分かったら、作業を広げず、Goal 実行中は Goal main に事実を返して commit 単位を切り直す。単発 Change では今回扱う単位を切り直す。
- Small は plan を省略してよい。作業内容と検証だけ簡潔に示す。
- 仕様・UX・データモデル・複数ファイル変更・設計判断を伴うなら plan を作る。
- High-risk は plan・検証・必要なレビューを明示する。
- 実装判断に影響する不明点は、調査・検証・既存情報で潰してから進む。複数案があっても、現在の要求と情報源から適切に選べるなら止まらず進める。
- 途中でタスクの性質が変わったら、Intake からやり直す（格上げは許容）。
- 既存 worktree 差分向けの特別な snapshot / staging / clean check フローは作らない。通常の差分確認と commit discipline で巻き込みを防ぐ。

## Specs Priority

複数情報源が矛盾した場合、新しい順で照合する。古い方を直す。

1. 現在のユーザー依頼
2. `docs/rules/`
3. `docs/decisions/`
4. `docs/specs/`
5. tests

仕様・UX に関わる判断は、現在の要求、`docs/rules/` / `docs/specs/` / `docs/decisions/`、既存コード、調査・検証結果から最善案を選ぶ。ユーザーが別方針を選ぶ可能性がある重要な判断は、進められるなら採用案で進め、Goal 完了報告の `ユーザー判断が必要` に残す。

## Acceptance

- ユーザーの要求が満たされている
- 必要な情報源が同期されている（`docs/specs/`, `backlog/backlog.md`, `docs/decisions/`、知見はソースコメント / `llm-wiki/`）
- 選んだ検証とレビューの深さを説明できる
- コミット済み、またはユーザーが明示的にコミット不要とした状態
- コミット後の進み方は `change/finish.md` に従う（Goal 実行中は次の 1 commit workflow へ、Goal 外の単発依頼はユーザー指示待ち）

## Stop Conditions

- その時点の情報では適切な仕様・UX・データ保持・削除方針を決められず、ユーザー判断や不足情報なしに進めること自体が不適切
- 要求と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している
- High-risk 変更で必須の検証を代替手段でも裏付けられない
- ユーザーが停止・相談・計画のみを指示している

## Subagent / Skill

- 複数ファイル横断・キーワードのファンアウト調査は Explore subagent に委譲する
- skill は各 phase の workflow の指示に従って使う。
- `boundary-control` は横断チェックとして全 phase に効く。今回の要求の外へ作業を広げそうな時、隣接作業が見つかった時、scope を変える編集の前に使い、active scope 内か（workflow-required / incidental-required）を判定する。active workflow を止めたり置き換えたりはしない
- 詳細は各 phase のファイル参照
