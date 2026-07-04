# Backlog

## v0.15.0

リンク解決コアと内部構造の整理。2026-07-03 maintenance audit の findings と確定済み設計方針から構成。順序は上から。

- [x] resolveMaps の登録 API を一本化する（方針確定 2026-07-03）
  - **症状**: `pathToID` は「DB insert 後に caller がセットする」順序契約で、add / build / update / move / move_helpers の 5 ファイル 13 箇所（asset 側 4 箇所）が暗黙手順に依存（`resolve_maps.go:31-33,47-48` のコメントが手順依存を自認）。順序を誤ると phantom promotion が誤った ID を掴みリンク解決が静かに壊れる
  - **対処案**: DB 依存は持ち込まず、`registerNote(path, id)` / `registerAsset(path, id)` 相当の登録 API を追加して caller の pathToID 直接書き込みを全置換する。「addNote したのに ID セット忘れ」を構造的に排除。build 後に全 node の path→ID 対応を突き合わせる整合性テストも追加する

- [ ] util.go の責務混在を解消する（resolveMaps API 確定後に実施）
  - **症状**: 純粋な文字列/パス正規化、resolveMaps 依存のリンク解決判定（`util.go:123-186`）、ファイルシステム副作用（`util.go:191-253` の `CleanupEmptyDirs` 等）の 3 系統が同居。resolveMaps の内部変更が util.go に波及することがファイル名から予測できない
  - **対処案**: `link_ambiguity.go` / `fs_cleanup.go` 等へ切り出す

- [ ] resolveLink / resolveLinkFromDB を basenameResolver 抽象で統合する（方針確定 2026-07-03）
  - **症状**: `build.go:214-283`（インメモリ resolveMaps 版）と `resolve.go:82-140`（DB クエリ版）が "Mirrors" コメント付きで判定分岐の順序・条件式まで複製（分岐数・順序は完全一致）。解決ルール変更時に resolve.go 側が漏れると `mdhop resolve` の答えとインデックス構築結果が矛盾する
  - **対処案**: 判定分岐を 1 実装に統合し、basename 解決ステップだけ interface に切り出す（build = map 実装 / resolve = DB クエリ実装）。曖昧時の仕様差（build は phantom フォールスルー / resolve は `ErrAmbiguousLink`）は resolver 実装差として吸収し、外部挙動は変えない

- [ ] query/diagnose/stats のフィールド名を core 定数化する（search 方式に統一）
  - **症状**: search のみ Go 定数（`search.go:42-44` → `format_search.go:14-16`）でコンパイラチェックが効き、query（`query.go:119-173` / `format_query.go:10-18`）・diagnose・stats は生文字列を core 分岐 / cmd validation map / format 出力キーの 2〜3 箇所で手動同期している。変更漏れは「無効フィールドが黙って無視される」形で実行時にしか出ない
  - **対処案**: 各コマンドのフィールド名を core 定数として定義し cmd 側から参照。`.claude/rules/conventions.md` に「フィールド名は core 定数を参照する」規約を明記

- [ ] ExcludeFilter を config.go から exclude_filter.go に分離する
  - **症状**: `config.go:93-247` の 154 行がクエリ時フィルタ実行ロジック。`PathExcludeSQL` は既に `pathfilter.go` にあり、同一型のメソッドが 2 ファイルに分散している
  - **対処案**: 型定義と全メソッドを 1 ファイルに集約する機械的移動。config.go は mdhop.yaml のロードとバリデーションに絞る

- [ ] テスト追加: db.go の LastInsertId フォールバック分岐
  - **症状**: `upsertNode` 54.5%（modernc.org/sqlite の LastInsertId フォールバック分岐 `db.go:172-179` が未検証）。move_helpers.go 分のテストは v0.14.0 の挙動一致テストに吸収済み
  - **対処案**: 同名削除→再削除の順序依存シナリオ等でフォールバック分岐を通すテストを追加する

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] パース層の package 化の再評価
  - **trigger**: v0.10.0 の `meta.link_keys` 着地後、parse 層の入出力（MetaConfig 注入）が安定した時点
  - **経緯**: internal/core は 30 ファイルで `docs/rules/architecture.md` の分割検討条件（20 ファイル超 + 責務グループ明確）に到達。2026-06-10 の audit + module-boundary 判断で「mutation クラスタは resolveMaps / rewriteEntry / dbExecer の密共有で export 面が大きく未成熟、パース層のみが候補だが link_keys で入出力が変わる直前」として package 分割は見送り、file 境界整理（v0.9.0）のみ実施

## 登録見送り（2026-07-03 maintenance audit）

- package 分割・format_meta_* 重複統合・同型スケルトン共通化: ignore 判断（便益がコストを下回る）
- meta_check.go / meta_validate.go のリネーム: 不採用。mdhop の慣例「CLI コマンド名 = ソースファイル名」を壊し、`mdhop meta-check` の実装を探す際の新しい乖離を作る。責務の判別問題は各ファイル先頭の doc comment で解決する（v0.13.0 の meta-validate プロファイル実装時に付与）
