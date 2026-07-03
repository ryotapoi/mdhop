# Backlog

## v0.13.0

2026-07-03 Knowledge 側のスキル改修（/dig /updating /maintenance 等）との検討で決めた機能追加。順序は上から。

- [ ] `mdhop set` — frontmatter の単一キーを安全に書き換えるコマンド
  - **背景**: 現状 frontmatter を書き換えるコマンドがなく（convert / repair / disambiguate / simplify は本文リンクのみ）、LLM が YAML を手編集していて崩し事故のリスクがある
  - **対処案**: 例 `mdhop set --file <path> --key reviewed --value 2026-07-03`。YAML 全体を再シリアライズせず対象キーの行だけ書き換える（キー順序・コメント・引用形式を保存し diff を汚さない）。書き換え後にインデックスも同時更新する。`today` のような相対値サポートの有無は設計時に判断

- [ ] meta-validate のパスパターン別 require プロファイル
  - **背景**: スキル側が散文ルール（rules/frontmatter.md）を毎回読んで予防する方式から「書く → meta-validate → エラーだけ直す」方式に移行するため、必須キー定義を設定ファイルに寄せたい
  - **対処案**: mdhop.yaml に「このパスパターンではこのキーが必須」（例: `03-Notes/media/**` は isbn 必須、全体では type / status / created / updated 必須）を書けるようにし、呼び出しごとの `--require` 指定なしで検証できるようにする。単一ファイルを対象にする利便（`--file` 相当）も検討

- [ ] `mdhop move --to-template` — frontmatter 値から移動先パスを展開するテンプレート
  - **背景**: アーカイブ先パスの組み立て（client 取得 → 年計算 → パス連結）を毎回 LLM がやっており、frontmatter 駆動の一括移動を1コマンドにしたい
  - **対処案**: 例 `mdhop move --from <path> --to-template "99-Archive/02-Projects/{client|others}/{updated:year}/{basename}"`。フィールド参照・fallback 記法（`{client|others}`）・日付の部分抽出（`{updated:year}`）のテンプレート構文の仕様整理が先（要仕様設計）

- [ ] search の強化 3 点
  - **背景**: 「reviewed の有無で 2 回検索してマージ」「全件 JSON を取ってから 10 件選ぶ」「件数しか使わないのに一覧を取る」という使い方が実際にあり、出力の無駄が大きい
  - **対処案**: (a) `--sample N` — 候補から CLI 側で無作為に N 件抽出して返す（候補全件を出力せずに済む）。(b) `--count` — 件数のみ返す。(c) `--where` の式強化 — `coalesce(reviewed, updated) <= today-1y` のような coalesce / OR 条件を 1 クエリで書けるようにする

- [ ] ヘルプ充実と examples スキルの薄型化
  - **背景**: LLM が `--help` だけで使い方を把握できる水準にしたい。あわせてスキルと実装のドリフトを防止する
  - **対処案**: 各サブコマンドの `--help` に「フラグの意味・出力列/JSON フィールドの定義・実例2〜3個」を載せる。トップレベルの `--help` は現在の短さを維持する。`examples/skills/mdhop/SKILL.md` と references/ を「存在の告知・いつ使うか・--help への誘導」中心に薄くし、CLI 構文の重複記述を減らす

## v0.12.1

- [x] Linux CI で Unicode 正規化系テストが落ちる不具合を修正する
  - **症状**: GitHub Actions の `ubuntu-latest` で `go test ./...` が失敗し、`TestAddNormalizesUnicodePathToExistingPhantom` / `TestAddRejectsUnicodeEquivalentExistingIndexPath` / `TestBuildNormalizesUnicodePathsToNFC` / `TestMetaCheckResolvesNFCValueToNFDPath` / `TestResolveNFCLinkAgainstNFDIndexName` が `Café.md` を見つけられない
  - **再現条件**: Linux filesystem 上で NFC / NFD の異なるファイル名を扱う path 正規化テストを実行する場合。macOS ではローカル `go test ./...` が通るため、OS ごとの filesystem 正規化差で CI のみ失敗する
  - **対処案**: テストデータ作成・path lookup・`NormalizePath` 適用位置を Linux 上でも成り立つ形に見直し、Ubuntu CI と macOS の両方で `go test ./...` を通す

