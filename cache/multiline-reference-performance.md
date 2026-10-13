---
generated_at: 2026-10-13
generated_from_commit: 2ec8056bcbde488c526a38286f1734c764cf04dd
historical_commit: e7a77c249e8c1e15bafe257a11b490732e06f8c1
---

# A4-06 長い複数行参照段落の計測

A4-03 (`7b975c9`) が除いた行ごとの再探索を実測した。今回の本番 parser 変更はない。旧版 `7b975c9^` と現行の比較、および今回の baseline/final 比較を分ける。baseline と final は上記生成 commit の独立した archive/build で、本番ソースは同一。

## 条件と予算

macOS 27.0.1 (26A434)、darwin/arm64、Apple M4 Pro、Go 1.27.2。既定の最適化、race/profiler なし、GOMAXPROCS 既定14、各実行に `GOMEMLIMIT=512MiB`。他 agent の CPU-heavy 作業を停止して直列で測定。入力生成・期待値検証は parser 計時外。

事前予算は全工程15分、1 process60秒、temporary disk512MiB、process address space1GiB。sandbox で RLIMIT_AS を設定できず、実行前に Go の soft memory limit 512MiBへ変更した。これは hard limit や RSS 上限を保証しない。`/usr/bin/time -l` も sysctl 権限で peak RSS を取得できなかったため、保持メモリのピークは未確認。時間超過・非zero終了・期待出力不一致は中止条件であり、中断値を集計しない。disk実測最大367.2MiB、二回の計測処理は14.85秒・13.14秒、build合計約11秒。上限に達した正常計測はない。

予備128行 `1x` は本集計に含めない。最初のCLI反復は各版の初回起動が316–362ms、以降11–13msであったため、全版を同じ方法で再測定した。以下は再測定の全反復（外れ値削除なし）。初回起動の差を parser 改善に帰属させない。OS page cache は温まった状態で、冷たい disk cache は未測定。

## 入力と同じ仕事の確認

1ノートの同一段落を128/512/2048物理行で構成する。中間行は24 ASCII bytes、先頭行は `[abcdefghijklmnopqrstuvw`、末尾行は `abcdefghijklmnopqrstuvwx][id]`。段落外に独立した `[known](target.md)` を置く。未定義 `id` は抽出しないため、旧新版とも独立リンク1件、正規化 target `target`、完全rawLink、開始/終了行 `n+2` の一致をassertした。総入力byte数は以下。

| 段落行数 | bytes |
| ---: | ---: |
| 128 | 3225 |
| 512 | 12825 |
| 2048 | 51225 |

現行のみ、同じ長い段落の後ろに `[id]: target.md` を置いた128/512/2048行もassertした。参照リンクと独立リンクの2件、参照リンクの完全原文、target `target`、lineStart=1/lineEnd=nを確認。旧版は有効な複数行参照の抽出文法が異なるため、このケースを旧新版の同一結果の速度比較には含めない。

## parser 全体

`testing.Benchmark` の ns/op、B/op、allocs/op。closed は5反復、controlsは512行のみ3反復。各反復の順は old/baseline/final と final/baseline/old を交互にする。各processは `-test.benchtime=100ms`。時間欄は中央値 [最小, 最大]、割当欄は中央値 [最小, 最大]。B/opは累積割当でありpeak保持量ではない。

| input | 行数 | 版 | ns/op | B/op | allocs/op |
| --- | ---: | --- | ---: | ---: | ---: |
| closed | 128 | old | 823899 [811807, 836070] | 706676 [706674, 706676] | 34840 [34840, 34840] |
| closed | 128 | baseline | 70556 [69783, 71308] | 130696 [130696, 130696] | 1603 [1603, 1603] |
| closed | 128 | final | 70626 [69334, 71924] | 130699 [130696, 130702] | 1603 [1603, 1603] |
| closed | 512 | old | 12493204 [12046185, 12947894] | 10691800 [10691747, 10692361] | 532506 [532506, 532507] |
| closed | 512 | baseline | 271600 [268403, 292961] | 517771 [517770, 517784] | 6217 [6217, 6217] |
| closed | 512 | final | 277387 [270683, 291347] | 517771 [517770, 517782] | 6217 [6217, 6217] |
| closed | 2048 | old | 203081833 [190599833, 375218625] | 168628336 [168627264, 168635752] | 8421412 [8421406, 8421440] |
| closed | 2048 | baseline | 1140912 [1124658, 1158436] | 2137930 [2137926, 2137978] | 24658 [24658, 24658] |
| closed | 2048 | final | 1102630 [1094110, 1114420] | 2137926 [2137925, 2137983] | 24658 [24658, 24658] |
| unclosed | 512 | old | 240922 [239687, 245748] | 210043 [210033, 210043] | 8731 [8731, 8731] |
| unclosed | 512 | baseline | 289429 [283287, 297874] | 517746 [517746, 517759] | 6216 [6216, 6216] |
| unclosed | 512 | final | 283380 [278133, 284239] | 517746 [517746, 517758] | 6216 [6216, 6216] |
| normal | 512 | old | 189576 [187773, 192185] | 169161 [169153, 169162] | 6687 [6687, 6687] |
| normal | 512 | baseline | 286240 [279223, 287632] | 517747 [517746, 517758] | 6216 [6216, 6216] |
| normal | 512 | final | 280245 [279537, 295058] | 517747 [517746, 517759] | 6216 [6216, 6216] |

