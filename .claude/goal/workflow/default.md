# Default Workflow (goal)

## Intent

goal-loop の A パスから呼ばれ、選んだ1タスクを必要十分な調査・計画・実装・検証・記録で完了させる。手続きの重さは、作業の大きさとリスクに合わせる。

## Inputs

- goal-loop で選ばれた1タスク（`backlog/backlog.md` の該当エントリ）
- 関連する `rules/`, `specs/`（あれば）, `decisions/`, `references/knowledge.md`
- 変更対象の既存コード

## Intake 分類

最初に作業を分類する。Small / 非 Small の境界で迷ったら非 Small に倒す。

- **Small**: typo、文書修正、テスト期待値の単純な追加、1 ファイルの明確な修正
- **非 Small**: 上記以外（通常の機能追加・バグ修正・複数ファイル変更・設計判断を伴う変更・High-risk 領域に触れる変更）

Intake では Small / 非 Small だけ判定する。リスクの細かい判別（並列レビューを呼ぶか self-check で済ますか等）は plan.md / review.md 側で行う。

**Exploratory（原因不明・仕様不明・技術検証が先に必要）** が必要な場合は、Intake より前に `investigate.md` に進む。ただし goal-loop で選ばれるタスクは通常 backlog で粒度が固まっているため、Exploratory が必要になるケースは少ない。

## Routing

- Exploratory → `investigate.md` で事実を揃えてから Intake に戻る（通常は不要）
- 非 Small → `plan.md` → `implement.md` → `verify.md` → `review.md` → `finish.md`
- Small → `implement.md` → `verify.md` → `review.md`（self-check）→ `finish.md`

各 phase の詳細はそれぞれのファイル参照。

## Decision Criteria

- Intake で Small なら plan / 並列レビューを省略してよい（review.md の self-check のみ）
- 非 Small は plan を立てる。Plan Review / Review は通常 並列レビューだが、軽微なら self-check に落とせる（`plan.md` / `review.md` 参照）
- 実装判断に影響する不明点は、調査で潰してから進む。自動判断できない案件は `goal-blocked.md` を作成して停止する
- 途中でタスクの性質が変わったら Intake からやり直す（格上げは許容）

## Specs Priority

複数情報源が矛盾した場合、新しい順で照合する。古い方を直す。

1. `backlog/goal.md` および goal-loop で選ばれたタスク
2. `rules/`
3. `decisions/`
4. `specs/`（mdhop では現状未配置だが将来用に位置付ける）
5. tests

仕様・CLI 挙動に関わる判断は実装で決めず、自動判断できる範囲なら判断して `goal-decisions.md` に記録する。判断不能なら `goal-blocked.md` を作成して停止する。

## Acceptance

- タスクの要求が満たされている
- 必要な情報源が同期されている（`backlog/backlog.md`, `decisions/`, `references/knowledge.md`、必要なら `specs/`）
- コミット済み

## Stop Conditions

次の場合は `goal-blocked.md` を作成して停止する。

- 仕様・CLI 挙動・データ保持・削除方針に複数の妥当な選択肢があり自動判断不能
- 要求と `rules/` / `specs/` / `decisions/` が矛盾している
- High-risk 領域に触れる変更で検証手段が確保できない
- レビュー2周しても LGTM に至らない（`review.md` / `plan.md` 参照）

## Subagent / Skill

- 複数ファイル横断・キーワードのファンアウト調査は Explore subagent に委譲する（CLAUDE.md の Constraints / サブエージェント活用に従う）
- skill は判断プロトコル（`design-principles`, `tdd`, `refactor-guard` など）として呼ぶ
- 詳細は各 phase のファイル参照
