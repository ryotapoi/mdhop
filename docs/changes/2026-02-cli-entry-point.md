# CLI エントリポイント: build, resolve, query の3コマンドを CLI から実行可能に

## Touches
- `cmd/mdhop/main.go` — エントリポイント。os.Args[1] で build/resolve/query をルーティング。未実装コマンドは "not yet implemented" を返す
- `cmd/mdhop/build.go` — runBuild: --vault フラグ → core.Build() を呼ぶ
- `cmd/mdhop/resolve.go` — runResolve: --vault, --from, --link, --format, --fields → core.Resolve() → text/JSON 出力
- `cmd/mdhop/query.go` — runQuery: --vault, --file, --tag, --phantom, --name, --format, --fields, --include-head, --include-snippet, --max-backlinks, --max-twohop, --max-via-per-target → core.Query() → text/JSON 出力
- `cmd/mdhop/format.go` — 共有フォーマッタ。printResolve{Text,JSON}, printQuery{Text,JSON}, parseFields, validateFormat, validateResolveFields
- `cmd/mdhop/format_test.go` — 16 テスト（parseFields, validateFormat, validateResolveFields, resolve text/JSON, query text/JSON, exists=false, phantom/tag ケース）
- `cmd/mdhop/cli_test.go` — 6 テスト（CLI バリデーション: missing flags, invalid format/fields）

## Verification
- `go build -o bin/mdhop ./cmd/mdhop` — コンパイル OK
- `go test ./...` — 全テストパス（既存 98 + 新規 22 = 120）

## Pitfalls
- writeNodeInfoText で firstIndent と restIndent を分離する必要がある（リスト項目 `- type: note` vs 継続行 `  name: ...`）
- jsonNodeInfo の Exists は `*bool` + `omitempty` にしないと、note の exists=false が JSON から落ちる
- twohop の text 形式は overview.md の例に従い one-line 形式（`via: note: path`）を採用
