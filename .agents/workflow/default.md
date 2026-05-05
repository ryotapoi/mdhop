# Default Workflow

## ICAR

- **Intent**: ユーザーの依頼を、必要十分な調査・計画・実装・検証・記録で完了させる。
- **Constraints**:
  - 手続きの重さは作業の大きさとリスクに合わせる。
  - 判断に影響する `rules/`, `decisions/`, `backlog/backlog.md`, `references/knowledge.md`, `specs/`（あれば）は推測で済ませず実物を確認する。
  - 仕様・CLI 挙動・データ保持・削除方針に関わる判断は、実装で決めず必要ならユーザー確認に回す。
  - コミット完了後は次のタスクに進まない。
- **Acceptance**:
  - ユーザーの要求が満たされている。
  - 必要な情報源が同期されている。
  - 選んだ検証とレビューの深さを説明できる。
  - コミット済み、またはユーザーが明示的にコミット不要とした状態。
- **Relevant**:
  - `.agents/workflow/investigate.md`
  - `.agents/workflow/plan.md`
  - `.agents/workflow/implement.md`
  - `.agents/workflow/verify.md`
  - `.agents/workflow/review.md`
  - `.agents/workflow/finish.md`
  - `.agents/workflow/maintenance.md`

## Intake

最初に作業を分類する。判定が揺れたら High-risk 寄りに倒す。Small / Normal の境界は迷ったら Normal で進めてよい。

- **Small**: typo、文書修正、テスト期待値の単純な追加、1 ファイルの明確な修正
- **Normal**: 通常の機能追加・バグ修正・複数ファイル変更
- **High-risk**: SQLite スキーマ、SQL 意味変更、リンク解決、ルート優先ルール、stdout JSON、vault escape、`move` / `delete --rm` 等の破壊的処理、公開 API、外部連携、並行性
- **Exploratory**: 原因不明、仕様不明、技術検証が先に必要

## Flow

- Exploratory → `investigate.md` で事実を揃えてから Intake をやり直す。
- Plan が必要な変更 → `plan.md` で実装前の ICAR を揃える。
- Plan 省略可な変更 → `implement.md` へ進み、局所 ICAR を満たす。
- 実装 → `implement.md`
- 検証 → `verify.md`
- レビュー → `review.md`
- 完了 → `finish.md`
- 節目で構造を見る → `maintenance.md`

## Source Priority

複数情報源が矛盾した場合、新しい順で照合する。古い方を直す。

1. 現在のユーザー依頼
2. `rules/`
3. `decisions/`
4. `specs/`（あれば）
5. tests

## Execution Notes

独立した調査・レビュー・実装は並列化してよい。領域固有の判断は各 phase の workflow に従って skill を使う。

## Stop Conditions

- 仕様・CLI 挙動・データ保持・削除方針に複数の妥当な選択肢が実際にある。
- 要求と `rules/` / `specs/` / `decisions/` が矛盾している。
- High-risk 変更で検証手段が確保できない。
- ユーザーが停止・相談・計画のみを指示している。
