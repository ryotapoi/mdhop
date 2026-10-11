---
generated_at: 2026-10-11
source_commit: aa13f3339422293f7d309f884424029f164c9ca7
candidate: 同コミットに自動差分 build の未コミット差分を適用
---

# build 自動差分更新の比較

1,500 ノート・約 12 MiB の無変更 build は旧全量方式 277.49 ms に対し auto 37.00 ms（7.50 倍）、wikilink 本文 1% 変更は 310.52 / 51.06 ms（6.08 倍）。代表の repository 文書 45 ノート・189,725 bytes の無変更は 17.64 / 8.96 ms（1.97 倍）。変更検出・virtual node 名の整合処理を含む操作全体の壁時計時間であり、ノートの未読・未解析は診断用コピーと権限を外した未変更ノートの test で別途確認した。

## 条件と再現

macOS 27.0.1 / darwin arm64、Go 1.27.1、modernc.org/sqlite v1.29.0。CPU の詳細取得は sandbox が拒否し、architecture のみ確認。標準最適化の CLI（`GOFLAGS=-buildvcs=false` は VCS module cache 書込制約の回避）、OS cache は flush せず warmup 後に baseline / new full / auto の順序を組ごとに反転。計測・test は直列。各 command は最大 60 秒、計測全体は最大 8 分、不正な終了・stdout・意味不一致で中止する。最終 31 ケースを各 3 組、合計 61.67 秒で完了した。

代表入力は `docs/`・`decisions/`・`README.ja.md` のコピー。synthetic は 64 / 1,500 ノート × nominal 768 / 8,192 bytes（本文生成は 141 bytes の行単位、実際の合計は raw JSON）× Markdown path / wikilink basename × 本文変更 0 / 1 / 50%。追加・削除・移動も大規模・大本文で各 1% 比較した。20 親 directory に一意な note 名、frontmatter rank、tag、一件の既存参照先と全ソースが参照する未作成 Future を含む。Future の追加は未変更 1,500 ソースの再解決を必要にする。利用者の実 Vault は測っていない。

DB は Vault 外の独立した明示パス。auto は各 run の前に同じ初期全量 DB を復元し、同じ変更後 filesystem に対して実行する。snapshot 復元と入力生成は計時外、CLI 起動・設定読込・二回の一覧/署名確認・DB状態読込・必要な複製/解析/更新/公開は計時内。各組の baseline / full / auto を全 node 属性・全 edge 属性・全 meta（数値 ID を除く）で比較し全組一致した。全量性能の最終値には診断 instrumentation を使っていない。

```sh
mkdir -p .tmp/build-baseline
git archive aa13f3339422293f7d309f884424029f164c9ca7 | tar -x -C .tmp/build-baseline
GOCACHE=/private/tmp/mdhop-go-cache GOFLAGS=-buildvcs=false go -C .tmp/build-baseline build -o ../../bin/mdhop-baseline ./cmd/mdhop
GOCACHE=/private/tmp/mdhop-go-cache GOFLAGS=-buildvcs=false go build -o bin/mdhop ./cmd/mdhop
python3 cache/instrument-build-incremental.py .tmp/_go/build-instrumented
GOCACHE=/private/tmp/mdhop-go-cache GOFLAGS=-buildvcs=false go -C .tmp/_go/build-instrumented build -o ../../../bin/mdhop-instrumented ./cmd/mdhop
python3 cache/measure-build-incremental.py bin/mdhop-baseline bin/mdhop bin/mdhop-instrumented
```

再現 script が表示する temporary directory の `product.json` と stdout に個別 run が残る。今回の raw 結果は [build-incremental-product.json](build-incremental-product.json) に保存した。診断用 source は計測 script と別の [instrument-build-incremental.py](instrument-build-incremental.py) で再生成でき、本番コードに計測 API は追加しない。計測 binary の SHA-256 は以下。

- `mdhop-baseline`: `09a485f75be355d2bc0e99a00385897c22229db073e3c872933f8ffaaa77322e`
- `mdhop`: `1271e98f7034af414d7e0adbe83b907c634b52b9c1abb070822d8f0c5ff8c500`
- `mdhop-instrumented`: `20c2527988d21bc50ed9872dcc141f3e3a267752906e64b9d8b88b6004ad326f`

## 壁時計時間

単位 ms、中央値 [最小–最大]。full は新しい保存形式を含む `--full`、baseline は変更前の全量方式。

