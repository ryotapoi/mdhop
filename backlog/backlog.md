# Backlog

## この backlog の運用ルール

### バージョンと見出し

- バージョン番号は SemVer に従う（機能追加 = minor、修正のみ = patch、破壊的変更 = major）
- 見出しはバージョン単位で切る。リリースとして出す価値のあるまとまりで区切り、goal の大きさには合わせない
- バージョン見出しが大きくなったら、配下にサブ見出しを立てて塊ごとに分ける。**サブ見出し 1 つが 1 goal の実行単位**（数コミットで終わる大きさ）。小さいバージョンならサブ見出しを作らず、バージョン見出しごと 1 goal にしてよい
- **未リリースのバージョン番号は挿入・繰り下げしてよい**。v0.1.0 と v0.2.0 がある状態で v0.1.0 直後にやりたい作業ができたら、それを新 v0.2.0 とし、既存 v0.2.0 を v0.3.0 にずらす。タグを打ったバージョンは動かせない
- 番号が決まらないタスクは番号なしの見出し（例: `## docs`）に置く。次のリリースへ同梱するか独立バージョンにするかは、タグを打つときに決める
- 挙動が変わらない変更（リファクタ・テスト・ドキュメント）だけでバージョンを刻まない

### タグと CHANGELOG

- タグと GitHub Release は 1:1 で作る
- CHANGELOG は**リリース時にまとめて書く**（`[Unreleased]` は通常使わない）。そのバージョンの項目が全て `[x]` になってから `## [vX.Y.Z]` に書く
- **書くときは各 commit の diff を読む**。backlog のタスク文をそのまま写さない。内部整理のつもりの項目でも、エラー文言等のユーザー影響が出ることがある
- 同一バージョン内で「A を作って後で X に変えた」場合は、A に触れず X だけを書く。前のバージョンの A を変えた場合は変更として書く
- 完了項目は `- [x]` にして残し、そのバージョンをリリースしたら見出しごと削除する（内容は CHANGELOG と commit に残る）。**削除は明示指示があるときだけ**

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] パース層の package 化の再評価
  - **trigger**: 「internal/core の外部（別バイナリ・別リポジトリ）から parse だけを再利用したい要求が出た時」または「parse の出力型 `linkOccur` への誤った参照が実際にバグを生んだ時」。副次ウォッチ: core が 60 ファイル級に到達、bridge 層（link_resolver / link_ambiguity）の増殖、`linkOccur` のフィールドが 12+ に増加
  - **経緯**: internal/core は 30 ファイルで `docs/rules/architecture.md` の分割検討条件（20 ファイル超 + 責務グループ明確）に到達。2026-06-10 の audit + module-boundary 判断で「mutation クラスタは resolveMaps / rewriteEntry / dbExecer の密共有で export 面が大きく未成熟、パース層のみが候補だが link_keys で入出力が変わる直前」として package 分割は見送り、file 境界整理（v0.9.0）のみ実施
  - **2026-07-08 再評価（maintenance-audit deep）**: 旧 trigger（link_keys 着地後の parse 入出力安定）は達成済みだが、結論は「できるが割に合わない」で見送り継続。parse クラスタ自体は依存クリーンだが、`linkOccur`（unexported）が 14 ファイルに漏れており、package 化は 8 フィールド全 export ＋ `LinkType` の循環回避のための型 package（`linkmodel`）新設を強制する。誤参照の実害も grep で観測されず、file 境界 + DI シグネチャ（`parseLinksWithLinkKeys([]string)`）で目的は達成済み。将来分割時の DAG 設計は audit 記録参照（core → parse → linkmodel）
  - **2026-07-17 再評価（maintenance-audit deep）**: trigger 未達（core 49 ファイル、`linkOccur` 8 フィールド、誤参照の実害なし）で見送り継続。追加確認: 横断ハブ（NodeType/LinkType 26 / NormalizePath 21 / openDBChecked 18 ファイル参照）を安定した base package に括り出さない限り、どう割っても全 package がハブへ依存する star 構造になる

- [ ] `delete --rm` のディスク削除と DB トランザクションの順序見直し（ディスク削除が tx より先行し、commit 失敗時にディスクと DB が恒久不整合・復旧手段なし。稀な事象なので急がないが、削除前 tmp 退避か tx 先行への作り替えを検討）

## 登録見送り（maintenance audit。再判断トリガー付きのものだけ残す）

- where.go の single / coalesce SQL 生成 4 関数の統合（2026-07-08）: 意見が割れた（「coalesce 版に吸収すれば 4→2 関数」vs「coalesce 版は優先順位ロジックが本質的に追加されており分離が妥当」）。新演算子追加が実際に来た時に再判断。`comparisonOpSQL` 等の SQL 断片生成の一本化だけなら低リスク
- sentinel error 14 個が本番コードで未使用（2026-07-08。テスト専用 fixture 化）: exit code 分岐等の要求が来た時に初めて活きる。error 文言変更時の二重管理だけ注意。2026-07-17: 分岐機能は引き続き要望待ちのまま、同義生文字列の sentinel 揃え + errors.go コメントの実態合わせのみ v0.16.6 でタスク化
- 構造整理系 4 件（2026-07-17 deep audit）: (1) db.go の NodeType/LinkType 型抽出（types.go）+ ディレクトリ列挙クエリの query 系への移動、(2) add.go の単一 363 行関数のステップ分解、(3) move_rewrite.go 末尾（425-508 行）の相対リンク構文変換の rewrite 系への移動、(4) resolveMaps への lookup メソッド集約（あわせて link_resolver.go:64-87 と build.go:242-285 の解決フォールバック順序重複、dryLinkResolver 到達不能メソッド、resolveLink/resolvePathTarget の build.go 同居を確認）。**trigger**: 該当ファイルに機能変更が入る時に同時実施を検討。v0.16.3 で同種の挙動不変整理を実施した直後のため寝かせる。詳細は 2026-07-17 audit 記録（A1/A3）参照
- フィールド表示ゲートの text/JSON 二重（2026-07-17）: 各コマンドの「どのフィールドを出すか」の分岐が text 用と JSON 用の 2 関数にコピーされている。共通化は薄い抽象を挟んで読解経路が増えるリスクと天秤で、条件が単純な現状は据え置き。**trigger**: フィールド追加時に text と JSON の食い違い（片方だけ更新）が実際に起きたら、フィールド数の多い query/stats から再判断
- needDiskMove（bool）の move クラスタ 6 ファイル貫通（2026-07-17）: 現状 disk 状態は 2 値なので bool が妥当。**trigger**: move の disk 状態に第 3 の分類（部分移動・シンボリックリンク等）が要求された時に enum 化
- meta_check.go / meta_validate.go のリネーム（2026-07-03）: 不採用。mdhop の慣例「CLI コマンド名 = ソースファイル名」を壊し、`mdhop meta-check` の実装を探す際の新しい乖離を作る。責務の判別問題は各ファイル先頭の doc comment で解決する（v0.13.0 の meta-validate プロファイル実装時に付与）
