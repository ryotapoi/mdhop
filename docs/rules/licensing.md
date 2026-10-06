# 外部依存のライセンス通知

mdhop 自身のライセンスはルートの `LICENSE`、外部依存の著作権表記とライセンス全文は `THIRD-PARTY-NOTICES.txt` に置く。通知本文は原文を保持する。Go に通知ファイル名や集約形式を定める標準機能はないため、一般的な通知ファイル形式と [google/go-licenses](https://github.com/google/go-licenses) を使う。

`go-licenses` の収集結果を確認し、拾えない通知は手動で補う。独自の更新スクリプトは使わない。以下の既知の補完項目は現行依存に対するもので、依存の追加・更新時は対象ライブラリの原文も確認する。

## 依存・Go version の変更時

`cmd/mdhop` の実行時依存を対象とし、直接依存だけでなく間接依存を含める。`go.sum` 全体やテスト専用依存を配布対象の一覧として扱わない。OS / architecture / build tag によって依存が変わるため、配布対象の設定ごとに確認する。

```sh
go install github.com/google/go-licenses/v2@v2.0.1
mkdir -p tmp/licenses
go-licenses report ./cmd/mdhop --ignore github.com/ryotapoi/mdhop > tmp/licenses/report.csv
go-licenses save ./cmd/mdhop --ignore github.com/ryotapoi/mdhop --save_path tmp/licenses/collected
```

`save` の出力先は未作成のディレクトリを使う。上記は現在の OS / architecture を調べる例。現行通知の確認対象は darwin / linux / windows の amd64 / arm64。別の配布対象は `GOOS=linux GOARCH=amd64 go-licenses report ...` のように環境変数を指定し、`save` も同じ設定で実行する。収集先は対象ごとに分け、`CGO_ENABLED` / build tag も配布ビルドに合わせる。依存モジュールと version は同じ設定での `go list -deps -f '{{if .Module}}{{.Module.Path}} {{.Module.Version}}{{end}}' ./cmd/mdhop` と照合する。非 Go コード等を検査できない旨の警告も確認する。

収集した `LICENSE` / `NOTICE` 等をモジュール名、version、出典とともに通知ファイルへ集約し、次も確認する。ツールの判定だけで確認を完了としない。

- `gopkg.in/yaml.v3` はファイルごとに MIT / Apache-2.0 が異なる。`LICENSE` と `NOTICE` に加え、Apache-2.0 の全文を保持する。ツールが MIT とだけ判定しても Apache の通知を削除しない。
- モジュール内の別名ライセンス・サブディレクトリの通知も確認する。現行の `modernc.org/libc` の `LICENSE-GO` と `honnef.co/go/netdb/LICENSE`、`modernc.org/memory` の `LICENSE-GO` / `LICENSE-MMAP-GO`、`modernc.org/sqlite` の `SQLITE-LICENSE` 等を含める。
- `modernc.org/libc` の `musl_*.go` に埋め込まれた `copyright.c` の通知を保持し、配布対象間の差を確認する。`int128.go` の uint128 由来の MIT 通知と `libc_windows.go` の University of California の BSD 通知も含める。モジュールのルートライセンスだけでなく、生成コード等に埋め込まれた追加の著作権・許諾条件も確認する。
- バイナリには Go の runtime / 標準ライブラリも含まれる。使用する Go version の `GOROOT/LICENSE` / `PATENTS` と `GOROOT/src/vendor` 内のライセンス・特許通知を保持する。Homebrew 等で `LICENSE` が `GOROOT` にない場合は配布パッケージのルート、または公式 Go リポジトリの該当 version から取得する。Go version と出典も更新する。独自の build tag や外部コードを追加した場合はその条件も確認する。

通知全文と収集元を照合し、配布対象の依存に漏れがないことを確認して差分を読む。

## 配布時

バイナリの配布アーカイブに `LICENSE` と `THIRD-PARTY-NOTICES.txt` を同梱する。単体バイナリを配る場合も、両ファイルを配布物と一緒に提供する。通知ファイルは実際のビルドに使った依存・Go version に合わせる。

GitHub Release がソースのみを公開する場合も、タグに通知ファイルの更新が含まれることを確認する。
