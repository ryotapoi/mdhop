# Implement Workflow

## ICAR

- **Intent**: 承認済み plan、または plan を省略できる軽微な変更の明確な要求を、既存設計と情報源に整合する形で実装する。
- **Constraints**:
  - 既存の局所パターンに従う。変える場合は理由を説明できるようにする。
  - 型定義・API・依存方向は実物で確認する。
  - 振る舞い変更があるなら、必要に応じて `rules/overview.md`、`specs/`、テストを同期する。
  - 実装中に見つかった別タスクは、今やる理由がなければ `backlog/backlog.md` に逃がす。
  - ループ内で時刻を扱う場合は各反復で取得する。
- **Acceptance**:
  - 要求された振る舞いが実装されている。
  - 必要な docs / tests / `backlog/backlog.md` の同期が済んでいる。
  - 余計なスコープ拡張がない。
- **Relevant**:
  - 承認済み plan、または Small 変更の明確な要求
  - 関連する `rules/`, `references/knowledge.md`, `specs/`（あれば）
  - 変更対象と周辺コード

## Flow ICAR

### Code Change

- **Intent**: 要求された振る舞いを最小十分な差分で実装する。
- **Constraints**: TDD でやる場合は `tdd` スキルに従う。Go コードは `gofmt` を通す。
- **Acceptance**: plan と実装上の事実が食い違っていない。
- **Relevant**: 変更対象コード、関連テスト、関連 rules。

### Documentation Sync

- **Intent**: 実装で変わった仕様・知見・未着手作業を正しい情報源に反映する。
- **Constraints**:
  - CLI 表面仕様が変わったら `rules/overview.md` と派生ドキュメントの要否を確認する。
  - 完了した backlog 項目があれば `backlog/backlog.md` を更新する。
  - 技術的知見は `references/knowledge.md` に残す。
  - 後から制約になる判断は `decisions/` に残す。
- **Acceptance**: 実装差分と情報源が矛盾していない。
- **Relevant**: `rules/`, `derived/`, `backlog/backlog.md`, `decisions/`, `references/knowledge.md`。

## Go Tooling

- ビルド: `go build ./...`
- テスト: `go test ./...`
- 静的チェック: `go vet ./...`
- バイナリ: `go build -o bin/mdhop ./cmd/mdhop`

## Stop Conditions

- plan と実装上の事実が食い違う。
- 実装中に仕様判断が必要になった。
- リファクタなしでは変更が不自然または危険になる。
