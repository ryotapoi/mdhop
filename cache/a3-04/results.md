# A3-04: build・preview のディレクトリ対応再利用

## 比較対象と仮説

基準は `54fb0473ddfd9cf2695c40f20cfcf040570afa51`。A3-02 後の resolver は Vault 全体の対応表を持たず、path component ごとに `ReadDir` と NFC 正規化を反復していた。同じ操作で unchanged directory の対応を再利用し、ファイル本文の読取りと検証を省略せずに待ち時間を減らす。

候補は build の入力採取と本文、query の head/snippet、search の head に操作寿命 resolver を共有する。各 lookup で followed directory の `SameFile`、nanosecond mtime、size、mode を確認し変化時に再構築する。公開前の入力再採取は独立した fresh resolver。mutation は従来の uncached resolver。snippet の既存 body cache と head の再読取りはそのまま。

## 入力・手順・予算

`measure.py BEFORE AFTER OUTPUT [PROBE_BEFORE PROBE_AFTER]` は実 CLI の時間と `/usr/bin/time` の peak RSS を測る。`instrument.py SOURCE_COPY` は隔離コピーの util.go にのみ `ReadDir` 回数・処理 entries 記録を加える。instrumented binary は時間比較に使わない。production API/hook は増やしていない。

- 各環境内の両版を同じ toolchain で `go build ./cmd/mdhop` の通常設定でビルド。実際の Go version は各環境の結果欄に記載。並列計測なし。
- 80ノートの予備観測後、4ディレクトリに1,600ノート＋Indexノートを配置。各ノートは `[[Index]]` を一つ持つ。NFC の exact 対照と NFD の実名を比較。
- full build、変更なし build、1ノート変更 build、1,600件の backlinks query(head 2行＋snippet 0行)、1,601件の search(head 2行)。両版ウォームアップ後、3反復 A/B・B/A・A/B。
- 各操作の前に同じ基準 DB を復元。full は DB を削除、少数差分は同じ本文と署名へ変更。DB は Vault 外、設定・リンク・初期ファイル署名は共通。
- command 起動から終了までの壁時計秒。JSON hash、全 table の内容 hash、note/edge 件数を各成功 run で比較。全 measured run は exit 0。
- 操作30秒、全体900秒、child peak RSS512MiBを超えれば中止。fixture/output の見積りは100MiB未満、生成物は専用 directory 内。GitHub job 上限20分。

## macOS ローカル結果

環境: `macOS-27.0.1-arm64-arm-64bit` / `go version go1.27.2 darwin/arm64`。NFD実名は存在し NFC direct lookup も成功する alias filesystem。Ubuntu を代替しない。

|入力|操作|before 中央値 [範囲] 秒|after 中央値 [範囲] 秒|before/after 最大RSS MiB|
|---|---|---:|---:|---:|
|exact-1600|full|1.4048 [1.2555–1.8794]|0.1428 [0.1270–0.1565]|28.14 / 26.55|
|exact-1600|unchanged|0.7287 [0.6950–0.7419]|0.0442 [0.0441–0.0457]|20.20 / 19.45|
|exact-1600|changed|0.7107 [0.7071–0.7322]|0.0533 [0.0526–0.0540]|21.67 / 20.81|
|exact-1600|query|0.8417 [0.8140–0.8698]|0.1318 [0.1267–0.1963]|23.39 / 22.67|
|exact-1600|search|0.4320 [0.4150–0.7057]|0.0650 [0.0643–0.0704]|19.66 / 19.06|
|nfd-1600|full|1.7571 [1.7277–1.8069]|0.1289 [0.1231–0.1310]|27.42 / 26.28|
|nfd-1600|unchanged|1.0333 [1.0227–1.0391]|0.0447 [0.0443–0.0453]|20.14 / 19.84|
|nfd-1600|changed|1.0357 [1.0149–1.1145]|0.0546 [0.0531–0.0598]|21.91 / 20.17|
|nfd-1600|query|1.1948 [1.1917–1.2015]|0.1430 [0.1389–0.1636]|23.75 / 22.53|
|nfd-1600|search|0.5508 [0.5441–0.5803]|0.0655 [0.0655–0.0707]|20.34 / 19.30|

full の実名解決 ReadDir は9,603→10、NFC正規化対象 entries は1,944,015→3,210。各 directory は入力採取と独立した最終検証の各一回。query は6,400→5、entriesは1,296,000→1,605で、各 directory 一回。exact/NFD とも同じ回数。実 CLI 全体の時間差は両版の測定範囲が重ならず、局所削減だけを高速化の根拠にしていない。

