---
generated_at: 2026-10-11
source_commit: 5d312874455121bfd6eeacdd0b03fc724c800175
candidate: 同コミットに索引除外判定の操作単位再利用を加えた未コミット差分
---

# 索引除外判定の比較

操作ローカルな matcher を採用。選択 DB の配置解決は最初の候補で一度、候補親の symlink 解決は親ごとに一度行い、末尾 entry は追跡しない。`build`・`status` の note / asset で共有する。scan rewrite と add / delete / move の直接の一括判定にも適用し、mutation 前の検証と後の cleanup は別の matcher を使う。

## 条件と再実行

macOS 27.0.1 (26A434)、Apple M4 Pro、darwin/arm64、Go 1.27.1。標準の `go build` を使い、profiling instrumentation は入れていない。OS cache は flush せず、両 binary を操作ごとに warmup した。A/B 順を組ごとに反転、計測・test は直列。各 command は最大 60 秒、異常終了または不正な出力で中止する。

代表 Vault は `docs/`・`decisions/`・`README.ja.md` のコピー、42 note / 2 親 directory。大規模は 15,000 note・5,000 asset / 200 親 directory、note に frontmatter・wikilink・tag を含む。既定 cache と Vault 内明示 DB を別々に比較した。明示 DB の自己除外を含め、どちらも測定対象は同じ入力数。fresh temporary directory 内で既存 index を再生成し、各組は同じ入力・有効な完成済み DB から始まる。build の成功・stdout が空、status の JSON が全配列空であることを確認した。

再実行は repository root で以下を順に実行する。baseline は git archive へ展開し、既存差分は切り替えない。

```sh
mkdir -p /private/tmp/mdhop-placement-baseline-src
git archive 5d312874455121bfd6eeacdd0b03fc724c800175 | tar -x -C /private/tmp/mdhop-placement-baseline-src
cd /private/tmp/mdhop-placement-baseline-src
GOCACHE=/private/tmp/mdhop-go-cache go build -o /private/tmp/mdhop-placement-before ./cmd/mdhop
cd /Users/ryota/Sources/ryotapoi/mdhop
GOCACHE=/private/tmp/mdhop-go-cache go build -o /private/tmp/mdhop-placement-after ./cmd/mdhop
python3 cache/measure-index-files.py /private/tmp/mdhop-placement-before /private/tmp/mdhop-placement-after
```

script は fresh temporary directory を作り、先頭の `artifacts:` に場所を表示する。個々の run は stdout とその場所の `product.json` に残る。今回の product artifact は `/var/folders/1y/l1zx_f3n7s987jz5n6hhfd2c0000gn/T/mdhop-placement-uyt14hkk/product.json`。既定 Go cache は sandbox で書けなかったため `GOCACHE` を指定した。build は module stat cache の書込拒否を stderr に出したが exit 0 で binary を生成した。

## 製品の壁時計時間

値は ms、中央値 [最小–最大]。代表は 10 組、大規模は 3 組。

| Vault | DB | 操作 | 変更前 | 変更後 | 中央値の削減 |
| --- | --- | --- | ---: | ---: | ---: |
| 42 note | 既定 | build | 18.339 [16.642–21.164] | 15.664 [14.615–18.002] | 14.6% |
| 42 note | 既定 | status | 11.259 [9.897–13.215] | 8.797 [8.032–10.503] | 21.9% |
| 42 note | 明示 | build | 17.597 [16.509–19.223] | 17.240 [15.676–19.723] | 未確定 |
| 42 note | 明示 | status | 8.970 [8.720–9.133] | 7.817 [7.281–8.149] | 12.9% |
| 20,000 entry | 既定 | build | 2231.687 [2114.839–2340.031] | 1223.437 [1210.831–1494.992] | 45.2% |
| 20,000 entry | 既定 | status | 1119.956 [1076.669–1139.575] | 192.948 [187.652–196.036] | 82.8% |
| 20,000 entry | 明示 | build | 1770.953 [1738.920–1802.868] | 1243.042 [1180.021–1281.586] | 29.8% |
| 20,000 entry | 明示 | status | 755.535 [734.246–762.808] | 196.432 [196.347–197.356] | 74.0% |

代表既定の paired 差 (前−後) は build 中央値 1.798 ms [0.875–5.939]、status 2.155 ms [1.141–4.460]、両操作とも 10/10 組で改善した。大規模既定の paired 差は build 904.008 ms [845.039–1008.250]、status 923.920 ms [883.721–951.923]、両操作とも 3/3 組で改善した。代表明示 build は 2/10 組で逆転し、速度改善を断定しない。

初回の予備 run では candidate 起動に約 384 ms が混じったため採否に使わず、上表は両 binary を warmup して fresh fixture で測り直した結果だけを使う。親 path の深さは symlink 解決の仕事量に影響する。予備の `/private/tmp` 配置と最終の `TMPDIR` 配置の数値は混ぜていない。

## 配置除外判定のコスト

