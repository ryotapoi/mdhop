# Backlog

## v0.9.0 — コード・ドキュメント全体見直し

v0.10.0 / v0.11.0 の大型機能追加の前に、コード全体とドキュメントを見直す整備リリース。観点は変更容易性・管理のしやすさ・わかりやすさ（実行速度は対象外）。見直し（maintenance audit + ドキュメント整合監査 + 設計判断）は 2026-06-10 に実施済みで、以下が確定タスク。**全タスク挙動変更なし**（テスト追加・移動・重複の畳み込みのみ）。各タスクは 1 commit 粒度。

見直し結果の要約: 依存方向違反ゼロ、vet 指摘ゼロ、ドキュメントと実装の不一致なし。負債は v0.10.0 で触る継ぎ目（path filter の分散配置、parse 層の 2 責務同居、root 優先ルールの分散実装、format 出力のテスト不在）に集中。

- [x] コード全体の構造見直し → maintenance audit 実施済み（2026-06-10）。結果は下記タスクに展開
- [x] ドキュメント全体の見直し → 整合監査実施済み。実装との不一致・SSoT 違反・陳腐化なし。用語揺れ 1 件のみ（下記タスク）
- [x] `parseFrontmatter` の責務分離の要否判断 → **v0.9.0 で実施**と判断（Later から昇格。v0.10.0 タスク 8 の `meta.link_keys` で 3 系統目 + 設定注入が入り、条件が実質発火するため。整理を先行させ v0.10.0 の diff を純粋な機能追加にする）
- [x] convert / repair / simplify / init-meta / search(text) の CLI・format テスト追加（`format_repair.go` / `format_simplify.go` / `format_convert.go` / `printSearchText` が 0% カバレッジ。v0.10.0 受け入れ条件「既存出力の互換性を壊さない」の検証基盤。既存 cli_test.go / format_test.go の規約に乗せる。1〜2 commit）
- [x] path filter 部品の集約: `pathIncludeSQL`（search.go）/ `PathExcludeSQL` / `globMatch` / `validateGlobPatterns`（config.go）を `internal/core/pathfilter.go` へ移動（挙動変更なしの移動のみ。v0.10.0 で diagnose / query / graph / reachable に広げる前の置き場所確定）
- [x] path glob の Go 実装と SQLite GLOB の同値性テスト追加（`pathfilter_test.go`。同一入力を両実装に通して突き合わせるテーブル駆動テスト。SQL 一本化の要否は v0.10.0 タスク 2 の設計時に判断）
- [x] diagnose.go の note / asset conflict 集計の重複ブロック共通化（diagnose.go:36-82 と 84-123 が型名以外ほぼ同一。v0.10.0 タスク 1 の `diagnose --path` で触る箇所）
- [ ] NodeInfo 行スキャンの共通化（query_fetch.go / query_entry.go / search.go に 7 回出現 → `scanNodeInfo(rows)` ヘルパー）
- [ ] basename 一致 + root 優先の DB 側二重実装の統合（`resolveBasenameFromDB` と `findEntryByName` は同一ビジネスルールの別実装。ADR 0004 ルールの実装箇所を削減）
- [ ] util.go の責務整理（`noteResolveMaps` / `assetResolveMaps` を resolve_maps 系ファイルへ移動し、ファイル名と中身を一致させる。移動のみ）
- [ ] sentinel error の統一（`checkStale` の素の fmt.Errorf → `ErrSourceStale`、query_entry.go の file-not-in-index → `ErrFileNotRegistered`。errors.go の宣言方針に合わせる）
- [ ] `parseFrontmatter` の戻り値構造化 + parse.go / parse_frontmatter.go のファイル分割（本文パースと frontmatter パースは変更理由が異なる 2 責務。戻り値 2 値返しを構造体ベースへ。API 変更なし）
- [ ] ドキュメント用語統一: two-hop 表記（two-hop / 2hop / twohop / 2-Hop が rules/ / README / README.ja / examples skill で混在 → 1 表記に統一）