raw records、各出力/DB hash、RSS、directory ごとの回数は [macos-product.json](macos-product.json)。累積 allocation は測定していない。操作中に参照 directory の名前表を保持する代償はあり、今回の peak RSS は増加していない。

## Ubuntu CI

候補 snapshot `ff059602dc05e0ebe70cdf46b8369f81c1a71258` の [CI run](https://github.com/ryotapoi/mdhop/actions/runs/38416617279) は Ubuntu/macOS 全 suite・Unicode 集中 suite・Ubuntu paired measurement の全3 jobsが成功。performance job は両OS test完了後に直列実行。

計測環境: Ubuntu24 image `20261004.327.1`、ext4 (`/dev/sda1`)、`Linux-6.17.0-1022-azure-x86_64-with-glibc2.39`、`go version go1.27.1 linux/amd64`。NFD実名は存在し NFC direct lookup は80/1,600両入力で失敗することを観測。新規5回帰テストは Ubuntu で全PASS、skip 0。macOS15 CIでは新規collision2件がaliasによりskip、他はPASS。

|入力|操作|before 中央値 [範囲] 秒|after 中央値 [範囲] 秒|before/after 最大RSS MiB|
|---|---|---:|---:|---:|
|exact-1600|full|1.5033 [1.5013–1.5068]|0.2397 [0.2355–0.2402]|24.92 / 25.16|
|exact-1600|unchanged|0.9113 [0.9076–0.9135]|0.0727 [0.0725–0.0730]|19.39 / 18.48|
|exact-1600|changed|0.9368 [0.9352–0.9370]|0.0975 [0.0949–0.0984]|20.94 / 20.41|
|exact-1600|query|1.1260 [1.1251–1.1342]|0.2451 [0.2417–0.2459]|22.07 / 21.02|
|exact-1600|search|0.5363 [0.5360–0.5386]|0.0992 [0.0989–0.1018]|19.23 / 18.49|
|nfd-1600|full|2.6062 [2.6016–2.6765]|0.2509 [0.2434–0.2582]|24.52 / 24.87|
|nfd-1600|unchanged|1.6356 [1.6258–1.6528]|0.0773 [0.0758–0.0787]|19.27 / 19.56|
|nfd-1600|changed|1.6547 [1.6461–1.6643]|0.0986 [0.0975–0.0990]|19.38 / 19.49|
|nfd-1600|query|1.8735 [1.8728–1.8741]|0.2503 [0.2483–0.2543]|22.41 / 20.40|
|nfd-1600|search|0.9062 [0.9021–0.9068]|0.1018 [0.1009–0.1043]|19.53 / 20.38|

NFD入力の full は2.6062→0.2509秒（90.4%減）、query は1.8735→0.2503秒（86.6%減）。3反復の範囲は重ならない。exact対照・差分build・searchも操作全体で改善。対応表再構築回数と正規化 entries はmacOSと同じ削減で、input採取と公開前のfresh確認を省略していない。全 measured output/DB hash一致、note1,601・edge1,600と全返却head/snippetを検証。

cache による名前表保持と各lookupのdirectory Statが代償。Ubuntuでは一部peak RSSが増え、最大差はNFD searchの約0.85MiB（両版各3runの最大値同士）、full NFDは約0.36MiB。最大child RSSは26MiB未満で512MiB予算内。数秒単位の待ち時間削減と局所的な状態管理に見合うため採用。累積allocationや真のlive heapの増減は未測定でRSSからは推定しない。

raw records・環境・出力hash・directory回数は [ubuntu-product.json](ubuntu-product.json)。実行手順は専用CI overlayでbaseline/candidateを `git archive` から別directoryへ展開し、両版通常binaryとartifact-only probe binaryを各ビルド後、同じ `measure.py` を実行した。公開workflowには計測jobを恒久追加せず、回帰test filterのみ更新。

runnerではNFC/NFD実例・旧DB保持・symlink retargetがすべて実行され、完了条件を満たした。CI snapshot と最終workspaceのproduction/tests/依存は照合済みで、snapshot専用計測段取りと後追いの記録/backlogのみ差がある。

## 検証と限界

変更前集中 suite は成功。変更後集中 suite、全 Go tests、go build、go vet は成功。名前追加/削除/rename、symlink retarget、exact+Unicode collision、共有 head の stale/missing と旧DB保持を回帰テストで確認。macOS の新規collision2件はaliasのためskipし、Ubuntuでは5新規回帰テストすべてPASS。

代表合成入力・warm filesystem cache の比較であり、大規模実Vault、cold disk、極端に多いディレクトリ、並行編集の完全検知には外挿しない。metadata の同署名復元・時刻分解能以下の変更、最終確認後の編集、filesystem snapshot/lock は既存保証の対象外。
