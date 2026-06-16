---
regen: compiled
sources:
  - internal/core/build.go
  - internal/core/add.go
  - internal/core/update.go
  - internal/core/delete.go
  - internal/core/move.go
  - internal/core/move_dir.go
  - internal/core/disambiguate.go
  - internal/core/simplify.go
  - internal/core/convert.go
  - internal/core/repair.go
  - internal/core/init_meta.go
  - internal/core/init_meta_yaml.go
  - internal/core/rewrite.go
  - internal/core/db.go
  - internal/core/frontmatter_path_guard.go
  - cmd/mdhop/init_meta.go
  - docs/decisions/0003-move-auto-detects-disk-state.md
  - docs/decisions/0004-root-priority-for-ambiguous-basename.md
  - docs/decisions/0005-delete-rm-flag.md
  - docs/decisions/0008-move-collateral-rewrite.md
  - docs/decisions/0012-remove-external-rewrite-stale-check.md
---

# 書き込み系コマンドの破壊性・波及マップ

ディスク / DB を変更するコマンドの「何を書き換えるか」「破壊的か」「波及範囲」の早見表。
エッジケースの実測は `llm-wiki/knowledge.md` を参照。ルール原則は `docs/rules/information-management.md`。

---

## 1. コマンド別マトリクス

| コマンド | DB 変更 | ディスク変更 | 破壊的操作 | DB→ディスクの順序 |
|---|---|---|---|---|
| `build` | 全再構築（temp→rename） | なし | — | DB のみ |
| `add` | nodes/edges INSERT | 他ファイルのリンク書き換え（auto-disambiguate 時） | なし | ディスク先・DB 後 |
| `update` | edges 全削除→再挿入、nodes mtime 更新 | なし | ディスク不在ファイルを phantom 化 | DB のみ |
| `delete` | node 削除または phantom 変換 | `--rm` 時のみ `os.Remove` | `--rm` = ファイル削除 | ディスク先・DB 後 |
| `move` | node path/key/mtime 更新、edges 全削除→再挿入 | ファイル移動（`os.Rename`）＋ incoming/collateral/outgoing 書き換え | なし（失敗時ロールバック） | ディスク先・DB 後 |
| `move`（ディスク移動済み） | 同上 | リンク書き換えのみ（Rename スキップ） | なし | ADR 0003 参照 |
| `move-dir` | 複数 node 一括更新 | 複数 Rename ＋ リンク書き換え | なし（失敗時ロールバック） | ディスク先・DB 後 |
| `disambiguate` | edges raw_link 更新、nodes mtime 更新 | リンク書き換え（basename → full path） | なし（ DB バックアップ+ロールバック） | ディスク先・DB 後 |
| `disambiguate --scan` | なし（DB 不要） | リンク書き換えのみ | なし | ディスク のみ |
| `simplify` | なし（DB 不要） | リンク書き換え（path/relative → basename。`--dry-run` 時は行わない） | なし | ディスク のみ |
| `convert` | なし（DB 不要） | リンク書き換え（`--dry-run` 時は行わない） | なし | ディスク のみ |
| `repair` | なし（DB 不要） | リンク書き換え（`--dry-run` 時は行わない） | なし | ディスク のみ |
| `init-meta --write` | なし | `mdhop.yaml`（temp+rename で上書き） | 既存キー以外を追記 | ディスク のみ |

---

## 2. ディスク→DB の順序ルール

**ディスクを先に書き、その後 DB トランザクションを開始する**のが共通パターン。
理由: DB ロールバックはトランザクション内で自動処理できるが、ディスク変更は手動でロールバック（`restoreBackups`）が必要なため、失敗時にディスクとDB の二重巻き戻しを避けられる。

- `add.go:260–288` — ディスク rewrite → `db.Begin()` の順。`defer` で commit 失敗時に `restoreBackups` を呼ぶ
- `move.go:334–403` — 外部ファイル書き換え（4.1）→ 移動ファイル書き換え（4.2）→ `os.Rename`（4.3）→ `db.Begin()`（Phase 5）
- `move_dir.go:99–208` — 同パターンを複数ファイルに拡張
- `disambiguate.go:188–210` — `applyFileRewrites` → `db.Begin()` の順

---

## 3. 波及範囲の詳細

### 3-1. add

- **phantom 変換**: 追加するファイルの basename に一致する phantom が存在すれば `promotePhantom`（`move_helpers.go:798`）で note に昇格 → `AddResult.Promoted`
- **basename 再カウント**: `rm.addNote` で in-memory カウントを更新し、既存リンクが ambiguous になるか検査（`add.go:111–196`）
- **auto-disambiguate**: デフォルト ON（`--no-auto-disambiguate` で無効化。CLI フラグは `cmd/mdhop/add.go:17`、`AutoDisambiguate: !*noAutoDisambiguate` が `add.go:30`）。pattern A（既存ユニーク note が重複になる場合）は、既存の basename リンクをフルパスに書き換える。pattern B（phantom が ambiguous になる場合）は auto-disambiguate が効かずエラー（`add.go:185–195`）
- **ルート優先ルール**: 追加ファイルがルート直下なら basename collision でもエラーにしない → ADR 0004

