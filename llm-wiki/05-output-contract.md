---
regen: compiled
sources:
  - cmd/mdhop/format.go
  - cmd/mdhop/format_query.go
  - cmd/mdhop/format_resolve.go
  - cmd/mdhop/format_stats.go
  - cmd/mdhop/format_diagnose.go
  - cmd/mdhop/format_search.go
  - cmd/mdhop/format_move.go
  - cmd/mdhop/format_movedir.go
  - cmd/mdhop/format_repair.go
  - cmd/mdhop/format_simplify.go
  - cmd/mdhop/format_disambiguate.go
  - cmd/mdhop/format_convert.go
  - cmd/mdhop/format_add.go
  - cmd/mdhop/format_delete.go
  - cmd/mdhop/format_update.go
  - cmd/mdhop/format_graph.go
  - cmd/mdhop/format_reachable.go
  - cmd/mdhop/format_meta_check.go
  - cmd/mdhop/format_meta_validate.go
  - cmd/mdhop/main.go
---

# stdout 出力契約ガイド

CLI の stdout / stderr 分離方針と、JSON 出力の構造的なポイントをまとめる。フィールド仕様の正本は `docs/specs/overview.md`。個々のフィールドの実装上の罠は `cmd/mdhop/format*.go` のコメントを参照。

---

## stdout / stderr の分離方針

| ストリーム | 何を出力するか |
|---|---|
| stdout | コマンドの主結果（JSON または text）。agent が parse するもの |
| stderr | warnings、hints、エラーメッセージ、usage |

**stderr の出力パターン（`cmd/mdhop/format.go` および各コマンドファイル）:**

- `printWarnings(warnings []string)` — `warning: <msg>` 形式で各行を stderr に書く。build/add/update が呼び出す
- `formatCommandError(command, err)` — `error: <command>: <message>` 形式でトップレベルエラーを整形する（`main.go:82`）
- `fmt.Fprintln(os.Stderr, "hint: ...")` — index が存在しない場合のヒント（repair/simplify/convert）
- `fmt.Fprint(os.Stderr, "Usage: ...")` — main.go:93 でサブコマンド不明時

**原則:** stdout に warning や hint を混ぜない。agent が stdout をそのまま parse できることが不変条件。

---

## JSON エンコーダ共通仕様

`format.go` の `encodeJSON(w, v)` を全コマンドで共有する。

- `json.NewEncoder` + `SetIndent("", "  ")` → インデント付き JSON
- 末尾に改行が自動付与される（`json.Encoder.Encode` の仕様）

---

## JSON フィールドの出し方: 設計パターン

### パターン A — struct + omitempty（リクエスト条件つきフィールド）

**query コマンド** (`format_query.go:21-30`):

```
queryJSONOutput.Backlinks []jsonNodeInfo  `json:"backlinks,omitempty"`
queryJSONOutput.TwoHop    []jsonTwoHop    `json:"twohop,omitempty"`
queryJSONOutput.Meta      map[string][]string `json:"meta,omitempty"`
```

リクエストされなかったフィールドは nil のまま → `omitempty` で JSON から落ちる。

**罠:** nil スライスと空スライス `[]` は `omitempty` では同じ扱い（両方省略）。しかし **mutation 系コマンド（add/delete/update/move/repair/simplify/convert/disambiguate）は常に全フィールドを返す** ため、nil を `[]` に変換してから encode する（下記参照）。

### パターン B — nil → 空配列への正規化（mutation 系）

mutation 系の JSON formatter はすべて、encode 前に `emptyIfNil`（`format.go:15-20`）で nil スライスを空スライスへ正規化する（代表例 `format_add.go:21-27`）。これにより、操作がなかった場合も `"added": []` のように空配列が出力され、フィールドが消えない。agent がフィールド存在を前提に parse できる安定 IF。

### パターン C — map[string]any / map[string]int（フィールド選択型）

**resolve / stats / diagnose / reachable** はリクエストされたフィールドのみ map に入れて encode する。

```
resolve:   buildResolveMap() → map[string]any  (format_resolve.go:43)
stats:     printStatsJSON()  → map[string]int  (format_stats.go:19)
diagnose:  printDiagnoseJSON() → map[string]any (format_diagnose.go:37)
reachable: printReachableJSON() → map[string]any (format_reachable.go:17)
```

**罠:** `map[string]any` はリクエストされたフィールドが空でも常に key が出力される。一方 struct + `omitempty` だと空配列はフィールドごと消える。意図の違いに注意。