| 入力・変更 | baseline | full | auto |
| --- | ---: | ---: | ---: |
| representative-docs | 17.64 [16.39–18.14] | 19.36 [19.18–19.96] | 8.96 [8.48–10.16] |
| n64-b768-path-body-0 | 17.99 [17.59–18.00] | 22.32 [21.88–23.57] | 10.22 [10.16–10.24] |
| n64-b768-path-body-0.01 | 16.76 [16.56–17.59] | 20.95 [20.59–22.58] | 11.72 [11.54–12.48] |
| n64-b768-path-body-0.5 | 16.75 [16.65–17.39] | 21.48 [21.19–21.77] | 17.36 [17.18–17.74] |
| n64-b768-wiki-body-0 | 16.90 [16.51–17.33] | 21.65 [21.08–22.98] | 10.01 [9.93–10.91] |
| n64-b768-wiki-body-0.01 | 16.55 [16.40–16.88] | 20.60 [20.30–21.39] | 11.15 [11.07–11.36] |
| n64-b768-wiki-body-0.5 | 18.56 [17.36–27.20] | 21.76 [20.96–21.88] | 17.04 [16.93–17.26] |
| n64-b8192-path-body-0 | 24.33 [22.58–34.32] | 27.31 [27.18–27.41] | 9.84 [9.59–10.28] |
| n64-b8192-path-body-0.01 | 24.67 [24.29–25.09] | 28.92 [28.16–29.33] | 12.20 [12.20–12.53] |
| n64-b8192-path-body-0.5 | 23.54 [22.82–24.19] | 28.09 [27.63–28.70] | 20.25 [19.61–20.39] |
| n64-b8192-wiki-body-0 | 23.39 [22.78–24.09] | 27.40 [27.37–28.49] | 9.85 [9.64–10.09] |
| n64-b8192-wiki-body-0.01 | 23.63 [22.85–24.82] | 27.71 [27.52–28.47] | 12.03 [11.90–12.27] |
| n64-b8192-wiki-body-0.5 | 24.10 [23.34–24.50] | 29.47 [28.26–30.54] | 21.21 [20.71–23.05] |
| n1500-b768-path-body-0 | 145.11 [141.39–150.18] | 194.67 [192.45–195.85] | 34.90 [34.66–35.19] |
| n1500-b768-path-body-0.01 | 144.74 [143.25–146.63] | 199.32 [198.79–199.84] | 49.26 [49.03–50.77] |
| n1500-b768-path-body-0.5 | 154.66 [154.55–160.21] | 209.57 [207.44–231.08] | 183.99 [182.58–185.48] |
| n1500-b768-wiki-body-0 | 142.93 [142.84–145.07] | 195.94 [195.05–197.35] | 35.55 [35.04–36.96] |
| n1500-b768-wiki-body-0.01 | 144.46 [143.07–145.94] | 199.52 [191.23–201.74] | 50.42 [49.43–80.95] |
| n1500-b768-wiki-body-0.5 | 158.62 [158.02–162.67] | 213.74 [211.01–214.93] | 189.31 [184.83–189.92] |
| n1500-b8192-path-body-0 | 277.49 [277.34–295.54] | 330.53 [329.15–373.91] | 37.00 [35.59–37.59] |
| n1500-b8192-path-body-0.01 | 287.16 [282.25–308.92] | 353.26 [336.10–356.34] | 51.85 [51.39–52.86] |
| n1500-b8192-path-body-0.5 | 290.91 [288.64–294.10] | 342.08 [341.91–349.71] | 254.83 [252.10–256.88] |
| n1500-b8192-wiki-body-0 | 275.70 [275.22–283.07] | 328.01 [325.87–329.12] | 35.40 [35.24–35.60] |
| n1500-b8192-wiki-body-0.01 | 310.52 [275.62–418.16] | 337.22 [325.89–387.13] | 51.06 [50.63–51.82] |
| n1500-b8192-wiki-body-0.5 | 289.79 [286.54–299.40] | 361.13 [349.31–364.14] | 257.70 [255.85–308.62] |
| n1500-b8192-path-add | 273.35 [267.30–273.72] | 327.02 [320.50–353.51] | 169.87 [169.49–171.79] |
| n1500-b8192-path-delete | 276.89 [269.98–334.89] | 338.38 [327.13–339.30] | 50.88 [50.32–51.98] |
| n1500-b8192-path-move | 278.68 [278.06–314.14] | 343.32 [330.67–344.09] | 53.61 [53.60–54.24] |
| n1500-b8192-wiki-add | 265.01 [263.36–272.67] | 322.15 [315.92–323.75] | 165.84 [165.62–168.70] |
| n1500-b8192-wiki-delete | 272.29 [272.26–286.16] | 329.01 [323.55–330.52] | 50.11 [48.33–52.91] |
| n1500-b8192-wiki-move | 285.83 [278.12–313.24] | 330.86 [325.80–332.00] | 54.02 [52.65–54.48] |

