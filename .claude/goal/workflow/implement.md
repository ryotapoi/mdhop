# Implement (goal)

## Intent

`plan.md` で立てた plan、または plan を省略できる軽微な変更の明確な要求を、既存設計と情報源に整合する形で実装する。

## Inputs

- `plan.md` で立てた plan（コンテキスト上または `goal-plan.md`）、または Small 変更（`default.md` の Intake 分類）の明確な要求
- 関連する `rules/`, `specs/`（あれば）, `references/knowledge.md`
- 変更対象と周辺コード

## Decision Criteria

- 既存の局所パターンに従う。変える場合は理由を説明可能にする
- 型定義・API・依存方向は実物で確認（推測しない）
- TDD でやる場合は `tdd` スキルに従う（非 Small の振る舞い変更は基本 TDD。Small は省略可）
- 振る舞いが変わるなら `specs/`（あれば）の該当箇所を同期する
- backlog に積んでいた項目を実装完了したら `backlog/backlog.md` の該当行を `[x]` 等で更新する
- 実装中に見つかった別タスクは、今やる理由がなければ `backlog/backlog.md` に追記する。`goal-decisions.md` に「別タスクとして切る」と書くだけで終わらせない（追記しないと存在しないものとして扱われる → workflow.md「Task SSoT」）
- 構造の悪さが実装を歪める場合は、同じ変更で直すか、別タスク（B パスに移行）に切るかを判断する
- ループ内で時刻を扱う場合は各反復で取得（ループ外で 1 回だけ取得しない）

## Go Tooling

- ビルド: `go build ./...`
- テスト: `go test ./...`
- 静的チェック: `go vet ./...`
- フォーマット: `gofmt` / `goimports` は PostToolUse hook（`.claude/hooks/go-format.sh`）で自動実行されるため手動不要
- バイナリ: `go build -o bin/mdhop ./cmd/mdhop`（CLI 動作確認用）

## Worktree Check

Primary working directory が `.claude/worktrees/` 配下のときだけ:

- 環境変数や git status から実際の作業先を確認する
- 不一致なら `goal-blocked.md` を作成して停止する（worktree 不一致は自動修復不能）

## Acceptance

- 要求された振る舞いが実装されている
- 必要な `specs/`（あれば） / tests / `backlog/backlog.md` の同期が済んでいる
- 余計なスコープ拡張がない

## Stop Conditions

次の場合は `goal-blocked.md` を作成して停止する。

- plan と実装上の事実が食い違い、自動判断不能
- 実装中に仕様判断が必要になり、自動判断不能
- リファクタなしでは変更が不自然または危険になり、現タスクに含めるか別タスクに切るかの判断が困難
