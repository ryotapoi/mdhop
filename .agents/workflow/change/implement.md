# Implement Workflow

## ICAR

- **Intent**: 承認済み plan、または plan を省略できる軽微な変更の明確な要求を、既存設計と情報源に整合する形で実装する。
- **Constraints**:
  - Goal の Implementer は commit、review lane、受け入れ判定を行わない。Gatekeeper の return は Conductor 経由で受け、同じ Change 内で修正・再検証する。
  - plan を省略する場合でも、workflow は 1 commit に収まる軽微な変更だけにする。
  - 実装中に 1 commit として不自然だと分かったら、作業を広げず `change/plan.md` に戻るか、今回扱う 1 commit 単位へ切り直す。Goal 実行中は `goal-workflow` skill に戻って Goal 側で切り直す。
  - 既存の局所パターンに従う。変える場合は理由を説明できるようにする。
  - 新しい型・ファイル・外部依存・責務配置・module/package/target/folder 境界を扱う場合は、実装前に `module-boundary` で配置判断を確認する。
  - 型定義・API・依存方向は実物で確認する。
  - 振る舞い変更や bug fix では、同じ commit に unit test / regression test を追加または更新する。テストできない場合は理由を明記する。
  - 振る舞い変更があるなら、必要に応じて `docs/specs/` とテストを同期する。
  - commit に含める内容変更（code / tests / `backlog/backlog.md` / `docs/specs/` / `llm-wiki/` / `docs/decisions/` / ADR）は、この phase で完了する。review 後の finish / commit では tracked file の内容を追加・変更・削除しない。
  - 実装中またはレビュー指摘対応中に product decision（UX・データ意味・cross-surface 等。カテゴリ一覧は同ファイル）が発生した場合は、`.agents/workflow/design-decision-record.md` の Alternative Check・記録・報告基準に従う。可逆で影響が小さい選択は採用案で進めて ledger に残す。複数の妥当案が残り、かつ選択が非可逆（データ保持・削除・マイグレーション・外部公開契約）またはやり直しコストが大きい場合、または正本と矛盾する場合は Stop Conditions に従う。
  - 実装中に見つかった別タスクは、今やる理由がなければ `backlog/backlog.md` に逃がす。今回の commit の active scope 内か迷う作業は、`change/workflow.md` の横断スコープ制御で分類してから着手する（adjacent なら実行せず capture / report）。
  - ループ内で時刻を扱う場合は各反復で取得する（ループ外で 1 回だけ取得しない）。
- **Acceptance**:
  - 要求された振る舞いが実装されている。
  - 必要な `docs/specs/` / tests / `backlog/backlog.md` / `docs/decisions/` / `llm-wiki/` の同期が済んでいる。
  - 余計なスコープ拡張がない。
- **Relevant**:
  - 承認済み plan、または Small 変更の明確な要求
  - 関連する `docs/rules/`, `docs/specs/`, `llm-wiki/`（作業地図）
  - 変更対象と周辺コード

## Flow ICAR

### Code Change

- **Intent**: 要求された振る舞いを最小十分な差分で実装する。
- **Constraints**:
  - テストファーストで進める場合は `tdd` スキルに従う。
  - 構造の悪さが実装を歪める場合は、同じ変更で直すか、別リファクタ plan に切るかを判断する。
- **Acceptance**: plan と実装上の事実が食い違っていない。
- **Relevant**: 変更対象コード、関連テスト、関連 `docs/specs/`。

### Documentation Sync

- **Intent**: 実装で変わった仕様・知見・未着手作業を正しい情報源に反映する。
- **Constraints**:
  - 完了した backlog 項目があれば `backlog/backlog.md` の該当行を `[x]` 等で更新する。
  - 技術的知見は、特定ソースに紐づく罠はそのコードのコメントへ、横断的な挙動・設計理解は `llm-wiki/` の該当地図へ残す。単一の集約知見ファイルは作らない。
  - 今回の変更で `llm-wiki/` の地図が古くなっていないか確認し、古くなった場合は同じ差分で追従する。各ページの更新方法（再生成するか手編集するか）は `regen` 区分に従い、その判断基準の正本は `docs/rules/information-management.md`（および `llm-wiki/` の索引）とする。区分ごとの手順はこの workflow に写経しない。
  - 後から制約になる判断は `docs/decisions/` に残す。
- **Acceptance**: 実装差分と情報源が矛盾していない。
- **Relevant**: `docs/specs/`, `backlog/backlog.md`, `docs/decisions/`, `llm-wiki/`（作業地図）。

## Tooling

<!-- slot: ビルド・テスト・実行のコマンドと使うツールを書く（例: XcodeBuildMCP の build_macos / test_macos と「Bash で xcodebuild を直接叩かない」、go build ./... / go test ./...、./gradlew verifyAll、CLI なら bin/<tool>）。API 仕様の一次情報確認手段も書く（外部 API が無ければ「N/A — 外部 API 参照なし」と明記する）。 -->

- 全体ビルド: `go build ./...`
- 全体テスト: `go test ./...`
- 静的チェック: `go vet ./...`
- バイナリ作成: `go build -o bin/mdhop ./cmd/mdhop`
- 集中テスト例: `go test ./internal/core/`、`go test ./internal/core/ -run TestBuild`
- CLI 挙動確認: `bin/mdhop <args>` で stdout / stderr / 終了コード / DB 副作用を見る。
- 破壊的処理（`delete --rm`, `move` など）は `testdata/` または `tmp/` 配下の一時 vault で確認する。
- Go / SQLite / YAML 等の外部 API 仕様が判断に影響する場合は一次情報を確認する。
<!-- /slot -->

## Stop Conditions

- plan と実装上の事実が食い違う。
- 上記の product decision 境界で Stop に該当する重要な仕様・UX・プロダクト判断が必要になった。
- リファクタなしでは変更が不自然または危険になる。
- module / package / target / folder 境界の判断なしに、新しい責務や外部依存を既存構造へ押し込む必要が出た。
