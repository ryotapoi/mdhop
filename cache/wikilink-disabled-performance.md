---
generated_at: 2026-10-11
source_commit: 97b76612410d78fa8f5e8ceb379fe6dff8dacc20
---

# wikilink 明示無効化候補の差分 build 比較

明示無効化設定は見送る。wikilink を使わない入力では、既定の自動差分が既に basename 依存の再評価を省いており、今回の無効候補で省けた依存確認・リンク再解決は 0 件だった。代表 35 ノートの既定差分と候補の中央値差は −0.47～+0.20 ms。1,500 ノート・約 12 MiB の無変更は 36.31 / 35.80 ms、本文 1% は 52.65 / 52.80 ms で範囲が重なる。設定と各適用経路を増やす負担に見合う操作全体の待ち時間削減は確認できない。

## 候補と入力の範囲

[候補 patch](wikilink-disabled-candidate.patch) は本文の wikilink / embed 抽出と mask、frontmatter wikilink 抽出、reference 継続判定内の wikilink mask を省く。DB・依存 dispatcher・再解決・Markdown / 画像 / asset・meta.link_keys raw path は既定と同じ。製品設定の parse や伝搬費用を持たないため、この省略だけの効果を比較する限定候補である。設定切替・rewrite・wikilink を含む入力での完全な無効化契約は実装・検証していない。採用せず、本番 Go・config・README/help には変更を残していない。

代表入力は当該コミットの実開発 repository の `docs/*.md` と `decisions/*.md` の隔離コピー。`[[` が一つでもあるファイルを除外し、35 ノート・138,497 bytes・asset 0。全ノートの文字列検査で本文・frontmatter・code block を含め `[[` 不在を確認する。除外9ファイルと選択35ファイルは [raw 結果](wikilink-disabled-product.json) の selected / excluded_wiki_literal に保存した。README・backlog・testdata・source code は選定対象外。一般的な開発 repository 全体ではなく、設計・運用文書の部分集合であり、多量のコードや添付資料がある用途は未確認。

synthetic は 64 ノート・41,718 bytes と 1,500 ノート・12,189,390 bytes、各 asset 1（7 bytes の SVG）。20 directory、一意名、frontmatter rank、tag、既存の次ノート・未作成 Future への root Markdown path、SVG画像を持つ。nominal 本文 768 / 8,192 bytes を141-byteの行で埋める。本文変更は追記して size を確実に変える。追加は最初に Future を実在化し、削除・移動はソート順の先頭を対象にする。移動の新 path は inventory で検出される。1% は代表と64では1ノート、1500では15ノート。50% は代表17、64では32、1500では750ノート。raw basename 補助は64ノート・42,742 bytesと同じ画像に `related: Future` と `meta.link_keys = ["related"]` を加える。synthetic を実 repository の結果とは扱わない。

## 条件と再現

macOS 27.0.1 / darwin arm64、Go 1.27.1、modernc.org/sqlite v1.29.0。標準最適化の CLI、`GOCACHE=/private/tmp/mdhop-go-cache`、`GOFLAGS=-buildvcs=false`（VCS module cache 書込制約回避）。計測はテスト・別計測と直列、OS cache は flush せず、各方式を1回 warmup 後に5組。full / enabled / disabled の順を組ごとに反転する。開始 / 終了の load averages は9.98/10.61/10.25、9.54/10.48/10.23で同居負荷がある。静止・専用機の測定ではない。

3方式とも各回同じ初期全量 DB を計時外で復元し、同じ変更後 filesystem に対して実行。DB は Vault 外の明示パス。CLI 起動・設定読込・変更検出・DB 状態読込・複製・更新・原子的公開まで壁時計時間に含む。復元・入力生成・意味比較・診断は計時外。各commandは60秒、全体8分を上限として小規模smokeから開始し、非0終了・stdout混入・意味不一致・予算超過で中止する。smokeは2組で成功。本計測28ケースは55.25秒で成功。最終候補fullの追加意味確認28件は5.91秒で成功。5組は小差を採用するための精密推定ではなく、実用的な差がないという今回の見送り判断の範囲である。

