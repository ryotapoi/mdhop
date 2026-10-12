---
generated_at: 2026-10-12
baseline_commit: 4b2d091ced6335c7b1e2afe85ce83550f6cf66d4
---

# meta-check ディレクトリ参照の重複走査（A11-C6）

一回の MetaCheck 内で既存 resolver を共有する変更を採用。不存在300参照の CLI 全体の待ち時間は中央値3.386秒から43ms（98.72%減）となり、ばらつきより十分大きい。resolver の map は従来同様に直接 stat 失敗時だけ作り、次回呼び出しには持ち越さない。永続 cache・追加依存・解決規則の変更はない。

## 条件と予算

macOS Darwin 27 arm64、Go 1.27.2、通常 `go build`（追加 gcflags / ldflags なし）。基準は上記 commit、候補 source の SHA256 は [生結果](meta-check-performance.json) に収録。測定は使い捨て Vault と明示 `--db`、DB は Vault 外。実ノートは使わない。合成入力・warm OS cache・コマンドごとに別 process。test と計測は直列。

pilot は251ノート・250 asset・130ディレクトリ（root除外）、各key30参照、3回。不存在の wall 中央値106ms、RSS最大17.6MB。代表測定は1,501ノート・1,500 asset・400ディレクトリ、各key300参照（計900参照）。通常ディレクトリ100個、NFD名 `Cafe\u0301NNNN` のディレクトリ300個。Index.md の missing / unicode / direct key にそれぞれ不存在末尾 `/`、NFC表記末尾 `/`、実在通常ディレクトリ末尾 `/` を300件置く。

予算は全計測15分、disk500MB、各CLI60秒timeout、pilot RSS1GB超なら拡大しない。代表fixtureを含む全artifactは約66MB。各入力をwarm-up後、旧新7回ずつ、実行順をAB/BAで交互にした。wall は Python monotonic で `time -l` processを含む、user/sysとpeak RSSはmacOS `time -l`。CPUは10ms単位のため短いコマンドの小差を判定しない。`time -l` の sysctl がsandbox拒否となった最初のpilotは破棄し、承認済みescalationで新規fixtureを作成して測り直した。

## 無計装CLI全体の比較

wall の中央値（最小–最大）、CPU user/sys中央値、peak RSS中央値を示す。RSSは MiB。

| 対象（各300参照） | 旧 wall 秒 | 新 wall 秒 | 旧 user/sys 秒 | 新 user/sys 秒 | 旧/新 RSS MiB |
|---|---:|---:|---:|---:|---:|
| 不存在 | 3.3861（3.3518–3.4952） | 0.04346（0.04166–0.05095） | 1.32 / 2.28 | 0.01 / 0.02 | 22.41 / 17.55 |
| Unicode表記差 | 0.03242（0.03195–0.03273） | 0.03230（0.03174–0.03290） | 0.01 / 0.01 | 0.01 / 0.01 | 16.34 / 16.36 |
| 直接stat | 0.03105（0.03078–0.03172） | 0.03101（0.03063–0.03225） | 0.01 / 0.01 | 0.01 / 0.01 | 16.33 / 16.25 |

Unicodeと直接statの差は測定ばらつき内。不存在はCPUとRSSも減った。RSSは最悪時の生存メモリの観測であり、累積allocationの測定ではない。新mapはコマンド終了まで保持するが、この規模で資源消費の悪化は見られない。

## 重複走査の寄与と限界

基準・候補の隔離ソースコピーにだけ計装し、`collectNormalizedDiskPaths` 入り口で回数を加算し `time.Now` / defer `time.Since` で累計時間を計測。MetaCheck終了時にstderrへ出した。repositoryのproductionには計装を残していない。

| 対象 | 旧走査回数 / 累計秒 | 新走査回数 / 累計秒 |
|---|---:|---:|
| 不存在 | 300 / 3.33912 | 1 / 0.01096 |
| Unicode表記差 | 0 / 0 | 0 / 0 |
| 直接stat | 0 / 0 | 0 / 0 |

計装CLIは各1回のみで旧3.789秒、新0.450秒。これらは無計装CLIの採否値に混ぜない。計装・初回起動等の負荷を個別分離していないため、走査秒の比をCLI高速化率として扱わない。300→1回という実測と既存関数の呼び出し経路から、重複一覧構築が主な削減対象と判断した。

macOSではNFDディレクトリにNFCパスで直接statが成功した。Unicode300参照は意味・出力の同等性と直接stat経路の確認に留まり、表記差fallbackの性能は未計測。Docker CLIはあるがdaemon socketがなく、Linuxは利用できなかった。Unicode表記を区別するfilesystem、cold cache、実運用の参照頻度、より大きなVault、同時編集は今回の測定で保証しない。Linux fallbackを含む恒久テストは保持しているが、今回のローカル実行でLinux成功を主張しない。

## 正しさと検証

変更前集中test成功（Go表示0.436秒）。変更後の全結果・コマンド・durationは生結果に収録。集中core 1.71秒、集中CLI 1.03秒、`go test ./...` 10.11秒、`go build ./...` 0.39秒、すべてexit 0。

恒久テストではpath/auto双方の直接stat、複数不存在、NFC参照/NFDディレクトリ、非ディレクトリ、escape、全issueフィールドと順序、次回呼び出しまでに作られたディレクトリを検証。既存suiteがURL/wikilink等を保護する。実バイナリでは3代表入力×path/auto×JSON/textの12組でstdout/stderr/exitが旧新完全一致、JSONのissue件数も確認。別の小fixtureでもURL、wikilink、ordinary prose、escape、非ディレクトリを混ぜたpath/auto JSON/textと、invalid kind、invalid format、keyなしの計7組が完全一致した。Vault全ファイルとDBのSHA256は旧新実行前後で不変。

## 再現

基準ソースを独立directoryへ `git archive 4b2d091ced6335c7b1e2afe85ce83550f6cf66d4` で展開し、そこで `go build -o /tmp/meta-check-baseline ./cmd/mdhop`。候補repositoryで `go build -o /tmp/meta-check-candidate ./cmd/mdhop`。branch / checkout / stashは不要。

```sh
python3 cache/measure-meta-check.py \
  --baseline /tmp/meta-check-baseline --candidate /tmp/meta-check-candidate \
  --out /tmp/meta-check-ab-20261012 --notes 1500 --assets 1500 --references 300 --repeats 7
```

outは存在しない新規directoryを指定する（既存vaultがあればscriptは停止）。Darwin専用の `time -l` が使える環境で実行。結果はout/results.json。pilotは `--notes 250 --assets 250 --references 30 --repeats 3`。計装版の作成手順と実行結果はworkflow artifact `.tmp/workflow/review-response-20261012-resume-02/changes/change-13/performance/instrument.py` / `representative/results.json`、追加のmixed fixtureは同directoryの `contract.py` / `supplemental-contracts.json`。計装は上記2関数にカウンタと経過時間を置くだけで、採否は無計装結果で行う。生結果の contracts には初回のkeyなしerror比較も残し、invalid kind/format の具体的な比較は supplemental_contracts に収録した。

採用理由は、待ち時間を約3.34秒削減し、直接stat対照を維持し、CPU/RSSに悪化がなく、既存resolverの寿命を一回のMetaCheckへ延ばすだけで達成できたため。この規模で効果は明確だが、利用者の実Vaultで常に同じ改善率になるとは判断しない。
