# Verify Workflow

## ICAR

- **Intent**: 変更が要求を満たし、既存挙動を壊していないことを、適切な証拠で確認する。
- **Constraints**:
  - 自動検証を優先する。ビルド・テスト・静的チェックで確認できるものは先に通す。
  - CLI 出力・実 vault・SQLite DB ファイルへの副作用は、`bin/mdhop <args>` で確認する。
  - 破壊的処理（`delete --rm`, `move` など）は、テスト用 vault を作って確認する。
  - 検証不能な High-risk 変更は完了扱いにしない。
- **Acceptance**:
  - 実行した検証と結果を説明できる。
  - 検証しなかった項目がある場合、その理由が説明できる。
  - ユーザー確認が必要なものだけ依頼し、不要なものは理由を説明できる。
- **Relevant**:
  - 変更差分（`git diff`, `git diff --cached`）
  - plan または要求
  - 関連テスト、ビルド設定、testdata

## mdhop Verification

- 自動検証: `go build ./...` / `go test ./...` / `go vet ./...`
- バイナリビルド: `go build -o bin/mdhop ./cmd/mdhop`
- CLI 動作: `bin/mdhop <args>` で stdout / stderr / 終了コード / help 文言を見る
- DB 挙動: `testdata/vault_*` をコピーして `build` → `query` / `stats` で確認
- 破壊的処理は testdata 等の一時 vault で確認する

## User Check

- docs / テストのみ / 内部ロジックのみの変更では不要。
- ユーザー固有 vault、巨大データ、外部ツールとの同時編集など、こちらで再現できない観察が必要な場合だけ依頼する。

## Stop Conditions

- 必須の検証が環境要因で実行できない。
- ユーザー確認が必要な挙動が未確認。
- 検証結果が要求または仕様と矛盾する。