行数4倍時の中央値増加比は旧版 15.16倍・16.26倍、final 3.93倍・3.98倍。2048行の旧版/final時間比は 184.2倍。旧版のほぼ二次的な増加が、この測定範囲で現行ではほぼ線形へ抑えられた。

旧版 `parseLinks` は pendingReference が残る各物理行で `referenceContinuationCloses` を呼び、段落の残りをstrip/maskし直す。現行 `walkBodyParagraphs` → `parseBodyMarkdown` → `scanMarkdownBoundaries` は段落の括弧対応表を一度作り、`markdownSpans` が対応を使用する。時間・allocationの増加傾向はこの仕事の除去と整合する。profilerによる各helperのCPU内訳は採取していないため、厳密なCPU割合は未確定。

normalは冒頭・末尾とも通常文字列、unclosedは冒頭bracketのみで閉鎖しないcontrol。現行ではこれらのB/opが旧版より増え、時間も悪化する。段落の原文・clean・対応表を持つ資源交換条件があり、一般の複数行リンクや段落すべての高速化を意味しない。baseline/finalはallocationが同等。2048行parserの再測定ではfinal中央値がbaselineより3.4%小さく範囲も重ならなかったが、最初の反復では範囲が重なりfinal中央値が1.4%大きかった。本番ソースが同一で差の方向も再現しないため、今回の性能改善とは扱わない。旧版2048行には375msの反復も含め、除外していない。

## 実CLI `build --full`

2ノート（sourceとtarget）、各版専用Vault/DB。各反復でDB・sidecarを削除し、DBなしから開始。入力は上記closedで、targetノートは `target\n`。process起動から成功終了までのwall timeをPython monotonicで測る。DB初期化・入力読取・解析・resolve・索引publish・起動固定費を含む。DB検証と削除は計時外。5反復、同じ交互順序。

全45実行がexit 0。全実行で実在nodesは source.md/target.md の2件、edgesはrawLink `[known](target.md)`、line_start=line_end=n+2 の1件をSQLiteからassertした。文法差やDB形式差を共通DBで混在させない。

| 行数 | 版 | wall ms 中央値 [最小, 最大] |
| ---: | --- | ---: |
| 128 | old | 12.553 [11.863, 28.518] |
| 128 | baseline | 11.599 [11.121, 26.959] |
| 128 | final | 12.653 [11.203, 26.735] |
| 512 | old | 24.556 [23.852, 25.348] |
| 512 | baseline | 11.861 [11.529, 12.196] |
| 512 | final | 11.887 [11.439, 11.996] |
| 2048 | old | 202.395 [200.829, 206.076] |
| 2048 | baseline | 13.500 [12.982, 14.375] |
| 2048 | final | 13.424 [12.698, 14.193] |

2048行のCLI実測比は 15.08倍。小規模では約11–13msの起動/DB等の固定費が局所改善を薄める。parserの高速化率をCLIへ外挿していない。baseline/finalの範囲は重なり、今回の差は性能改善と扱わない。

## 正しさと再実行

計測前に `go test ./internal/core/ -run 'Test(Markdown|ParseCodeFence|ParseInlineCode|StripInlineCode|Multiline|Reference)' -count=1` がexit 0（package 0.798秒、wall約1秒）。既存のLF/CRLF、コード保護、未閉鎖回復、段落/fence/参照定義境界を確認した。一時harnessの `TestA406` は全3版でexit 0。作成時の期待targetは `target.md` から既存正規化に合わせ `target` へ訂正し、その失敗値を性能値に混ぜていない。