- [x] `rewriteOutgoingRelativeLink` が `filepath.Rel` の戻り値を `filepath.Clean` せず壊れた相対リンクを生成しうる
  - **症状**: `move_helpers.go:642` 付近で `rel := filepath.Rel(filepath.Dir(to), resolvedTarget)` の結果を `filepath.ToSlash(rel)` するだけで `filepath.Clean` していない。`filepath.Rel("other", ".")` は `".."` ではなく `"../."` を返すため、移動後の相対リンクが `[[../.]]` のような末尾 `/.` 付きで書き換えられるケースがある
  - **再現条件**: from がサブディレクトリにあり vault ルート（`..` で解決される先）を指す相対リンクを持ち、その from を別ディレクトリへ move する場合（検証エージェントが Go で `filepath.Rel("other", ".") == "../."` を実測確認済み）
  - **対処案**: `rel = filepath.ToSlash(filepath.Clean(rel))` にする。`move_helpers.go` の 2 箇所（wikilink / markdown 系）両方を確認。回帰テストを追加する
  - **経緯**: 旧 knowledge.md にあった「`filepath.Rel(dir, ".")` は `"../."` を返す。`filepath.Clean` を適用すること」という教訓。コメント化するとコードの現状（Clean なし）と矛盾するため backlog 化（2026-06-17 の knowledge.md 解体時に分離）

## Maintenance Audit (2026-07-03)

deep pass（7 観点）の findings から対応するもの。package 分割・format_meta_* 重複統合・同型スケルトン共通化は ignore 判断（便益がコストを下回る）で登録しない。

### Small（1 commit 粒度・低リスク）

- [ ] moved file backup の rollback をヘルパーに集約する
  - **症状**: `move.go:364-419` に 5 箇所、`move_dir.go:136-163` に 3 箇所、`writeFilePreservePerm` + `os.Rename` 巻き戻しの同型 inline パターンが反復。collateral 側は `restoreBackups()`（`rewrite.go:150`）に集約済みで 12 箇所から再利用されており非対称
  - **対処案**: `rewriteBackup` 型を流用して `restoreBackups` と同型のヘルパーに集約。rollback 仕様変更時の更新漏れ（部分ロールバックで vault 不整合）を防ぐ
- [ ] query のデフォルト値（100/100/10）を core 定数に一元化する
  - **症状**: `cmd/mdhop/query.go:21-23` の flag デフォルトと `internal/core/query.go:104-111` のフォールバックが独立定義。片方だけ変えると CLI 経由と `core.Query` 直呼びで挙動が乖離する
  - **対処案**: core 側に定数を定義し、flag デフォルトから参照する
- [ ] ExcludeFilter を config.go から exclude_filter.go に分離する
  - **症状**: `config.go:93-247` の 154 行がクエリ時フィルタ実行ロジック。`PathExcludeSQL` は既に `pathfilter.go` にあり、同一型のメソッドが 2 ファイルに分散している
  - **対処案**: 型定義と全メソッドを 1 ファイルに集約する機械的移動。config.go は mdhop.yaml のロードとバリデーションに絞る

### Refactoring（別タスク化）

- [ ] query/diagnose/stats のフィールド名を core 定数化する（search 方式に統一）
  - **症状**: search のみ Go 定数（`search.go:42-44` → `format_search.go:14-16`）でコンパイラチェックが効き、query（`query.go:119-173` / `format_query.go:10-18`）・diagnose・stats は生文字列を core 分岐 / cmd validation map / format 出力キーの 2〜3 箇所で手動同期している。変更漏れは「無効フィールドが黙って無視される」形で実行時にしか出ない
  - **対処案**: 各コマンドのフィールド名を core 定数として定義し cmd 側から参照。`.claude/rules/conventions.md` に「フィールド名は core 定数を参照する」規約を明記
- [ ] util.go の責務混在を解消する
  - **症状**: 純粋な文字列/パス正規化、resolveMaps 依存のリンク解決判定（`util.go:123-186`）、ファイルシステム副作用（`util.go:191-253` の `CleanupEmptyDirs` 等）の 3 系統が同居。resolveMaps の内部変更が util.go に波及することがファイル名から予測できない
  - **対処案**: `link_ambiguity.go` / `fs_cleanup.go` 等へ切り出す
