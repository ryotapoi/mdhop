# Backlog

## v0.16.0

2026-07-04/05 に確定した機能仕様と、先行して着地した実装（v0.13.0 / v0.14.0）との突き合わせで見つかった差分の修正。順序は上から。

- [ ] meta-validate: `--require` 明示時は meta.profiles を置換する（確定仕様 2026-07-04）
  - **現状**: `--require` と `meta.profiles` は合算して検証され dedup される（`internal/core/meta_validate.go:93-102`。`docs/specs/overview.md` の meta-validate 節も合算で記述）
  - **確定仕様**: `--require` を明示指定した実行では profiles の require は無視して置換する（マージしない）。meta.types の型 / enum 検査は `--require` の有無に関わらず従来通り常に実行
  - **対処案**: `opts.Require` 非空なら `validateRequiredProfiles` を呼ばない。overview.md・`--help`・テスト（合算を検証している `TestMetaValidate_DeduplicatesCLIAndProfileRequire` は置換検証に差し替え）を同一コミットで更新

- [ ] set: `--date <expr>` オプションを追加する（確定仕様 2026-07-04）
  - **現状**: `--value` のみのリテラル書き込みで、usage に "relative dates are not expanded" と明記（`cmd/mdhop/usage.go:87`）
  - **確定仕様**: `--date <expr>` は search の相対日付構文（`today` / `today-90d` / `today+1y` 等）を YYYY-MM-DD に解決してから書き込む。`--value` と相互排他（両方指定・両方省略はエラー）
  - **対処案**: `expandRelativeDate`（`internal/core/where.go:539`）を set から再利用できる形にして実装。usage の "not expanded" 記述と overview.md も更新

- [ ] set: frontmatter なしファイルでは frontmatter を自動作成する（確定仕様 2026-07-04）
  - **現状**: `frontmatter not found` エラー（`internal/core/set.go:121-124`、`TestSetNoFrontmatterError`）
  - **確定仕様**: エラーにせず、ファイル先頭に `---` ブロックを新規作成してキーを書き込む
  - **対処案**: 先頭に `---` / `key: value` / `---` を挿入する分岐を追加。既存のエラー検証テストを自動作成検証に差し替え。overview.md も同期

- [ ] search: `--where` 式内の ` || ` 区切りを追加する（確定仕様 2026-07-04）
  - **現状**: 式は ` && ` でのみ分割され（`internal/core/where.go:72`）、` || ` は区切りとして解釈されない。混在検知もなし
  - **確定仕様**: 1 つの式内で ` || ` 区切りの OR が書ける。1 つの式内での ` && ` と ` || ` の混在は（括弧がなく優先順位を決められないため）明示エラー。複数 `--where` フラグ間は従来通り AND。括弧・任意ネストは非対応のまま（必要になったら Later で再検討）
  - **対処案**: `ParseWhere` に OR グループを追加し、SQL は UNION で結合（既存の同一キー OR と同系の生成）。`coalesce()`（実装済み）と併用可能にする。overview.md の `--where` 節も更新

- [ ] move: `--to-template` の確定仕様との差分を埋める（確定仕様 2026-07-04）
  - **現状と差分**（`internal/core/move_template.go`、`cmd/mdhop/move.go:33-35`）:
    - `--dry-run` が存在しない（確定仕様: 移動計画のみ出力するモードあり）
    - 日付部分抽出は `{key:year}` のみ（確定仕様: `{key:month}` / `{key:day}` も。正規化済み sort_value ベース）
    - `{key:year|2099}` のような部分抽出とフォールバックの併用が不可（確定仕様: 併用可。フォールバックはキー欠損時に使う。キーは存在するが値が日付でない場合はエラーのままでよい）
    - dir 指定 + `--to-template` を CLI が明示拒否（確定仕様: dir 一括モードあり。全ファイルを事前検証し、1 件でも失敗があれば全体を中止する all-or-nothing。部分実行なし）
    - 展開値の途中に `/` が含まれるケースの扱いが未定義・テストなし（確定仕様: エラー。テンプレート側の `/` だけがディレクトリ区切り）
  - **対処案**: 上から順に実装。dir 一括は MoveDir のバッチ機構に from→to リストを渡す形で載せる。`/` 混入エラーはテスト付きで明確化。overview.md の `--to-template` 節・`--help` を同期

## v0.15.0

リンク解決コアと内部構造の整理。2026-07-03 maintenance audit の findings と確定済み設計方針から構成。順序は上から。

- [x] resolveMaps の登録 API を一本化する（方針確定 2026-07-03）
  - **症状**: `pathToID` は「DB insert 後に caller がセットする」順序契約で、add / build / update / move / move_helpers の 5 ファイル 13 箇所（asset 側 4 箇所）が暗黙手順に依存（`resolve_maps.go:31-33,47-48` のコメントが手順依存を自認）。順序を誤ると phantom promotion が誤った ID を掴みリンク解決が静かに壊れる
  - **対処案**: DB 依存は持ち込まず、`registerNote(path, id)` / `registerAsset(path, id)` 相当の登録 API を追加して caller の pathToID 直接書き込みを全置換する。「addNote したのに ID セット忘れ」を構造的に排除。build 後に全 node の path→ID 対応を突き合わせる整合性テストも追加する

- [x] util.go の責務混在を解消する（resolveMaps API 確定後に実施）
  - **症状**: 純粋な文字列/パス正規化、resolveMaps 依存のリンク解決判定（`util.go:123-186`）、ファイルシステム副作用（`util.go:191-253` の `CleanupEmptyDirs` 等）の 3 系統が同居。resolveMaps の内部変更が util.go に波及することがファイル名から予測できない
  - **対処案**: `link_ambiguity.go` / `fs_cleanup.go` 等へ切り出す

- [x] resolveLink / resolveLinkFromDB を basenameResolver 抽象で統合する（方針確定 2026-07-03）
  - **症状**: `build.go:214-283`（インメモリ resolveMaps 版）と `resolve.go:82-140`（DB クエリ版）が "Mirrors" コメント付きで判定分岐の順序・条件式まで複製（分岐数・順序は完全一致）。解決ルール変更時に resolve.go 側が漏れると `mdhop resolve` の答えとインデックス構築結果が矛盾する
  - **対処案**: 判定分岐を 1 実装に統合し、basename 解決ステップだけ interface に切り出す（build = map 実装 / resolve = DB クエリ実装）。曖昧時の仕様差（build は phantom フォールスルー / resolve は `ErrAmbiguousLink`）は resolver 実装差として吸収し、外部挙動は変えない

- [x] query/diagnose/stats のフィールド名を core 定数化する（search 方式に統一）
  - **症状**: search のみ Go 定数（`search.go:42-44` → `format_search.go:14-16`）でコンパイラチェックが効き、query（`query.go:119-173` / `format_query.go:10-18`）・diagnose・stats は生文字列を core 分岐 / cmd validation map / format 出力キーの 2〜3 箇所で手動同期している。変更漏れは「無効フィールドが黙って無視される」形で実行時にしか出ない
  - **対処案**: 各コマンドのフィールド名を core 定数として定義し cmd 側から参照。`.claude/rules/conventions.md` に「フィールド名は core 定数を参照する」規約を明記

- [x] ExcludeFilter を config.go から exclude_filter.go に分離する
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
