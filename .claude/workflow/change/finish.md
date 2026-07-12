# Finish

## Intent

Gatekeeper（Small では Conductor）を通過した変更を、コミットまで含めて完了状態にする。Goal 経由では Implementer → Gatekeeper（Normal 以上）→ Conductor の順で担当し、commit は常に Conductor が行う。Goal を経由しない単発 Change では Gatekeeper / Conductor という役割分担自体が存在しないため、現在の agent が実装・review 差配・採否・commit のすべてを担う。

## Inputs

- 変更差分
- 検証結果（Implementer が実行）
- Gatekeeper の受け入れ判定（Small では Conductor 自身の照合結果）

## Decision Criteria

- commit 前照合は、機械照合と、Gatekeeper の受け入れ判定・commit message 草案の確認からなる。機械照合の内容: `git rev-parse` での Gatekeeper 報告 baseline HEAD SHA の実在確認、`git status --short`（Gatekeeper 報告の `git status --porcelain` の対象状態との一致）で意図しない書き込みがないことの確認、`git diff --stat` と Gatekeeper 報告の stat の一致、commit 予定差分全体のハッシュ（`git diff <baseline>..HEAD` 相当、未 commit 差分なら `git diff | shasum`）を再計算して Gatekeeper 報告のハッシュと一致すること、テストの自己実行（成功時は exit code のみ確認、生の出力は読まない）。Conductor 自身のテスト自己実行が worktree を変更し得るため、実行後に `git status` と diff hash を再照合してから commit する。Small では Conductor が diff を直接実読して照合する（Gatekeeper 省略、この場合は baseline SHA・diff hash の照合対象も Conductor 自身の実読結果に置き換わる）。
- コミットは `commit` スキルで作成する。finish では tracked file の内容を追加・変更・削除しない
- 文書同期（`backlog/backlog.md` / `docs/decisions/` / `llm-wiki/` / `docs/specs/`）や ADR が不足していると分かった場合は、commit せず Conductor 経由で Implementer に差し戻し、`change/implement.md` の続きから verify と review（Gatekeeper 再照合）をやり直す
- commit 前に差分、review 結果、Product Decision Ledger、同期済み docs を照合する。product decision（UX・データ意味・cross-surface 等。カテゴリ一覧は同ファイル）について `.claude/workflow/design-decision-record.md` の基準で採用案・別案・理由を追えない場合は commit せず Implementer に差し戻す
- コミットメッセージ規約は `commit` スキル側が判断する。Implementer が返した commit message 草案は参考にしつつ、規約適合は `commit` スキルが最終判断する
- このファイルでは commit スキルを呼ぶこと自体を担保する
- Goal 実行中の場合、commit 後に Goal 全体が完了したか、次の 1 commit workflow に進むかを `goal.md` で確認する
- Goal 完了報告では、`ユーザー判断が必要` と Goal Review の `レビュー上限超過` の有無（reviewer が複数の場合は reviewer ごと）、および Goal Review が MUST を出した場合のすり抜け記録（`goal.md` 参照）を明示する

## Acceptance

- コミット済みで、作業ツリーの残差分が意図したものだけ
- Goal 実行中は次の 1 commit workflow に進むか Goal 完了かを `goal.md` で判断する。Goal 外の単発依頼の場合はコミット完了後に次のタスクへ進まない（ユーザー指示待ち）
- Goal が完了する場合は、記憶ではなく Product Decision Ledger / review 結果 / 同期済み docs から、ユーザー判断が必要な事項と Goal Review のレビュー上限超過の有無が明示されている

## Stop Conditions

- ユーザーがコミット前確認を求めている