- [ ] meta_check.go / meta_validate.go のファイル名近接を解消する
  - **症状**: check はディレクトリ整合チェック（`meta_check.go:131-186`）、validate は型・必須キー検証（`meta_validate.go:101-179`）だが、どちらを触るべきか名前から判別できない
  - **対処案**: ソースファイルのリネームのみ（例: meta_dirmatch_check / meta_schema_validate）。CLI コマンド名は変えない
- [ ] テスト追加: db.go / move_helpers.go の中核パス
  - **症状**: `removeOrPhantomize` 64.3%（既存 phantom への統合パスが未検証。同名削除→再削除の順序依存シナリオでのみ到達）、`upsertNode` 54.5%（modernc.org/sqlite の LastInsertId フォールバック分岐 `db.go:172-179`）、`rewriteOutgoingRelativeLink` 66.7%（ターゲット側も同時に移動されるケース）
  - **対処案**: 順序依存シナリオと相互リンクの同時移動シナリオのテストを追加する

### Design Decision が先に必要

- [ ] resolveMaps の不変条件をカプセル化する
  - **症状**: `pathToID` は「DB insert 後に caller がセットする」順序契約で、add / build / update / move / move_helpers / delete の 6 ファイルが暗黙手順に依存（`resolve_maps.go:26,43` と `add.go:314-320` のコメントが手順依存を自認）。順序を誤ると phantom promotion が誤った ID を掴みリンク解決が静かに壊れる
  - **対処案**: design-decision で不変条件をメソッド内に閉じる API 再設計を決めてから実装する
- [ ] Move を MoveDir へ統合するか判断する
  - **症状**: リンク解決の root-priority 判定が `move.go` 2 箇所 + `move_helpers.go` 2 箇所（`collectIncomingRewritesForDir` / `collectCollateralRewritesForDir`）の並行実装。Phase 構造も並行実装で、rollback 関連ヒット数が 17 vs 8 と既に非対称化が進行。解決ルール変更時に片方だけ直すと Move と MoveDir で結果が食い違う
  - **対処案**: design-decision で「Move = 1 件の MoveDir」案を評価。Move 固有の自己参照リンク除外（`move.go:189`）等の分岐を要検証。`move_helpers.go`（886 行）の shared / dir 別分割と `frontmatter_path_guard.go`（move 専用検証）の帰属整理も統合判断とセットで決める
  - **経緯**: 2026-06-10 audit の「move 系分割再評価」（trigger: 次の move 機能要求）を本 audit の具体案で置き換え
- [ ] resolveLink / resolveLinkFromDB の二重実装の扱いを決める
  - **症状**: `build.go:214-283`（インメモリ resolveMaps 版）と `resolve.go:82-140`（DB クエリ版）が "Mirrors" コメント付きで判定分岐の順序・条件式まで複製。解決ルール変更時に resolve.go 側が漏れると `mdhop resolve` の答えとインデックス構築結果が矛盾する
  - **対処案**: design-decision でデータソース抽象を挟むコストと天秤にかけ、統合方式（または同期テストで担保する現状維持）を決める
- [ ] rollback 失敗の沈黙設計を見直すか判断する
  - **症状**: move 系の `_ =` 13 箇所は意図的な best-effort だが、rollback 自体が失敗しても一切通知されず、壊れても誰も気づかない設計が全経路に埋め込まれている
  - **対処案**: design-decision で「rollback 失敗時に警告を出力する」設計変更を検討（失敗系のテスト追加より筋が良い可能性）

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] パース層の package 化の再評価
  - **trigger**: v0.10.0 の `meta.link_keys` 着地後、parse 層の入出力（MetaConfig 注入）が安定した時点
  - **経緯**: internal/core は 30 ファイルで `docs/rules/architecture.md` の分割検討条件（20 ファイル超 + 責務グループ明確）に到達。2026-06-10 の audit + module-boundary 判断で「mutation クラスタは resolveMaps / rewriteEntry / dbExecer の密共有で export 面が大きく未成熟、パース層のみが候補だが link_keys で入出力が変わる直前」として package 分割は見送り、file 境界整理（v0.9.0）のみ実施
