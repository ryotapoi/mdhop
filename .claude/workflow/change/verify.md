# Verify

## Intent

変更が要求を満たし、既存挙動を壊していないことを、適切な証拠で確認する。

## Inputs

- 変更差分（`git diff`, `git diff --cached`）
- plan または要求
- 関連テスト、ビルド設定、Preview / GUI 確認手段

## Decision Criteria

- 自動検証を優先する。ビルド・テスト・静的チェックで確認できるものは先に通す
- テスト可能な振る舞い変更や bug fix に unit test / regression test がない場合、検証未完了として扱う。例外は理由を明記する
- GUI / Preview / 実機依存 / 外部連携 / 時間・非同期の挙動は、まず自力で取れる証拠（ビルド・テスト・スナップショット・ログ等）で確認する
- それでもユーザーの観察・操作なしに確定できない挙動が残るなら、Stop Condition または残存リスクとして報告する
- 検証不能な High-risk 変更は完了扱いにしない
- テスト実行がハング・タイムアウトで中断された場合、テストプロセスや test host アプリが残留することがある。再実行の前に残留を確認し、その中断に由来するプロセス（起動時刻が中断した run と整合するもの）に限って終了する。無差別な kill はしない（別セッションやユーザーが起動したプロセスを巻き込むため）
- テスト失敗を「環境フレーク」と判定する前に、残留プロセスを確認する。残留があるならフレーク判定せず、掃除して再実行した結果で判断する（残留 test host がフレークに見える失敗を作った実測あり）

## Verification

<!-- slot: ビルド・テスト・実行・実画面確認の具体手段を書く（例: build_macos / test_macos、go build ./... / go test ./...、./gradlew verifyAll / verifyAllConnected、CLI なら bin/<tool> <args>、Preview 確認手段）。ユーザー確認が必要な領域（実機・権限・外部連携）も書く。 -->

- 基本: `go test ./...`、`go build ./...`、必要に応じて `go vet ./...`。
- バイナリ: `go build -o bin/mdhop ./cmd/mdhop`。
- 集中検証: 変更範囲に応じて `go test ./internal/core/`、`go test ./internal/core/ -run TestBuild`、対象 test の `-run` を使う。
- CLI 出力・対話的挙動・DB 副作用: `bin/mdhop <args>` を実行し、stdout / stderr / 終了コード / DB 内容を確認する。
- `delete --rm`, `move`, rewrite 系などの破壊的処理は `testdata/` または `tmp/` 配下の一時 vault で確認する。
- ユーザー確認が必要な領域: N/A — 実機・権限・外部連携なし。
- Go / SQLite / YAML 等の外部 API 仕様が判断に影響する場合は一次情報を確認する。
<!-- /slot -->

## Acceptance

- 実行した検証と結果を説明できる
- 追加・更新した unit test / regression test、または追加しなかった理由を説明できる
- 検証しなかった項目がある場合、その理由が説明できる
- ユーザー確認が完了条件に必要な場合は通過している。必須でない場合は、未確認の理由を残存リスクまたは Goal 完了報告のユーザー判断候補として説明できる。

## Stop Conditions

- 必須の検証が環境要因で実行できず、代替手段でも裏付けられない
- 完了判断に必須の UI / 挙動確認を、自力検証・代替証拠・ユーザー確認のいずれでも裏付けられない
- 検証結果が要求または仕様と矛盾する
