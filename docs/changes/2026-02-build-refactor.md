# Build refactor: fail-fast validation and transaction

## Touches
- `internal/core/build.go` — Build() リファクタリング: detectAmbiguousLinks 削除、バリデーション前倒し、トランザクション化
- `internal/core/build_test.go` — TestBuildExcludesMdhopDir, TestBuildEdgeLineEnd 追加
- `internal/core/update.go` — コメント修正のみ

## Verification
- `go test ./...` 全テスト通過

## Pitfalls
- 曖昧リンク判定で `basenameKey(link.target)` を使うと `.md` 以外の拡張子も削られてしまう。`strings.ToLower(link.target)` に修正（update.go と一貫性を保つ）
