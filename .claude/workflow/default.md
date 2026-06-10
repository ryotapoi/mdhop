# Default Workflow

この workflow は、単発依頼、または Goal 内で切り出された 1 commit 分の作業を完了させるための手順。
Goal を使う作業全体の入口は `goal-workflow` skill とし、この workflow は Goal 内の各 commit で繰り返す。

## Intent

単発依頼、または Goal 内で切り出された 1 commit 分の作業を、必要十分な調査・計画・実装・検証・記録で完了させる。手続きの重さは、作業の大きさとリスクに合わせる。

## Inputs

- ユーザー依頼
- 関連する `rules/`, `specs/`（あれば）, `backlog/backlog.md`, `decisions/`, `references/knowledge.md`
- 既存コードと git history

## Intake 分類

最初に作業を分類する。判定が揺れたら High-risk 寄りに倒す。Small / Normal の境界は迷ったら Normal でよい。

- **Small**: typo、文書修正、テスト期待値の単純な追加、1 ファイルの明確な修正
- **Normal**: 通常の機能追加・バグ修正・複数ファイル変更
- **High-risk**: 以下のいずれかに触れる変更
  - SQLite スキーマ変更、マイグレーション、`build` の挙動変更（`delete --rm` などの破壊的処理を含むもの）
  - SQL の意味的変更（プレースホルダの扱い、`GROUP BY`、`MAX`/`MIN` 等の集約関数、NULL 三値論理、exists_flag フィルタ）
  - `modernc.org/sqlite` 固有の罠を踏みうる変更（`LastInsertId` の罠、`:memory:` 接続、`PRAGMA table_info` ベースの migration など。詳細は `references/knowledge.md`）
  - リンク解決ロジック・ルート優先ルール（ADR 0004）に触れる変更
  - CLI 仕様の破壊的変更（stdout JSON フィールド、--where フィルタの解釈変更など）
  - vault escape / パス安全性に関わる変更
  - 公開 API の削除、外部連携、並行性
- **Exploratory**: 原因不明、仕様不明、技術検証が先に必要

## Routing

- Exploratory → `investigate.md` で事実を揃えてから判断し直す
- Plan が必要な変更 → `plan.md`（plan mode は使わず、内部で計画を立ててそのまま `implement.md` へ進む。詳細は `plan.md`）
- Plan 省略可な変更 → そのまま `implement.md`
- 検証 → `verify.md`
- レビュー → `review.md`
- 完了 → `finish.md`
- 節目で構造を見る → `maintenance.md`

## Decision Criteria

- workflow は 1 つの commit 単位で回す。実行中に 1 commit を超えると分かったら、作業を広げず（Goal 実行中は）`goal-workflow` skill に戻って commit 単位を切り直す。
- Small は plan を省略してよい。作業内容と検証だけ簡潔に示す。
- 仕様・データモデル・複数ファイル変更・設計判断を伴うなら plan を作る。
- High-risk は plan・検証・必要なレビューを明示する。
- 実装判断に影響する不明点は、調査かユーザー確認で潰してから進む。
- 途中でタスクの性質が変わったら、Intake からやり直す（格上げは許容）。

## Specs Priority

複数情報源が矛盾した場合、新しい順で照合する。古い方を直す。

1. 現在のユーザー依頼
2. `rules/`
3. `decisions/`
4. `specs/`（mdhop では現状未配置だが将来用に位置付ける）
5. tests

仕様・CLI 挙動に関わる判断は実装で決めず、ユーザー確認に回す。

## Acceptance

- ユーザーの要求が満たされている
- 必要な情報源が同期されている（`backlog/backlog.md`, `decisions/`, `references/knowledge.md`、必要なら `specs/`）
- 選んだ検証とレビューの深さを説明できる
- コミット済み、またはユーザーが明示的にコミット不要とした状態
- コミット後の進み方は `finish.md` に従う（Goal 実行中は次の 1 commit workflow へ、Goal 外の単発依頼はユーザー指示待ち）

## Stop Conditions

- 仕様・CLI 挙動・データ保持・削除方針に複数の妥当な選択肢がある（即停止して確認。ただし `design-decision` で結論が出る範囲なら止まらず採否を決める）
- 要求と `rules/` / `specs/` / `decisions/` が矛盾している
- High-risk 変更で検証手段が確保できない
- レビュー周回が 3 周目に入る前 → 状況報告して指示待ち（選択肢提示しない）
- ユーザーが停止・相談・計画のみを指示している

## Subagent / Skill

- 複数ファイル横断・キーワードのファンアウト調査は Explore subagent に委譲する（CLAUDE.md の Constraints / サブエージェント活用に従う）
- skill は判断プロトコル（`design-decision`, `module-boundary`, `tdd` など）として呼ぶ。mdhop 固有のレビューは `review-plan-all` / `review-code-all` 経由で `review-*-mdhop` を使う
- 詳細は各 phase のファイル参照
