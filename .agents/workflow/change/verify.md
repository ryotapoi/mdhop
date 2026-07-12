# Verify Workflow

## ICAR

- **Intent**: 変更が要求を満たし、既存挙動を壊していないことを、適切な証拠で確認する。
- **Constraints**:
  - 自動検証を優先する。ビルド・テスト・静的チェックで確認できるものは先に通す。
  - テスト可能な振る舞い変更や bug fix に unit test / regression test がない場合、検証未完了として扱う。例外は理由を明記する。
  - 自分で確認できる UI / 操作 / Preview / 実行環境の挙動は先に確認する。
  - 複雑な GUI、見た目の好み、実機依存、ユーザー観察が必要な挙動は、完了判断に必須ならユーザー確認に回す。必須でなければ残存リスクまたは Goal 完了報告のユーザー判断候補に残す。
  - 検証不能な High-risk 変更は完了扱いにしない。
  - Goal の Normal 以上では、Gatekeeper が Implementer の自己報告を証拠にせず、必要な test を再実行して command / exit code / duration を受け入れ証拠にする。
- **Acceptance**:
  - 実行した検証と結果を説明できる。
  - 追加・更新した unit test / regression test、または追加しなかった理由を説明できる。
  - 検証しなかった項目がある場合、その理由が説明できる。
  - ユーザー確認が必要なものだけ依頼し、不要なものは理由を説明できる。
- **Relevant**:
  - 変更差分（`git diff`, `git diff --cached`）
  - plan または要求
  - 関連テスト、ビルド設定、Preview / GUI 確認手段

## Verification

<!-- slot: ビルド・テスト・実行・実画面確認の具体手段を書く（例: build_macos / test_macos、go build ./... / go test ./...、./gradlew verifyAll / verifyAllConnected、CLI なら bin/<tool> <args>、Preview 確認手段）。テスト構成上の注意（SPM dependent package など）や API 仕様の一次情報確認手段も書く（外部 API が無ければ「N/A — 外部 API 参照なし」と明記する）。 -->

- 基本: `go test ./...`、`go build ./...`、必要に応じて `go vet ./...`。
- バイナリ: `go build -o bin/mdhop ./cmd/mdhop`。
- 集中検証: 変更範囲に応じて `go test ./internal/core/`、`go test ./internal/core/ -run TestBuild`、対象 test の `-run` を使う。
- CLI 出力・対話的挙動・DB 副作用: `bin/mdhop <args>` を実行し、stdout / stderr / 終了コード / DB 内容を確認する。
- `delete --rm`, `move`, rewrite 系などの破壊的処理は `testdata/` または `tmp/` 配下の一時 vault で確認する。
- Go / SQLite / YAML 等の外部 API 仕様が判断に影響する場合は一次情報を確認する。
<!-- /slot -->

## User Check

- docs / テストのみ / ロジックのみの変更では不要。
- UI 変更は、複雑な操作フロー・視覚判断・実機依存・自動検証不能な挙動がある場合に依頼する。

## Stop Conditions

- 必須の検証が環境要因で実行できず、代替手段でも裏付けられない。
- 完了判断に必須の UI / 挙動確認を、自力検証・代替証拠・ユーザー確認のいずれでも裏付けられない。
- 検証結果が要求または仕様と矛盾する。