以下の二つのscriptを `.tmp/_go/a406/prepare.py` と `measure.py` に保存し、repository rootから実行する（Python 3.9以上、依存Go modulesは導入済み）。`prepare.py` はread-only `git archive` から3個の隔離snapshotを作る。finalは本番ソースが同一のbaseline commitを使う。使い捨てGo sourceは隔離snapshot内だけに置く。

```sh
mkdir -p .tmp/_go/a406
GOCACHE="$PWD/.tmp/_go/a406/go-cache" python3 .tmp/_go/a406/prepare.py
python3 .tmp/_go/a406/measure.py
cp .tmp/_go/a406/measurements.json .tmp/_go/a406/initial-measurements.json
python3 .tmp/_go/a406/measure.py
```

build計時はベンチ値に含めない。module stat cacheへのsandbox書込警告はあったが各buildはexit 0。使い捨てsnapshot/source/binary/cacheを削除しても、この再現手順と結果は残る。

<details>
<summary>prepare.py（入力・期待値・build）</summary>

```python
import pathlib,subprocess,tarfile,io,time,json
root=pathlib.Path.cwd(); dest=root/'.tmp/_go/a406'
versions={'old':'e7a77c249e8c1e15bafe257a11b490732e06f8c1','baseline':'2ec8056bcbde488c526a38286f1734c764cf04dd','final':'2ec8056bcbde488c526a38286f1734c764cf04dd'}
harness=r'''package core
import("testing";"strings";"strconv";"os")
var perfSink parseResult
func perfInput(n int, kind string) string { lines:=make([]string,n); for i:=range lines {lines[i]="abcdefghijklmnopqrstuvwx"}; if kind!="normal" {lines[0]="[abcdefghijklmnopqrstuvw"}; if kind=="closed" || kind=="defined" {lines[n-1]="abcdefghijklmnopqrstuvwx][id]"}; s:=strings.Join(lines,"\n")+"\n\n[known](target.md)\n"; if kind=="defined" {s+="\n[id]: target.md\n"};return s }
func perfCheck(t testing.TB,s string,n int,kind string) {r:=parseLinks(s); want:=1;if kind=="defined" {want=2};if len(r.Links)!=want {t.Fatalf("links=%+v",r.Links)}; l:=r.Links[len(r.Links)-1];if l.target!="target"||l.rawLink!="[known](target.md)"||l.lineStart!=n+2||l.lineEnd!=n+2 {t.Fatalf("known=%+v",l)};if kind=="defined" {l=r.Links[0];raw:=strings.Split(s,"\n\n")[0];if l.target!="target"||l.rawLink!=raw||l.lineStart!=1||l.lineEnd!=n {t.Fatalf("reference=%+v",l)}} }
func TestA406(t *testing.T){for _,n:=range []int{128,512,2048}{for _,k:=range []string{"closed","unclosed","normal"}{perfCheck(t,perfInput(n,k),n,k)};if os.Getenv("PERF_VERSION")!="old" {perfCheck(t,perfInput(n,"defined"),n,"defined")}}}
func BenchmarkA406(b *testing.B){n,_:=strconv.Atoi(os.Getenv("PERF_N"));k:=os.Getenv("PERF_KIND");s:=perfInput(n,k);perfCheck(b,s,n,k);b.SetBytes(int64(len(s)));b.ReportAllocs();b.ResetTimer();for i:=0;i<b.N;i++ {perfSink=parseLinks(s)}}
'''
logs=[]
for name,rev in versions.items():
 p=dest/name;p.mkdir(exist_ok=True); data=subprocess.check_output(['git','archive',rev]);tarfile.open(fileobj=io.BytesIO(data)).extractall(p);(p/'internal/core/a406_test.go').write_text(harness)
 for cmd in [['go','test','-c','-o',str(dest/(name+'.test')),'./internal/core'],['go','build','-o',str(dest/(name+'.cli')),'./cmd/mdhop']]:
  start=time.monotonic();r=subprocess.run(cmd,cwd=p,capture_output=True,text=True,timeout=60);logs.append({'command':cmd,'version':name,'exit_code':r.returncode,'duration_s':time.monotonic()-start,'output':r.stdout+r.stderr});assert r.returncode==0,logs[-1]
(dest/'builds.json').write_text(json.dumps(logs,indent=2));print(json.dumps(logs,indent=2))
```

