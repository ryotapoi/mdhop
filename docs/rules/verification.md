# 変更時の検証

変更を完了とみなすための project 固有 gate を定める。対象に複数の条件が当てはまる場合は、該当する gate をすべて満たす。

## 変更条件ごとの必須 gate

| 変更条件 | 必須の検証 |
| --- | --- |
| ドキュメント・agent 指示だけを変更した | 記載した path、command、設定を repository の実物と照合し、差分を読む。Go の build / test は不要。 |
| Go の本番コード、`go.mod`、`go.sum` を変更した | 変更した package または振る舞いの集中 test を実行した後、`go test ./...` と `go build ./...` を実行する。 |
| test code または `testdata/` だけを変更した | 対象 test を集中実行した後、`go test ./...` を実行する。本番コードや build 設定を変えていなければ `go build ./...` は不要。 |
| flag 解析、help、出力形式、error、終了コード、または CLI から見える副作用を変更した | Go 本番コードの gate に加え、`go build -o bin/mdhop ./cmd/mdhop` で実バイナリを作り、影響する正常系と異常系を実行する。stdout、stderr、終了コード、および file / DB の副作用を確認する。 |
| DB、link parse / resolve、path 操作、file mutation を変更した | 関連する `internal/core` の test を集中実行し、Go 本番コードの gate も通す。CLI の観測可能な振る舞いが変わる場合は実バイナリ確認も行う。 |
| `go vet` が検査する構造を変更した | 上記に加えて `go vet ./...` を実行する。典型例は format 呼び出し、struct tag、build tag、test signature、copylock を含む変更。 |

集中 test は失敗を早く特定するためのものであり、`go test ./...` の代わりにはしない。例として package 単位なら `go test ./internal/core/`、test 単位なら `go test ./internal/core/ -run TestBuild` の形を使う。

## 非自明な制約と例外

- CI の自動 gate は `go.mod` の Go version を使い、Ubuntu と macOS の両方で `go test ./...` を実行する。path、Unicode 正規化、file operation に関わる変更は platform 差を test で固定し、CI が動く変更では両 OS の結果を確認する。
- `cmd/mdhop` の CLI test は主に `runQuery` などの run 関数を直接呼ぶため、process の終了コードや main で付与する error prefix までは保証しない。CLI contract に触れる変更では実バイナリ確認を省略しない。
- `delete --rm`、`move`、`set`、rewrite 系など disk を変更する確認は、`testdata/vault_*` を一時 directory へコピーした vault または `tmp/` 配下の使い捨て vault で行う。repository の共有 fixture や利用者の vault を直接変更しない。file と DB の双方を確認し、dry-run がある command は無変更であることも確認する。
- `convert`、`repair`、`simplify` は DB を使わず file を直接走査する。実際に書き換える確認の後は同じ一時 vault で `build` を実行し、index と書き換え後の file が整合することを確認する。
- schema や index の永続形式を変えた場合は、既存 DB の in-place migration ではなく `build` による再生成を確認する。mutation の失敗系を変えた場合は、file と DB に部分更新や一時 file が残らないことも確認する。
- JSON contract の変更では `AGENTS.md` の出力契約に対し、stdout を単独で parse でき、付加情報が stderr に分離されることを確認する。
- CLI test は process-wide の `os.Stdout` / `os.Stderr` を差し替えるものがあるため、それらを使う test は `t.Parallel()` にしない。
- repository root の `mdhop.yaml` は `testdata/**` と `examples/**` を build 対象外にする。また root 全体には用途の異なる同名 file が多いため、mdhop 自身で docs や wiki を確認するときは `--path` で対象 subtree を限定し、root 全体の basename conflict を検証失敗として扱わない。
- Go、SQLite、YAML の依存 API の仕様が実装判断や期待値に影響する変更では、その一次資料と実装・test を照合する。外部 API に依存しない変更では不要。