`BenchmarkFilterIndexFiles` は 42 件 / 2 親、20,000 件 / 200 親の synthetic candidate を同じ `filterIndexFiles` に渡す。DB 親 directory は timer の外で作成。file 内容・walk・parse・SQLite は含まず、配置解決・比較・結果 slice の割当を計時する。入力の全件が結果に残ることを確認する。製品時間とは別の測定である。

baseline source に同一 benchmark を置き、両方の test binary を作る。

```sh
cp internal/core/index_files_test.go /private/tmp/mdhop-placement-baseline-src/internal/core/
cd /private/tmp/mdhop-placement-baseline-src
GOCACHE=/private/tmp/mdhop-go-cache go test -c -o /private/tmp/mdhop-placement-before.test ./internal/core
cd /Users/ryota/Sources/ryotapoi/mdhop
GOCACHE=/private/tmp/mdhop-go-cache go test -c -o /private/tmp/mdhop-placement-after.test ./internal/core
/private/tmp/mdhop-placement-before.test -test.run='^$' -test.bench='^BenchmarkFilterIndexFiles$' -test.benchtime=3x -test.count=1 -test.timeout=60s
/private/tmp/mdhop-placement-after.test -test.run='^$' -test.bench='^BenchmarkFilterIndexFiles$' -test.benchtime=3x -test.count=1 -test.timeout=60s
```

最後の 2 command を A/B、B/A、A/B の 3 組実行する。以下は各 run の ns/op を ms に変換した中央値 [最小–最大] と、累積 allocation の中央値。ピーク保持メモリではない。

| 件数 | DB | 変更前 ms/op | 変更後 ms/op | B/op 前→後 | allocs/op 前→後 |
| --- | --- | ---: | ---: | ---: | ---: |
| 42 | 既定 | 2.615 [2.065–2.732] | 0.126 [0.110–0.196] | 622,989→68,400 | 5,965→433 |
| 42 | 明示 | 1.478 [1.253–3.580] | 0.096 [0.092–0.127] | 355,520→48,928 | 3,655→378 |
| 20,000 | 既定 | 1038.883 [1024.060–1042.774] | 20.947 [19.827–21.032] | 297,484,984→24,852,330 | 2,840,433→128,515 |
| 20,000 | 明示 | 632.108 [603.733–647.088] | 20.466 [19.254–20.485] | 169,927,978→18,439,757 | 1,740,004→128,453 |

明示 DB でも改善したので、既定 Vault hash の再計算だけが原因ではない。変更前は選択 DB 親と候補親の `EvalSymlinks` も各 entry で繰り返す。変更後は DB 配置 (既定時の Vault 実体・hash を含む) を matcher ごとに一度、候補親を一親ごとに一度解決する。これは実装から確認した仕事量で、個別 syscall の実測回数ではない。両 DB モードの差を hash 単独の処理時間とは扱わない。

操作中は親数に比例する map を追加保持するが、上表の累積 allocation は両規模・DB モードで減った。操作をまたぐ cache や失効管理は追加していない。ピーク RSS、ネットワーク filesystem、Linux 実行、2 万件を超える規模、全 entry が別親の配置は未計測。末尾 symlink と親 alias の契約は test で確認する。

## 正しさの検証

- 変更前集中 test: `GOCACHE=/private/tmp/mdhop-go-cache go test ./internal/core -run 'Test(Build|Status|.*Index|.*Symlink|.*DiskOnly|Delete|Move|Scan)'`、exit 0、package 2.867 秒。
- 変更後集中 test: 同じ対象に `FilterIndex` を加え `-timeout=60s`、exit 0、package 2.586 秒。
- 新規契約 test: 親 alias / symlink root と末尾 symlink entry の区別、SQLite 補助ファイルの自己除外、別操作で親 symlink の変更を再解決。基準版でも新規 test は pass し、既存契約であることを確認した。
- 既存の build / status / scan・登録済み index の mutation 拒否・directory cleanup / move・root symlink・Vault 外保護を集中 test と全体 test で確認した。
- 最終全体 test: `GOCACHE=/private/tmp/mdhop-go-cache go test ./... -timeout=60s`、exit 0、core 5.227 秒、CLI は前 run の成功結果を cache (前 run 1.515 秒)。
- 最終 build: `GOCACHE=/private/tmp/mdhop-go-cache go build ./...`、exit 0、wall 0.219 秒。
- 計測後の実 binary `stats --format json` で既定 / 明示の双方を確認: 代表は既存 note 42 / asset 0 / edge 21、大規模は既存 note 15,000 / asset 5,000 / edge 30,000。自己除外で入力やリンクを省略して速くした結果ではない。再実行 script にも計時後の件数確認を置いた。
- `git diff --check`、exit 0。Linux の実行は今回行っておらず、両 OS の CI 結果は未確認。

両規模の既定 `build`・`status` の実行時間と、両 DB モードの配置除外コストで改善が再現したため採用する。明示 DB の小規模 build は速度差を断定せず、残る制限として保持する。
