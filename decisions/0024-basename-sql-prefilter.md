---
status: current
---

# ADR 0024: basename 候補を SQL で保守的に絞る

## 判断時点と理由

2026-10-06、baseline `16ceea1c61cb1e491c6a2a7c3371f0d4b23d1126` に対して測定した。DB basename lookup は、対象 type の全行を Go に取り出して NFC 正規化・小文字化する転送と allocation が大きかった。索引の別状態を増やさず、確実に一致しない printable ASCII 行だけを SQL で除く案を採用した。

検索 key は従来の Go 正規化結果を渡し、`name = ? COLLATE NOCASE`、固定 `GLOB '*[^ -~]*'`、NUL を含む行のいずれかを満たす候補を従来の Go 比較に渡した。ASCII の小文字化は NOCASE と一致し、非 ASCII は絞り込まないので、旧 NFD と Kelvin sign / dotted I が ASCII になるケースも落とさなかった。検索名から SQL pattern を作らず、`%_*?[]` を literal として扱った。NUL より後を GLOB が検査しないため別条件を加え、Go 小文字化で replacement character になる invalid UTF-8 byte も保持した。

候補集合を決める正規化と、候補選択の一意・root 優先・note → asset → phantom 順序は既存処理に残した。判断の現在形は `internal/core/resolve.go` の共有 helper とコメント、回帰契約は `internal/core/basename_test.go` と既存 resolve/query test に置いた。

## 比較した案

| 案 | 効果と保守負担 | この時点の判断 |
| --- | --- | --- |
| Go 全行比較 | 互換性が明快だが、毎回全行の文字列転送・正規化・allocation がある | 同条件 baseline として測定 |
| printable ASCII の SQL prefilter | SQLite の対象 type 走査は残るが、ASCII 不一致行を Go に送らない。schema、移行、同期、接続登録を増やさない | 同条件で測定し採用 |
| text/byte 長差による prefilter | valid UTF-8 は扱えるが、単独 invalid byte では長さが同じになり、Go の replacement character への変換と一致しない候補を落とし得る | 初期測定後、完全性の理由で上記固定 GLOB 案に差し替え |
| SQL の name 完全一致 / LOWER / NOCASE への全面置換 | SQLite の ASCII case 比較だけでは Go NFC/Unicode lowercase と一致せず旧 NFD を落とす | 互換性の理由で不採用 |
| Go 正規化の custom SQL function | 永続状態は不要だが、登録の lifecycle・全接続/transaction の利用保証が必要。索引なしでは callback は全行分残る | 追加責務に対し局所案が十分だったため実装・測定しなかった |
| normalized column/index | indexed lookup が期待できるが、保存形式の変更・旧 NFD 互換・全 write 経路での整合が必要 | 移行と同期の負担を避け、実装・測定しなかった |
| 接続/cache | 繰り返し lookup は改善し得るが、初回全量読取・更新時の invalidation が必要。単発 CLI の利益が小さい | 実装・測定しなかった |

## 測定条件と再現

- Go 1.27.1、macOS 27.0.1 (26A434)、darwin/arm64、Apple M4 Pro、benchmark suffix 14 (GOMAXPROCS)。依存は modernc.org/sqlite v1.29.0、実行 SQLite 3.45.1、golang.org/x/text v0.14.0。
- `BenchmarkBasenameLookup` は initSchema で現在の schema/index を作り、transaction で合成 row を投入してから timed loop を開始する。背景 1千/1万/10万 row と target 0/1/2 row。背景名は `Note%06d`、NFC は `Café` prefix、NFD はその NFD 表現。target は `Target` / `Café`。note/asset の type は分けた。
- unique は subdirectory に一件、root は subdirectory と root に二件、ambiguous は別 subdirectory に二件、miss は target なし。target Unicode と背景 Unicode は別軸で、全 Unicode 背景も測った。
- 同じ DB に対し旧 helper と採用 helper を各 3 回、300ms の Go benchmark で測定した。旧 helper は benchmark file に `CHANGE-VERIFY:` 付きで残し、production の層は広げなかった。
- warm は同じ接続を再利用。open は毎 operation の DB open、最初の query、close を含む。いずれも file-backed DB を用い、OS cache を flush する cold I/O 測定ではない。全量 build や public CLI 全体の測定ではなく、単発接続による候補 lookup の測定である。

```sh
GOCACHE=/private/tmp/mdhop-basename-go-cache go test ./internal/core -run '^$' -bench '^BenchmarkBasenameLookup$' -benchtime=300ms -count=3
```

