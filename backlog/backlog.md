# Backlog

## v0.16.2

2026-07-08 の maintenance-audit deep（7 観点 + 8 観点の 2 run）の結果から。機能追加なし。エラー報告の改善とテスト安全網（挙動が良くなる系）。タスクは上から順に実施する（依存: 骨格共通化 → ロールバック統一 → restoreSetBackup テスト）。

- [ ] `move_template.go:210` の `err == sql.ErrNoRows` を `errors.Is` に統一: 他 17 箇所は `errors.Is` で、この 1 箇所だけ `==` 比較。将来ラップ層が入ると「source must be a registered note」分岐を素通りして生 error が漏れる（maintenance-audit deep 2026-07-08 第 2 run・HC3 由来）
- [ ] link_type の Go / SQL 一致検証テスト: `isPathLinkType`（Go の switch）と `pathLinkTypeSQLList` / `traversalLinkTypeSQLList`（SQL リテラル列挙）が別管理で、新 link_type 追加時に片方を忘れると rewrite 系だけがサイレントに新型を無視する。全 link_type で両者が一致することを表明するテストを追加
- [ ] meta-check / meta-validate の text 出力テスト追加: `printMetaCheckText` / `printMetaValidateText` が coverage 0%。text はデフォルト format で、無指定実行時に必ず通る経路が未検証。format_test.go の既存方式（結果構造体 → bytes.Buffer）で低コストに追加できる
- [ ] `removeOrPhantomize` の「既存 phantom へのエッジ付け替え」分岐（db.go:385-393）のテスト追加: coverage 64.3% の未カバーがこの分岐と疑われる。delete / update 両方から呼ばれる状態遷移（maintenance-audit deep 2026-07-08 第 2 run・HC4 由来）
- [ ] resolveMaps の mutate 直接操作をメソッドへ集約: `update.go:258-265` が `resolveMaps.addNote` の本体を、`update.go:300-311` が `rebuildBasenameToPath` をほぼ逐語コピーしてフィールドを直接変更している（`add.go:101` は正しくメソッド呼び出しで、経路間で流儀が割れている）。メソッド呼び出しに置換し、他ファイルにも内部マップを直接変更する箇所が残っていれば同様にメソッドへ寄せる。読み取りの直接参照は不変条件を壊さず package 内イディオムの範囲なので間接化しない（全面アクセサ化は design-decision で不採用判断 2026-07-08）（maintenance-audit deep 2026-07-08 第 2 run・RS2 由来）
- [ ] resolveFrontmatterPathDry の手動同期複製の解消: `frontmatter_path_guard.go:86-121` が `resolveLinkWithBackend`（link_resolver.go:22-67）の分岐 4 段を「keep the two in sync」コメント頼みで再現している。phantom を作らない dry-run backend を `linkResolverBackend` の第 3 実装として相乗りさせ、分岐の二重管理を消す（maintenance-audit deep 2026-07-08 第 2 run・RS2 由来）
- [ ] scan-and-rewrite 骨組みの共通化: `Convert` / `Simplify` / `Repair` / `DisambiguateScan` が「vault 走査 → exclude 適用 → リンク判定 → rewriteEntry 蓄積 → DryRun 判定 → grouping → applyFileRewrites」の同一骨格を各自で書き下している。可変部はリンク判定コールバック 1 つ。`groups := make(map[string][]rewriteEntry)` の grouping は計 7 箇所で字面重複。exclude 設定の扱いの差が「意図的か揃え忘れか」判別できない状態の解消。実施判断（2026-07-08、BSSN 適用で存続確定）: 横断変更の需要は将来仮説ではなく同リリース内に実在する（次のロールバック失敗報告の統一がこの 4 コマンドの apply 経路を全部触る）ため、ロールバック統一より先に実施すると統一が 1 箇所で済む。パラメータ化は現 4 コマンドが必要とする差分（判定コールバック・exclude 設定・fileSet 検証）に限定し、将来コマンド用のフック点は作らない。微差は共通化前に 1 個ずつ意図的か揃え忘れかを確定する
- [ ] ロールバック失敗報告の統一: 同じ書き換え基盤（`applyFileRewritesWithRollbackFailures`）を使うのに、復元失敗（ファイルが原本でも新版でもない状態で残る）を move / MoveDir だけが `wrapRollbackFailures` で報告し、convert / repair / simplify / add / disambiguate は `applyFileRewrites` / `restoreBackups` の `_` 破棄で握り潰している。move 方式に統一し、破棄ラッパ 2 種を廃止する。`set` の `restoreSetBackup`（復元失敗を呼び出し側に伝えていない）も同じ判断対象に含める
- [ ] `restoreSetBackup` のテスト追加: coverage 0%。`set` は vault ファイルを直接書き換える破壊的操作なのに、Update 失敗時に内容・パーミッション・mtime が復元される経路が未検証。既存の失敗系テストは全て書き込み前 return で、ロールバックに到達しない（ロールバック統一の後に、統一後の報告挙動でテストする）