## v0.10.0 — path filter とグラフ到達性

特定領域だけを path filter で絞れる汎用診断機能群の前半。「範囲を絞って診て、辿れる」を 1 つの capability として出す。特定フォルダ・特定運用名に依存する専用機能は作らない。詳細・設計方針・受け入れ条件は [diagnostics.md](diagnostics.md)。

- [ ] `diagnose --path` で対象範囲を絞る（最優先。vault 全体の phantom ノイズ解消）
- [ ] path filter（`--path` / `--exclude`。search の既存名を正とする）をコマンド間で統一
- [ ] frontmatter key の raw path 値を graph edge にする設定（`meta.link_keys`。`related:` / `sources:` を backlinks / reachable に反映）
- [ ] `reachable --from <entry> --path <glob>` 到達性チェック（`meta.link_keys` の後に実装。raw path edge がないと false positive が出る）
- [ ] reachable 設計時に ADR 0002（map ベースと DB ベースの解決ロジック二重化の許容）を再評価する（root 優先ルールが map 側 / DB 側に分散している現状を踏まえ、グラフ走査が増えるタイミングで interface 抽象へ進むか判断。2026-06-10 audit 由来）
- [ ] subgraph export（`graph --path` で node / edge を JSON dump。類似判定等は呼び出し側）
- [ ] examples skill（`examples/skills/mdhop/`）と README / README.ja を v0.10.0 追加分に同期

## v0.11.0 — frontmatter 検査と search 強化

frontmatter の品質検査と search の強化。v0.10.0 を実 vault に当てた結果（残る phantom、`link_keys` の効き方）を設計判断の材料にする。詳細は [diagnostics.md](diagnostics.md)。

- [ ] frontmatter 任意 key の path-like value 検査（`meta-check`）
- [ ] frontmatter schema validation（`meta-validate`: 必須 key・enum・型・空値。mdhop.yaml の `meta.types` を検査側でも使う）
- [ ] search の computed fields（行数・リンク数を `--fields` / `--sort` で）+ `--fields` の meta key 出力対応
- [ ] `--where` の相対日付比較（`updated<today-90d`。**実施決定**）
- [ ] `changed --since` 変更ファイル列挙（git との差分を見てから要否判断）
- [ ] `[[note#見出し]]` の anchor 切れ検出（**実装が軽い場合のみ**。Obsidian 互換の fragment 正規化が重ければ見送り）
- [ ] anchor 検査の設計前に convert.go のパース骨格重複（`parseLinksForConvert` / `parseMarkdownSelfLinks` が parseLinks 系の走査ループを再実装）を畳むか判断する（本文走査をもう 1 種類足すと三重化するため。2026-06-10 audit 由来）
- [ ] computed fields の設計時に search.go の count / main クエリの WHERE 二重適用を query builder 化するか検討する（SELECT 句が動的になると崩れやすい。2026-06-10 audit 由来）
- [ ] examples skill と README / README.ja を v0.11.0 追加分に同期

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] パース層の package 化の再評価
  - **trigger**: v0.10.0 の `meta.link_keys` 着地後、parse 層の入出力（MetaConfig 注入）が安定した時点
  - **経緯**: internal/core は 30 ファイルで `rules/architecture.md` の分割検討条件（20 ファイル超 + 責務グループ明確）に到達。2026-06-10 の audit + module-boundary 判断で「mutation クラスタは resolveMaps / rewriteEntry / dbExecer の密共有で export 面が大きく未成熟、パース層のみが候補だが link_keys で入出力が変わる直前」として package 分割は見送り、file 境界整理（v0.9.0）のみ実施
- [ ] move 系（move.go / move_dir.go / move_helpers.go 計 1,602 行、18 関数パイプライン）の分割再評価
  - **trigger**: move に次の機能要求が来た時点（v0.10.0 / v0.11.0 では触らない）
  - **経緯**: 凝集はしているが backup / rollback のベストエフォート復元コードが分散（2026-06-10 audit）