## 段階費用と省略の証拠

診断用 binary の単発値は profiler 相当の原因確認であり、上記 CLI 比較とは別に扱う。Inventory は初回と最終確認の合計、State は schema/fingerprint/保存情報の読込、Dependency は依存選択まで（後続の edge 再解決/書込は Total に含まれる）。Names は virtual node の最初の残存ソース・保存リンクから表記を選び直す処理で、旧/新の変更ソースの virtual key 選択 SQL は別途 Total に含まれる。basename 評価は依存選択と name 選択の lookup 数で、edge resolve 数は別列。

| ケース | inventory ms | state ms | copy ms | dependency ms | names ms | read/parse note | resolve link | basename 評価 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| representative-docs | 1.45 | 1.19 | 0.00 | 0.00 | 0.00 | 0 | 0 | 0 |
| n1500-b8192-path-body-0 | 22.93 | 5.06 | 0.00 | 0.00 | 0.00 | 0 | 0 | 0 |
| n1500-b8192-wiki-body-0.01 | 23.00 | 5.16 | 0.67 | 0.00 | 2.20 | 15 | 60 | 6 |
| n1500-b8192-path-add | 23.71 | 4.96 | 0.65 | 2.24 | 1.10 | 15 | 4500 | 0 |
| n1500-b8192-wiki-add | 23.39 | 5.41 | 0.65 | 2.36 | 1.11 | 15 | 4500 | 3004 |
| n1500-b8192-path-delete | 24.33 | 5.38 | 0.74 | 1.23 | 2.73 | 0 | 45 | 0 |
| n1500-b8192-wiki-move | 22.72 | 5.32 | 1.02 | 1.28 | 2.21 | 15 | 90 | 3006 |

補助 full trace では代表 45、大規模 1,500、追加後 1,515 の全 note を read/parse した。auto 無変更は read/parse・link resolve・copy とも 0、1% 本文変更は 15 note のみを parse し、対象集合が不変なので依存再評価は 0。表記整合処理は同ケース Names 2.20 ms。Markdown path の追加では未変更ソースの再解決と name 選択があっても basename 評価は 0。wiki の Future 追加は依存選択で distinct basename 1,501 個を旧新で 3,002 回比較し、name 選択も含む合計 3004 回、15 note の parse と 4,500 link resolve を実施した。

試行時に nil/empty の asset 集合比較が本文変更でも依存走査を起こすことを trace で確認し、集合比較を等価に修正した。その後、最初の表記を持つソースの削除・変更・前順追加で tag/phantom 名が全量と異なる問題を回帰 test で確認し、変更に接する virtual key だけ最終 inventory 順と保存 link 順から名前を選び直した。最終表はこの修正後に両版を同条件で全 31 ケース再測定した値。件数の初回記載 55 は誤りで、保存結果・script・表の実件数は 31。CPU 詳細取得の権限制約で report 最終化に失敗した試行は採否根拠に使わない。

## 代償・判断・限界

新 full は保存リンク・署名・最終入力確認の費用があり、無変更大本文/path の baseline 277.49 ms に対し 330.53 ms（19.1% 増）。同ケース DB は 724,992 → 2,064,384 bytes（2.85 倍）。本文を DB に保存せず、完全な解析済み link occurrence とファイル情報を保存する容量との交換である。差分 DB は再生成時より freelist が増え得る（大本文 path 50%: full 2,289,664 / auto 2,301,952 bytes；正確なケース別サイズは raw 結果）。長期間の反復更新・cold cache・network filesystem・更に大きな Vault・ピーク memory / CPU 時間は測っていない。

高変更率・小本文は旧全量方式に対する利益が小さく、name 整合処理の追加費用もある。1,500/path/768 bytes/50% は baseline 154.66 ms と auto 183.99 ms。ケース別の範囲を示し、小差や速度比を一般化しない。新 full に対する auto の低変更率・無変更の省略と、旧全量方式に対する代表・大本文の待ち時間削減を確認したため採用。すべての変更割合で旧方式より速いことは主張せず、固定の速度合格閾値は設けない。

同一 mtime/size の変更検出・filesystem snapshot の完全性は保証対象外。macOS 実 SQLite tests・実 binary で semantic matrix、失敗時旧 DB、temp cleanup、旧reader/新reader、全量化操作を確認した。`go test ./...` / `go build ./...` / `go vet ./...` は成功。Ubuntu の CI はこの未コミット段階では実行できず未確認。