## v0.16.3

同じ maintenance-audit deep の結果から。挙動不変の内部構造整理。タスクは上から順に実施する（依存: edge・mtime 統一 → move_helpers 分割 → raw_link 共通化。fallback 集約検討は backend 構造が出揃う最後）。

- [ ] where.go を where_parse.go / where_sql.go にファイル分割: 構文解析（`ParseWhere` 系）と SQL 生成（`MetaFilterSQL` 以降）は変更トリガーが別（CLI 仕様変更 vs SQL 組み立て変更）で、`WhereCond`/`WhereClause` 型を境に前後が明確に分離済み。pathfilter.go / exclude_filter.go と同じ「条件→SQL」サブグループとして揃える。single/coalesce 4 関数の統合は登録見送りのままで、これはファイル配置のみ（maintenance-audit deep 2026-07-08 第 2 run・RS1/HC4 由来）
- [ ] usage.go の help const を各コマンドファイルへ移動: フラグ名が `fs.String` と help prose の 2 箇所に文字列で二重定義され、コンパイラもテストも不一致を検出しない。usage.go は直近 60 commit で 9 回変更のホットスポット。help const を対応するコマンドファイル（buildHelp → build.go 等）へ移せば 1 ファイル内で実装と help を同時更新でき、524 行 critical も解消（maintenance-audit deep 2026-07-08 第 2 run・RS1/HC2 由来）
- [ ] add / disambiguate の edge・mtime 更新手書きを `updateExternalEdgesAndMtimes` に統一: canonical helper（move_helpers.go:835）と同一ロジックを `add.go:377-398` / `disambiguate.go:218-240` が手書き再実装。両者とも tx dbExecer を持ちシグネチャに合致するため置換だけで済む。move_helpers 分割の前にやると分割対象が 1 箇所に集約済みになる（maintenance-audit deep 2026-07-08 第 2 run・HC2 由来）
- [ ] `move_helpers.go`（952 行）の責務分割: 「helpers」の名の下に DB ロード検証 / リンク書き換え計算 / disk 適用・ロールバック / phantom 昇格が同居（7 エージェント中 5 が最重要指摘で一致）。`move_dir.go` の Phase 構造と 1:1 対応する 3 ファイル（load / rewrite / apply）に分割。あわせて `rewriteOutgoingRelativeLink` の wikilink / markdown ほぼ重複な 2 分岐（654-770 行）を共通手順＋link type 差分パラメータ化に集約。登録見送りに残した resolveMaps 系のフィールド名不一致もこの時に同時に直すと安い
- [ ] raw_link の alias/subpath 分割パースの部分共通化: `[[` 剥がし + `|`/`#` 分割が rewrite.go（2 箇所）・move_helpers.go・convert.go の計 5 関数で個別実装され、parse.go の canonical `splitAlias`/`extractSubpath` を使っていない。分割部分だけ共通化し、再構築ロジック（`.md` 付け外し・相対再計算）は各コマンドに残す。共通化で呼び出し側の意味がぼやける箇所は無理に寄せず見送ってよい（実装時に design-decision を適用して個別判断する）（maintenance-audit deep 2026-07-08 第 2 run・HC2 由来）
- [ ] リンク解決 fallback 優先順の二重実装の集約検討: build.go の `mapLinkResolver`（in-memory）と resolve.go の `dbLinkResolver`（DB クエリ）が basename の unique→root-priority→asset→phantom という優先順（ルート優先ルール = コア不変条件、ADR 0004）を各自ハードコードしており、片方だけ直すと build 時解決と resolve コマンドが乖離する。共有の段リスト化は design-decision 対象。段の共有が無理な抽象になるなら「両 backend が同じ答えを返すことを表明する突き合わせテスト」で防ぐ出口もある（maintenance-audit deep 2026-07-08 第 2 run・RS1 由来）

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] パース層の package 化の再評価
  - **trigger**: 「internal/core の外部（別バイナリ・別リポジトリ）から parse だけを再利用したい要求が出た時」または「parse の出力型 `linkOccur` への誤った参照が実際にバグを生んだ時」。副次ウォッチ: core が 60 ファイル級に到達、bridge 層（link_resolver / link_ambiguity）の増殖、`linkOccur` のフィールドが 12+ に増加
  - **経緯**: internal/core は 30 ファイルで `docs/rules/architecture.md` の分割検討条件（20 ファイル超 + 責務グループ明確）に到達。2026-06-10 の audit + module-boundary 判断で「mutation クラスタは resolveMaps / rewriteEntry / dbExecer の密共有で export 面が大きく未成熟、パース層のみが候補だが link_keys で入出力が変わる直前」として package 分割は見送り、file 境界整理（v0.9.0）のみ実施
  - **2026-07-08 再評価（maintenance-audit deep）**: 旧 trigger（link_keys 着地後の parse 入出力安定）は達成済みだが、結論は「できるが割に合わない」で見送り継続。parse クラスタ自体は依存クリーンだが、`linkOccur`（unexported）が 14 ファイルに漏れており、package 化は 8 フィールド全 export ＋ `LinkType` の循環回避のための型 package（`linkmodel`）新設を強制する。誤参照の実害も grep で観測されず、file 境界 + DI シグネチャ（`parseLinksWithLinkKeys([]string)`）で目的は達成済み。将来分割時の DAG 設計は audit 記録参照（core → parse → linkmodel）

