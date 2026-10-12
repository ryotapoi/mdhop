---
generated_at: 2026-10-12
baseline_commit: edf1db44a1a5ab91a71e4e9d5abac89c4254a323
candidate_code_diff_sha256: 5b2770ef9a7133f2ae70ad325134c1f844eaf5c8e6e7a9371746dea3df6ba581
---
# A1-03: テンプレート移動の重複準備

通常実行の事前 Plan を dry-run 分岐へ移す候補を採用した。20 件・200 件で全体待ち時間と CPU が減り、ピーク RSS は悪化しなかった。core の事前検査・rewrite・rollback は変更せず、CLI の不要呼出と二結果の選択を減らす。単一ファイルの速度差は断定しない。

## 条件

- macOS Darwin 27.0.0 / arm64、Go 1.27.1。標準 `go build`、無計装 CLI。baseline は上記 commit の `git archive` から別 tree へ展開した。candidate は `cmd/mdhop/move.go` と直接 test の差分。
- 合成 Vault: 1 / 20 / 200 件の `src/Nxxxx.md`。各 note は client・updated frontmatter、30 行の本文、移動先の外部 Target.md への相対リンク。Index.md は全 source を wikilink で参照する。単一は file 指定、20/200 は directory 指定。展開先は `archive/{client}/{updated:year}/{basename}`。
- 初期 DB は baseline の build で作成し、同じ vault root / DB path に同じ DB bytes と file mtime を毎回復元。復元・入力生成・build・結果検査は計時外。両 binary を一回ずつ warmup 後、7 対を直列で測定、各対の A/B 順を交互に反転。cache flush はせず warm OS cache 条件。tests は同時実行しない。
- 予算は入力総量 1 GiB 未満、各 command 60 秒以内。小規模から予備測定 2 対を実行し、最大 run が 0.2 秒未満だったため 7 対へ進めた。最大本計測 run は 0.173 秒未満。容量予算・timeout・異常終了・件数不整合を中止条件とした。
- Python perf_counter で CLI 起動から終了回収まで測り、`os.wait4` で process CPU（user+system）と peak RSS を取得。終了を 1 ms 間隔で観測するため約 1 ms の量子化・scheduler 誤差がある。Darwin の RSS は bytes。 `/usr/bin/time -l` は sandbox の sysctl 制限で失敗したため除外し、両版とも上記方式で最初から測り直した。

## 無計装 CLI の比較

壁時計は中央値 [最小–最大]、paired 差は同一対の baseline − candidate。各規模 7 対。

| 件数 | baseline (ms) | candidate (ms) | paired 差 (ms) | 中央値の減少 |
|---:|---:|---:|---:|---:|
| 1 | 10.23 [9.31–10.55] | 9.25 [8.84–10.46] | 0.94 [-0.05–1.34] | 9.5% |
| 20 | 23.22 [22.62–26.47] | 18.13 [17.94–19.38] | 5.20 [3.24–8.33] | 21.9% |
| 200 | 161.16 [152.34–172.31] | 117.33 [104.93–123.82] | 40.96 [35.01–56.23] | 27.2% |

20 / 200 件では全 7 対で candidate が速く、両版の全体時間の範囲も重ならない。200 件の paired 差の中央値は 40.96 ms。単一の paired 差には負値もあり、約 1 ms の差は観測分解能に近い。20/200 件での約 22% / 27% の待ち時間減少と実装の単純化に基づいて採用した。単一の表の割合を確実な効果として使わない。

| 件数 | CPU baseline→candidate (ms、中央値 [範囲]) | peak RSS baseline→candidate (MiB、中央値 [範囲]) |
|---:|---:|---:|
| 1 | 8.59 [8.37–9.62] → 7.89 [7.60–8.29] | 13.72 [13.31–13.84] → 13.39 [13.16–13.64] |
| 20 | 24.14 [23.34–26.62] → 17.25 [17.14–19.29] | 17.05 [16.58–17.52] → 16.17 [16.03–16.48] |
| 200 | 172.44 [165.07–183.14] → 124.97 [112.92–131.43] | 22.97 [22.56–23.20] → 21.70 [21.42–21.98] |

## 捨てる計画の寄与

