# Verify (goal)

## Intent

変更が要求を満たし、既存挙動を壊していないことを、適切な証拠で確認する。

## Inputs

- 変更差分（`git diff`, `git diff --cached`）
- plan または要求
- 関連テスト、ビルド設定

## Decision Criteria

- 自動検証を優先する。ビルド・テスト・静的チェックで確認できるものは先に通す
- CLI 出力・対話的挙動・実 vault・SQLite DB ファイルへの副作用は、`bin/mdhop <args>` を Bash で叩いて確認する
- 確認系コマンド（`build --dry-run`, `query`, `stats` など）は副作用なしで挙動を確認できる
- 破壊的処理（`delete --rm`, `move` など）は、テスト用 vault を作って挙動を確認する
- 再現が難しい / 環境依存（巨大 vault、特定の Markdown データ等）でユーザー観察が必須のものは `goal-blocked.md` を作成して停止する
- 検証不能な High-risk 変更は完了扱いにせず、`goal-blocked.md` を作成して停止する

## mdhop Verification

- 自動検証: `go build ./...` / `go test ./...` / `go vet ./...`
- バイナリビルド: `go build -o bin/mdhop ./cmd/mdhop`
- CLI 動作: `bin/mdhop <args>` を Bash で叩く（新規・変更したコマンドの stdout / stderr / 終了コード / help 文言を見る）
- DB 挙動: `testdata/vault_*` をコピーして `build` → `query` / `stats` で確認
- 破壊的処理（`delete --rm`, `move` 等）は、テスト用 vault で挙動を確認する

## Acceptance

- 実行した検証と結果を説明できる
- 検証しなかった項目がある場合、その理由が説明できる

## Stop Conditions

次の場合は `goal-blocked.md` を作成して停止する。

- 必須の検証が環境要因で実行できない
- CLI 挙動の確認に実機ユーザー観察が必須
- 検証結果が要求または仕様と矛盾する