- [ ] `delete --rm` のディスク削除と DB トランザクションの順序見直し（ディスク削除が tx より先行し、commit 失敗時にディスクと DB が恒久不整合・復旧手段なし。稀な事象なので急がないが、削除前 tmp 退避か tx 先行への作り替えを検討）

## 登録見送り（maintenance audit。再判断トリガー付きのものだけ残す）

- where.go の single / coalesce SQL 生成 4 関数の統合（2026-07-08）: 意見が割れた（「coalesce 版に吸収すれば 4→2 関数」vs「coalesce 版は優先順位ロジックが本質的に追加されており分離が妥当」）。新演算子追加が実際に来た時に再判断。`comparisonOpSQL` 等の SQL 断片生成の一本化だけなら低リスク
- resolveMaps / noteResolveMaps / assetResolveMaps のフィールド名不一致（2026-07-08。`pathSetLower` が型により lower→actual マップと存在集合の別物を指す）: 混乱源だが実害未発生。v0.16.3 の move_helpers 分割に触れる時に同時にやると安い
- sentinel error 14 個が本番コードで未使用（2026-07-08。テスト専用 fixture 化）: exit code 分岐等の要求が来た時に初めて活きる。error 文言変更時の二重管理だけ注意
- meta_check.go / meta_validate.go のリネーム（2026-07-03）: 不採用。mdhop の慣例「CLI コマンド名 = ソースファイル名」を壊し、`mdhop meta-check` の実装を探す際の新しい乖離を作る。責務の判別問題は各ファイル先頭の doc comment で解決する（v0.13.0 の meta-validate プロファイル実装時に付与）