隔離 baseline source の一時 probe が公開 `PlanMoveTemplate` 全体を呼び、time.Since と runtime.MemStats.TotalAlloc 差を取る。各 run 別 process、GC と初期 ReadMemStats は計時前、終了 ReadMemStats は計時後。runtime instrumentation と別 process の warmup/GC 条件が CLI とは異なる補助値であり、A/B の速度値に足さない。累積割当はピーク保持メモリではない。

| 件数 | Plan 時間 (ms、中央値 [範囲]) | Plan 累積割当 (MB、中央値 [範囲]) |
|---:|---:|---:|
| 1 | 1.080 [1.051–1.287] | 0.154 [0.154–0.154] |
| 20 | 5.368 [5.263–6.173] | 1.432 [1.432–1.432] |
| 200 | 46.215 [45.168–59.505] | 13.502 [13.487–13.529] |

200 件の Plan は baseline CLI の中央値の約 29% 相当。CLI 差と同じ桁で、重複準備の除去という原因仮説を支持する。Plan の累積割当が消えることと peak RSS の差は別の指標であり、13.5 MB 分 RSS が減ったとは解釈しない。新たな cache・状態・並列化は追加していない。

## 正しさと制約

- 変更前および変更後の CLI/core 集中 tests、`go test ./...`、`go build ./...`、実バイナリ build は成功。unsafe rewrite・destination collision・stale・index 保護・rollback の既存 tests を含む。format 呼出の形式・struct tag 等は変えていないため追加 go vet は不要。
- 計測 run は全て正常終了し、JSON の移動件数・rewrite 非空、全 file 移動、incoming/outgoing rewrite、DB の destination 登録と source 不在を検査。1/20/200 件の実バイナリ JSON dry-run は rewritten `[]` と file/DB SHA256 不変を確認。missing template field 拒否は exit 非0、stdout 空、stderr の理由と file/DB SHA256 不変を確認。単一・directory の実バイナリ text 通常/dry-run も件数・行先・副作用を確認した。
- 実 Vault 分布、冷 cache、巨大規模、Linux、他 hardware、同時 index 更新、故障注入の実バイナリ試験は未計測。故障時保証は変更していない core の既存 tests による。新しい性能閾値・内部呼出回数の恒久 test は作らない。
- 初回 Go build cache 拒否は `GOCACHE=/private/tmp/mdhop-a103-gocache` で解消した。mod version stat cache の書込警告は一部 build に出たが exit 0 で binary が生成され、実行成功した。sysctl で hardware details を取れなかったため型番・搭載 RAM は未確認。

## 再現

repository root で次を実行する。`RUN` は専用の使い捨て directory。既存 directory を指定しない。

```sh
RUN=$(mktemp -d "$PWD/.tmp/a103.XXXXXX")
mkdir -p "$RUN/baseline"
git archive edf1db44a1a5ab91a71e4e9d5abac89c4254a323 | tar -x -C "$RUN/baseline"
mkdir -p "$RUN/baseline/.tmp/_go/plan"
cp cache/a1-03/plan-probe.go.txt "$RUN/baseline/.tmp/_go/plan/main.go"
(cd "$RUN/baseline" && GOCACHE=/private/tmp/mdhop-a103-gocache go build -o "$RUN/baseline-mdhop" ./cmd/mdhop)
(cd "$RUN/baseline" && GOCACHE=/private/tmp/mdhop-a103-gocache go build -o "$RUN/plan-probe" ./.tmp/_go/plan)
GOCACHE=/private/tmp/mdhop-a103-gocache go build -o "$RUN/candidate-mdhop" ./cmd/mdhop
python3 cache/a1-03/measure.py --baseline "$RUN/baseline-mdhop" --candidate "$RUN/candidate-mdhop" --probe "$RUN/plan-probe" --output "$RUN/results" --repeats 7
```

入力生成・復元・集計は [measure.py](measure.py)、probe は [plan-probe.go.txt](plan-probe.go.txt)。本計測の元証拠は `.tmp/workflow/review-response-20261012-resume-02/changes/change-12/performance/final/{commands,results,summary,text-verification}.json`、tests/build は同 Change directory の `verification.json`。一時 artifact はコミットしない。
baseline binary SHA256: `178dba52805202ceff11cc2a7bb5b1778f265463448999e66c48d8dcf49e1f74`。candidate binary SHA256: `6ac95cd131e06c7eee7ef255aa8dea3f96a308a6022dae4da7a50f36c7f1c479`。