通常 GOCACHE が sandbox で拒否されたため writable cache を指定した。通常環境では GOCACHE 指定は不要。時間閾値の test/CI gate は追加していない。下表は各指標の 3 回の中央値で、同一 operation の ns/op・B/op・allocs/op を比較する。

| 背景 row / 背景名 / target / shape / type / 接続 | baseline ns/op | 採用 ns/op | baseline B/op | 採用 B/op | baseline allocs/op | 採用 allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1000 / ASCII / ASCII / unique / note / warm | 480,645 | 271,386 | 134,080 | 2,240 | 9,783 | 40 |
| 10000 / ASCII / ASCII / unique / note / warm | 4,707,613 | 2,589,864 | 1,358,086 | 2,240 | 99,783 | 40 |
| 100000 / ASCII / ASCII / unique / note / warm | 49,112,601 | 26,322,381 | 13,598,180 | 2,240 | 999,783 | 40 |
| 100000 / ASCII / ASCII / unique / note / open | 49,273,899 | 26,647,609 | 13,610,789 | 12,540 | 999,881 | 132 |
| 100000 / ASCII / NFC / root / note / warm | 48,979,423 | 26,358,106 | 13,598,237 | 2,400 | 999,793 | 50 |
| 100000 / ASCII / NFD / ambiguous / note / warm | 48,561,250 | 26,487,125 | 13,599,421 | 3,584 | 999,799 | 56 |
| 100000 / ASCII / ASCII / miss / note / warm | 48,656,976 | 26,292,237 | 13,597,979 | 2,088 | 999,772 | 29 |
| 100000 / NFC / NFC / unique / note / warm | 56,241,410 | 66,021,933 | 14,399,504 | 14,399,584 | 999,783 | 999,785 |
| 100000 / NFC / NFC / unique / note / open | 55,496,944 | 66,665,000 | 14,412,005 | 14,412,222 | 999,881 | 999,882 |
| 100000 / NFD / NFD / unique / note / warm | 73,892,517 | 86,397,396 | 75,200,275 | 75,200,324 | 1,299,786 | 1,299,789 |
| 100000 / NFD / NFD / unique / note / open | 74,229,075 | 86,392,625 | 75,212,948 | 75,212,784 | 1,299,885 | 1,299,886 |
| 100000 / ASCII / ASCII / unique / asset / warm | 49,326,077 | 26,287,718 | 13,598,080 | 2,240 | 999,783 | 40 |

SQLite 3.45.1 に採用 SQL の `EXPLAIN QUERY PLAN` を渡した結果は `SEARCH nodes USING INDEX idx_nodes_type_name (type=?)`。type の range scan は残り、name の index seek にはなっていない。ASCII 背景で Go へ送る行と allocation が減る改善である。全 Unicode 背景では Go 全行処理が残り、SQL 条件分の代償もある。旧10万 note の約56.5ms/opという別環境の数字はこの比較に用いなかった。

## 検証と限界

`TestBasenameUnicodeAndLiteralCandidates` は明示した ID 集合で ASCII、旧 NFD、Unicode case、Unicode→ASCII、literal、NUL、invalid UTF-8、miss を検査し、一意な対象は query の `findEntryByName` と DB basename resolve でも確認した。NFD target は rebuild や保存済み exact raw edge を使わず検索した。既存の resolve backend priority と query name/root/ambiguous/missing test により選択順序と error/fallback を検証した。

集中 test → `go test ./...` → `go build ./...` → `go vet ./...` は成功した。CLI の出力・終了コード・接続設定・schema・可視副作用を変えていないため追加の実バイナリ確認は不要とした。runtime raw log は当時の非永続作業ディレクトリに置いた（永続成果物には含めず、現在は残っていない）。測定要約と再現 command はこの記録と benchmark に残した。

実 Vault、他 OS、cold disk、更新との並行実行は未測定。合成名・全 ASCII/全 Unicode の極端な比率による性能傾向であり、実 Vault の性能保証ではない。DB 内で O(N) 走査が残ることと Unicode 背景の時間増を許容し、主対象の転送・allocation を大きく減らしながら移行・同期を増やさない点で採用した。

SQLite の [length/instr/GLOB の一次資料](https://sqlite.org/lang_corefunc.html) と [NOCASE の説明](https://sqlite.org/datatype3.html#collation) を確認した。pinned v1.29.0 の `lib/sqlite_darwin_arm64.go` の `_lengthFunc` / `_nocaseCollatingFunc` と実 SQLite の回帰結果を照合した。長さだけの案が invalid UTF-8 を除外し得る点は、この実装の byte counting と Go の実際の lookup を根拠にした。