```sh
python3 cache/prepare-wikilink-candidate.py .tmp/_go/wikilink-disabled
python3 cache/prepare-wikilink-candidate.py .tmp/_go/wikilink-enabled-trace --diagnostic --enabled
python3 cache/prepare-wikilink-candidate.py .tmp/_go/wikilink-disabled-trace --diagnostic
GOCACHE=/private/tmp/mdhop-go-cache GOFLAGS=-buildvcs=false go build -o .tmp/wikilink-enabled ./cmd/mdhop
GOCACHE=/private/tmp/mdhop-go-cache GOFLAGS=-buildvcs=false go -C .tmp/_go/wikilink-disabled build -o ../../wikilink-disabled ./cmd/mdhop
GOCACHE=/private/tmp/mdhop-go-cache GOFLAGS=-buildvcs=false go -C .tmp/_go/wikilink-enabled-trace build -o ../../wikilink-enabled-trace ./cmd/mdhop
GOCACHE=/private/tmp/mdhop-go-cache GOFLAGS=-buildvcs=false go -C .tmp/_go/wikilink-disabled-trace build -o ../../wikilink-disabled-trace ./cmd/mdhop
python3 cache/measure-wikilink-disabled.py .tmp/wikilink-enabled .tmp/wikilink-disabled .tmp/wikilink-enabled-trace .tmp/wikilink-disabled-trace 2 --smoke
python3 cache/measure-wikilink-disabled.py .tmp/wikilink-enabled .tmp/wikilink-disabled .tmp/wikilink-enabled-trace .tmp/wikilink-disabled-trace 5
```

当該コミットで実行する。temporary directory に product.json が出力される。source準備は [prepare-wikilink-candidate.py](prepare-wikilink-candidate.py)、測定は [measure-wikilink-disabled.py](measure-wikilink-disabled.py)、diagnosticの土台は既存 [instrument-build-incremental.py](instrument-build-incremental.py)。最終壁時計値はinstrumentationなし。今回の全生データ・binary SHA256・diagnosticはraw結果に保存した。既存31ケースの履歴は変更していない。

## 壁時計時間

ms、中央値 [最小–最大]。full は同じ既定コードの `build --full`、enabled は設定なし・wikilink有効の自動差分、disabled は上記隔離候補の差分。差は enabled中央値 − disabled中央値、正値が候補短縮。

| 入力・変更 | full | enabled | disabled | 差 ms |
| --- | ---: | ---: | ---: | ---: |
| representative-body-0 | 17.76 [16.08–18.40] | 8.28 [7.60–9.74] | 8.08 [7.96–8.88] | +0.20 |
| representative-body-0.01 | 16.48 [16.36–16.91] | 9.40 [8.97–9.76] | 9.53 [9.38–9.76] | -0.13 |
| representative-body-0.5 | 17.22 [16.61–17.62] | 12.58 [12.02–13.22] | 12.79 [12.19–13.83] | -0.21 |
| representative-add-0.01 | 16.90 [16.50–17.18] | 9.20 [8.91–9.52] | 9.67 [9.34–9.94] | -0.47 |
| representative-delete-0.01 | 15.72 [15.58–17.17] | 9.06 [8.81–9.50] | 9.03 [8.65–9.20] | +0.03 |
| representative-move-0.01 | 16.00 [15.70–16.95] | 9.48 [9.20–9.97] | 9.54 [9.20–9.88] | -0.06 |
| n64-b768-body-0 | 21.74 [21.53–22.86] | 10.30 [9.88–10.43] | 10.46 [10.10–13.29] | -0.16 |
| n64-b768-body-0.01 | 22.79 [21.51–32.30] | 13.19 [11.49–16.73] | 12.17 [11.51–13.20] | +1.02 |
| n64-b768-body-0.5 | 22.59 [22.30–22.98] | 18.08 [17.93–18.78] | 18.27 [17.66–18.63] | -0.19 |
| n64-b768-add-0.01 | 21.74 [21.32–22.37] | 17.87 [17.27–18.16] | 17.74 [16.99–18.04] | +0.13 |
| n64-b768-add-0.5 | 24.72 [23.22–27.13] | 21.22 [20.33–21.80] | 21.91 [20.61–24.36] | -0.69 |
| n64-b768-delete-0.01 | 21.78 [21.63–22.35] | 11.86 [11.71–12.30] | 12.28 [11.52–12.77] | -0.42 |
| n64-b768-delete-0.5 | 18.83 [18.29–19.26] | 14.57 [14.36–15.05] | 14.68 [14.27–14.73] | -0.12 |
| n64-b768-move-0.01 | 23.43 [22.54–24.50] | 12.98 [12.68–13.70] | 12.90 [12.30–13.05] | +0.07 |
| n64-b768-move-0.5 | 22.48 [22.08–23.37] | 20.44 [20.33–21.36] | 20.24 [19.79–21.07] | +0.20 |
| n1500-b8192-body-0 | 356.83 [349.97–370.43] | 36.31 [35.24–37.07] | 35.80 [35.55–37.51] | +0.51 |
| n1500-b8192-body-0.01 | 361.98 [355.70–413.14] | 52.65 [51.52–53.30] | 52.80 [51.28–55.64] | -0.15 |
| n1500-b8192-body-0.5 | 372.87 [370.87–376.74] | 271.87 [269.59–278.84] | 265.37 [265.00–307.66] | +6.50 |
| n1500-b8192-add-0.01 | 363.77 [353.42–405.51] | 182.76 [181.03–206.22] | 183.35 [181.38–183.83] | -0.60 |
| n1500-b8192-add-0.5 | 377.29 [371.23–394.60] | 248.14 [244.91–255.00] | 247.93 [244.60–249.77] | +0.20 |
| n1500-b8192-delete-0.01 | 358.26 [349.26–363.43] | 50.61 [49.50–51.82] | 50.61 [50.43–51.06] | -0.00 |
| n1500-b8192-delete-0.5 | 185.69 [185.06–190.83] | 104.34 [102.91–106.99] | 104.65 [101.71–106.63] | -0.31 |
| n1500-b8192-move-0.01 | 359.94 [357.05–373.92] | 56.54 [54.99–66.31] | 56.05 [55.33–65.84] | +0.49 |
| n1500-b8192-move-0.5 | 363.31 [357.75–369.00] | 323.24 [320.09–334.96] | 329.22 [324.32–336.87] | -5.98 |
| raw-basename-body | 29.04 [24.18–30.83] | 13.43 [12.72–14.63] | 12.70 [12.18–17.25] | +0.73 |
| raw-basename-add | 24.70 [22.92–25.99] | 20.03 [19.08–21.70] | 20.02 [18.65–24.86] | +0.01 |
| raw-basename-delete | 24.44 [24.07–24.69] | 12.42 [12.29–13.03] | 12.56 [12.48–13.88] | -0.14 |
| raw-basename-move | 27.36 [24.76–27.84] | 14.42 [13.26–16.66] | 15.41 [13.96–17.77] | -0.99 |