### 3-2. update

- ディスク不在ファイルは `removeOrPhantomize`（`db.go:350`）で delete 判定と同じロジックへ
- 存在ファイルは outgoing edges 全削除→再挿入（`update.go:145–181`）。meta エントリも削除→再挿入
- `cleanupOrphanedNodes` で tags / phantoms / assets の孤立ノードを除去（`db.go:410`）

### 3-3. delete

- **`--rm`（`DeleteOptions.RemoveFiles`）**: `os.Remove` をディスク操作フェーズで実行（`delete.go:65–97`）。Phase 2 がディスク削除、Phase 3 が DB 更新。`--rm` 失敗後に TX に入らないのでロールバック問題なし
- **phantom 変換条件**: incoming edges（自己リンク除く）が 1 件でもあれば phantom へ変換。なければノード完全削除（`db.go:350–406`）→ ADR 0005
- **注意**: `--rm` 成功後に TX 失敗してもファイルは復元しない。`--rm` なしで再実行すれば DB は復旧可能（ADR 0005 Consequences）

### 3-4. move / move-dir

- **incoming rewrite（Phase 2）**: 移動元への path リンクをすべて書き換える。basename リンクは basename が変わった場合か、ambiguous になった場合のみ書き換える（`move.go:182–215`）
- **collateral rewrite（Phase 2.5）**: 移動先 basename と一致する他の note への basename リンクが ambiguous になる場合に、それらを full path に書き換える → ADR 0008（`move.go:220–238`）
- **outgoing rewrite（Phase 3）**: 移動したノートの outgoing basename リンクのうち、移動後に解決先が変わるものと、relative リンクを書き換える（`move.go:244–328`）
- **ルート優先ルール**: incoming/collateral の書き換えスキップ判定に `hasRootInPathSet` を使用（`move.go:200, 201` と `222, 223`）→ ADR 0004
- **ディスク移動自動検知**: from 不在・to 存在なら Rename スキップ（`move.go:84–95`）→ ADR 0003
- **外部リライト stale チェック**: v0.12 で削除済み。ミスマッチ時は silent no-op で DB のみ更新（`build` で復旧）→ ADR 0012
- **move-dir**: `move.go` の同 Phase 構造を複数ファイルに適用（`move_dir.go`）。disk-only ファイル（DB 未登録）も `os.Rename` するが DB 更新はしない（`move_dir.go:179–190`）

### 3-5. disambiguate

- DB あり版（`Disambiguate`）: basename リンクに加え phantom 指し path リンクも対象（`disambiguate.go:97–142`）
- DB なし版（`DisambiguateScan`、`disambiguate.go:275`）: DB を使わず disk scan のみ。broken path リンクは `isLinkBrokenForScan`（`disambiguate.go:419`）で判定
- どちらも書き換え対象は source ファイルのディスク上コンテンツのみ。DB の edge raw_link も更新（DB あり版のみ）

### 3-6. simplify / convert / repair

- **DB 不要**: いずれもインデックスを使わず disk scan で動作（in-memory resolve maps を都度構築）
- **simplify**: 解決可能な path/relative リンクを basename リンクに短縮する（`simplify.go:26`、書き換えは `basenameTarget` へ `rewriteRawLink`、`simplify.go:170`）。`--dry-run` 対応。ambiguous になるものは短縮しない
- **convert**: wikilink ↔ markdown の形式変換（`convert.go:25`）。`--dry-run` で実際のファイル書き換えをスキップ
- **repair**: vault 外逃げリンク（escaping）と broken path リンクをデフォルト basename 形式に書き換える（`repair.go:35`）。`--dry-run` 対応。body links のみ対象（frontmatter wikilink は除外、`repair.go:219–225`）。候補 2 件以上の broken path リンクはスキップ・`Skipped` に報告

### 3-7. init-meta --write

- **対象ファイル**: `mdhop.yaml`（vault ルート）のみ。temp ファイル → `os.Rename` のアトミック書き換え（`cmd/mdhop/init_meta.go:35–40`）
- **DB 変更なし**。既存キーは変更しない（`internal/core/init_meta.go:13` `mergeMetaConfig`）

---

## 4. frontmatter_path の特別扱い

`frontmatter_path` リンクは書き換え不可能（raw YAML 値は構文が不定）。
`add` / `move` / `move-dir` で移動後も解決先が変わらないことを事前検証する（`frontmatter_path_guard.go:validateFrontmatterPathEdges`）。
解決先が変わる場合は操作を中断してエラーを返す（`frontmatter_path_guard.go:80`）。

---

## 5. 共通ロールバック機構

- `rewriteBackup`（`rewrite.go:29`）: 書き換え前のファイル内容と permissions を保持
- `restoreBackups`（`rewrite.go:150`）: best-effort でディスク書き換えを元に戻す
- DB は `tx.Rollback()` を `defer` で保証。ディスクのロールバックは DB ロールバック `defer` の後に続けて呼ぶ（`add.go:285–288`、`move.go:405–422`）
- `build` は temp DB（`.mdhop/index.sqlite.tmp`）に全書き込み後 rename する。失敗時は temp ファイルを `defer os.Remove` で除去（`build.go:123–126`）