diagnose の phantoms は例外的に、map へ `emptyIfNil` で正規化した空スライスを入れる:
→ `format_diagnose.go:54-56`

---

## `jsonNodeInfo` — note/asset/phantom/tag の共通型

`format.go:105-119` で定義（type / name / path / exists の 4 フィールド。定義本体は正本を読む）。

- `Path` と `Exists` は `type == note || type == asset` のときのみセットされる
- `Exists` は `*bool` + `omitempty` — **bool を直接使うと false が JSON から落ちる**
- query の entry は常に pointer (`*jsonNodeInfo`) で出力される（nil 不可）

---

## mutation 系の共通型（format.go）

| 型 | JSON フィールド | 用途 |
|---|---|---|
| `rewrittenJSON` | `file`, `old`, `new` | リンク書き換え1件 |
| `skippedJSON` | `file`, `raw_link`, `basename`, `candidates` | 曖昧で書き換えスキップ |
| `rewriteResultJSONOutput` | `rewritten`, `skipped` | repair/simplify 共通出力 |

`printRewriteResultJSON()` は repair と simplify の両方が使う。rewritten/skipped ともに nil → `[]` 変換済み。

---

## コマンド別 JSON 出力形状の早見表

| コマンド | トップレベル型 | nil→[] 変換 | 備考 |
|---|---|---|---|
| resolve | `map[string]any` | 不要 | フィールド選択型 |
| query | struct `queryJSONOutput` | 不要（omitempty） | entry は `*jsonNodeInfo` |
| search | struct `searchJSONOutput` | 不要 | `total`, `items[]` 常出力 |
| stats | `map[string]int` | 不要 | フィールド選択型 |
| diagnose | `map[string]any` | phantoms のみ | anchors は opt-in |
| reachable | `map[string]any` | reachable/unreachable | from は常出力 |
| graph | `map[string]any` | 不要 | nodes/edges 常出力 |
| move | struct `moveJSONOutput` | rewritten | from/to 常出力 |
| movedir | struct `moveDirJSONOutput` | moved, rewritten | |
| add | struct `addJSONOutput` | added, promoted, rewritten | |
| delete | struct `deleteJSONOutput` | deleted, phantomed | |
| update | struct `updateJSONOutput` | updated, deleted, phantomed | |
| repair | `rewriteResultJSONOutput` | rewritten, skipped | printRewriteResultJSON |
| simplify | `rewriteResultJSONOutput` | rewritten, skipped | printRewriteResultJSON |
| disambiguate | struct `disambiguateJSONOutput` | rewritten | |
| convert | struct `convertJSONOutput` | rewritten | |
| meta-check | struct `metaCheckJSONOutput` | issues（nil なら `[]` を生成） | |
| meta-validate | struct `metaValidateJSONOutput` | violations（nil なら `[]` を生成） | |

---

## フィールドバリデーションのタイミング

`validateFormat()` (`format.go:51`) / `validateFields()` (`format.go:60`) は DB オープン **前** に実行する。理由: index が存在しない状態でも unknown field エラーを即返せるようにするため。

---

## search の特殊フィールド: *int + omitempty

`searchJSONItem.Lines`, `OutgoingCount`, `IncomingCount` は `*int` + `omitempty`（`format_search.go:51-53`）。

- リクエストされた場合のみポインタをセット → フィールドが出力される
- リクエストされなかった場合は nil → `omitempty` で省略
- `bool` と同じ理由で直接 `int` を使うと 0 と「未リクエスト」が区別できない

---

## graph の特殊フォーマット: dot

graph コマンドのみ `--format dot` が有効。`format_graph.go:36-46` で Graphviz digraph 形式を stdout に出力。`dotQuote()` は `strconv.Quote` を使い、ラベルをエスケープする。

---

## text フォーマットの慣習

- YAML 風の `key: value` 形式
- リスト項目は `- ` プレフィックス
- `writeNodeInfoText(w, n, firstIndent, restIndent)` — リスト項目の1行目は `- type: note`、続行は `  name: ...` とインデントを分ける（`format_query.go:167-174`）
- `nodeInfoOneLine(n)` — twohop の via/targets 向けのコンパクト1行形式（`format_query.go:178-185`）
- text フォーマットで「リクエストされなかったフィールド」と「リクエストされたが空だったフィールド」はどちらも出力されない（mutation 系は nil でも print するが空なら `printStringListText` がスキップ）