## 仕事量・意味一致と採否

全28ケースの enabled / disabled の ReadParseNotes、DependenciesChecked、BasenameEvaluations、ResolvedLinks が一致した。DependenciesChecked は saved build_links の distinct dependency を旧新で比較した個数、basename 評価は dependency 解決と virtual name 選択の lookup 回数、resolve は edge生成の resolveLink 呼出数。これらは時間比較とは別の単発診断である。

| ケース | read/parse | 依存確認 | basename 評価 | link 再解決 |
| --- | ---: | ---: | ---: | ---: |
| representative-body-0 | 0 | 0 | 0 | 0 |
| representative-body-0.01 | 1 | 0 | 0 | 1 |
| n1500-b8192-body-0 | 0 | 0 | 0 | 0 |
| n1500-b8192-body-0.01 | 15 | 0 | 0 | 75 |
| n1500-b8192-body-0.5 | 750 | 0 | 0 | 3750 |
| n1500-b8192-add-0.01 | 15 | 1502 | 0 | 6000 |
| n1500-b8192-delete-0.01 | 0 | 1502 | 0 | 60 |
| n1500-b8192-move-0.01 | 15 | 1502 | 0 | 120 |
| raw-basename-body | 1 | 0 | 0 | 6 |
| raw-basename-add | 1 | 67 | 2 | 320 |
| raw-basename-delete | 0 | 67 | 2 | 5 |
| raw-basename-move | 1 | 67 | 2 | 10 |

無変更・本文のみの集合不変時は既定方式で依存確認0。通常Markdownだけの追加・削除・移動で basename評価0だが、pathの実在状態変更の依存確認は必要であり、無効化で除けない。候補でのみ省いた仕事は変更ノートのwikilink抽出/maskで、read/parse対象数は減らない。raw basenameの追加では両方式ともbasename評価2・resolve320、Future参照先がphantomから実在noteへ昇格する。本文では依存確認0、削除・移動ではbasename評価2を保つ。無効化宣言から全リンクをpathと仮定する処理は入れていない。

各測定組と診断の全node属性・edge属性・metaを数値IDを除いて既定fullと比較し一致した。候補fullも全28ケースで一致した。raw path追加時の参照先が `note / Future.md`、その他の補助ケースがphantomであることを別にassertした。正しさのbaselineは計測前の `go test ./internal/core/ -run 'TestBuildIncremental|TestBuildFingerprintAndDependencies'`（exit0、package実行0.859秒）。ビルド4件、smoke、本計測、候補full追加確認は全てexit0。

大規模本文50%で中央値6.50 ms（2.4%）の短縮はあるが、enabled269.59–278.84ms、disabled265.00–307.66msで重なり、pairedの一組では候補が35.59ms遅い。高率移動は逆に中央値5.98ms遅い。代表・頻繁な無変更/低変更率では改善の一貫性がなく、依存確認・再解決を追加で除ける原因もない。解析微差だけを製品設定採用の根拠にしないという受入条件から見送る。設定なしの既存差分だけで必要な依存省略が既に得られ、今回追加の本番最適化は行わない。

cold cache、巨大規模、network filesystem、長期反復、CPU時間・ピークmemory、wikilinkを含む入力の完全な無効化契約は未確認。Ubuntu CIも未実行であり、macOSのこの限定比較から外挿しない。候補の未実装経路を「効果なし」とは扱わず、想定用途のbuildと省略可能な依存仕事について設定を導入する根拠がないという判断である。