</details>

<details>
<summary>measure.py（実行・初期化・集計元JSON）</summary>

```python
import subprocess,pathlib,time,os,json,sqlite3
p=pathlib.Path(__file__).parent.resolve(); rows=[]; start=time.monotonic()
def run(cmd,env=None):
 assert time.monotonic()-start<900,'total budget exceeded'
 t=time.monotonic();r=subprocess.run(list(map(str,cmd)),env=dict(env or os.environ,GOMEMLIMIT='512MiB'),capture_output=True,text=True,timeout=60);row={'command':list(map(str,cmd)),'exit_code':r.returncode,'duration_s':time.monotonic()-t,'output':r.stdout+r.stderr};assert r.returncode==0,row;return row
for v in ['old','baseline','final']:
 row=run([p/(v+'.test'),'-test.run=TestA406','-test.v'],dict(os.environ,PERF_VERSION=v));rows.append(dict(row,stage='correctness',version=v))
# Preliminary observation, excluded from repeated statistics.
for v in ['old','baseline']:
 row=run([p/(v+'.test'),'-test.run=^$','-test.bench=BenchmarkA406$','-test.benchtime=1x'],dict(os.environ,PERF_VERSION=v,PERF_N='128',PERF_KIND='closed'));rows.append(dict(row,stage='pilot',version=v,n=128,kind='closed'));print(v,row['output'],flush=True)
for kind,ns,repeats in [('closed',[128,512,2048],5),('unclosed',[512],3),('normal',[512],3)]:
 for n in ns:
  for rep in range(repeats):
   order=['old','baseline','final'] if rep%2==0 else ['final','baseline','old']
   for v in order:
    row=run([p/(v+'.test'),'-test.run=^$','-test.bench=BenchmarkA406$','-test.benchtime=100ms'],dict(os.environ,PERF_VERSION=v,PERF_N=str(n),PERF_KIND=kind));rows.append(dict(row,stage='parser',version=v,n=n,kind=kind,rep=rep))
  (p/'measurements.json').write_text(json.dumps(rows,indent=2));print('parser',kind,n,'done',flush=True)
for n in [128,512,2048]:
 for rep in range(5):
  for v in (['old','baseline','final'] if rep%2==0 else ['final','baseline','old']):
   vault=p/('vault-'+v);vault.mkdir(exist_ok=True);lines=['abcdefghijklmnopqrstuvwx']*n;lines[0]='[abcdefghijklmnopqrstuvw';lines[-1]='abcdefghijklmnopqrstuvwx][id]';content='\n'.join(lines)+'\n\n[known](target.md)\n';(vault/'source.md').write_text(content);(vault/'target.md').write_text('target\n');db=p/(v+'.sqlite')
   # Each full build starts with no DB. Each version owns its vault and DB.
   for f in p.glob(v+'.sqlite*'): f.unlink()
   row=run([p/(v+'.cli'),'build','--full','--vault',vault,'--db',db]);c=sqlite3.connect(db);actual=c.execute('SELECT raw_link,line_start,line_end FROM edges').fetchall();nodes=c.execute('SELECT path FROM nodes WHERE exists_flag=1 ORDER BY path').fetchall();c.close();assert actual==[('[known](target.md)',n+2,n+2)],actual;assert nodes==[('source.md',),('target.md',)],nodes
   rows.append(dict(row,stage='cli',version=v,n=n,kind='closed',rep=rep,index=actual,nodes=nodes,bytes=len(content)))
 (p/'measurements.json').write_text(json.dumps(rows,indent=2));print('cli',n,'done',flush=True)
print('total seconds',time.monotonic()-start,flush=True)
```

</details>

集計は stage/input/n/version ごとにJSONのoutputから `ns/op`, `B/op`, `allocs/op` を抽出し、CLIはduration_s×1000、各指標のmedian/min/maxを取る。予備pilotとcorrectness行を除き、正常反復は全件含める。

## 限界と判断

合成ASCIIの単一段落と2ノートVault、128–2048行、ローカルmacOSのみ。実Vault、Unicode、より大きい規模、冷たいキャッシュ、Ubuntu、peak RSSには外挿しない。旧→現行にはA4-03以外の累積変更も含むため、CLI全体の差すべてを一つのhelperへ帰属させない。A4-03で重複探索の除去がすでに成立したことをsourceとparser増加率が支持するため、追加最適化は採用せず、対象の性能証拠を揃えてA4-06を完了とする。
