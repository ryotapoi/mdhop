# Backlog

## v0.16.4〜v0.16.6（maintenance-audit deep 2026-07-17 findings。全て機能追加なしのため patch。バージョン順に実施 — テスト安全網を先に入れてからリファクタに進む）

### v0.16.6（エラーメッセージの整備。エラー文言が変わりうる）

- [x] sentinel error の整備: resolve.go:34・query_entry.go:80/84/126 の同義生文字列エラーを対応 sentinel の `%w` ラップに揃え、internal/core/errors.go の冒頭コメントを実態（本番コードは分岐していない・テスト検証用 + 将来の分岐余地）に書き直す。exit code 分岐等の機能は作らない（要望待ち。「登録見送り」の sentinel 項参照）
- [x] エラーメッセージのプレフィックス統一: 個別メッセージは裸に統一し、コマンド文脈が必要なら main.go のエラー整形 1 箇所で subcommand 名を付与する方向を推奨。実装時に `search:`（search.go:91-）等の既存プレフィックスがコマンド名でなく flag 文脈を指していないか確認してから確定し、決めた規約を conventions.md に一文追記

### docs（バージョン不要。随時、各 1 commit。以下 7 件は v0.16.6 に同梱実施済み）

- [x] rules/01-concept.md の旧語「reconcile」「canonicalize」を実装・specs で採用済みの「disambiguate」「simplify」に更新
- [x] overview.md を正・CLI ヘルプを従(要約、詳細は spec 参照)とする正従関係を docs/specs/overview.md 自身の冒頭に一文明記（着地先を docs/rules/information-management.md から変更: 同ファイルは canon-sync 同期対象でローカル追記が上書き消失するため。現運用は同一 commit で両方手書き・破綻なし。運用変更は不要で宣言だけ）
- [x] db.go の NodeType コメントに SQL リテラル運用ルールを明文化（「SQL 内の `'note'` 等のリテラルは grep で追う前提。node type の増減・改名時は非テスト core 全体を検索する」。非テスト core に 40 箇所前後分布 — 空白や `IN (...)` の書式で集計が揺れるため、実施時に grep で確定する）
- [x] 02-requirements.md / 03-data-model.md に残る旧語「reconcile」「canonicalize」の更新（2026-07-25、01-concept.md の用語更新 v0.16.6 の follow-up）: 見出し 3 行の語置換で完了（`bd7c26c`）。capture 時の「2 経路のどちらを指すかの意味判断が要る」という前提は誤りだった — 各節の本文は操作そのもの（衝突遷移の検知、リライト対象 source の特定）を書いており起動方法に触れていないため、`add` の auto-disambiguate と `disambiguate --name` はいずれも同一操作への入口で、経路を選ぶ必要はなかった。`canonicalize / shorten` の `shorten` は先例 `78c5b0c` が英語別名を残さない方針だったこと、正本 overview.md に `shorten` が登場しないことから道連れで削除
- [x] 01-concept.md の diagnose 記述と overview.md の食い違い解消（2026-07-25、01-concept.md の用語更新 v0.16.6 の follow-up）: 実装を確認した結果パース失敗の報告経路は存在せず（frontmatter の YAML エラーは `internal/core/parse_frontmatter.go:47-48` で握りつぶされ error が伝播しない）、正本 overview.md が正しかったため 01-concept.md 側を修正（`2a65a3b`）。capture が名指ししていたのは `01-concept.md:91` だけだったが、同じ誤りが `docs/rules/02-requirements.md:96`（§2.6 diagnose の「除外数、パース失敗一覧」）にもあった — 「除外数」を出力する機能も存在しない（`DiagnoseOptions.Exclude` は入力側のパス glob フィルタ）ため同一 commit で修正し、あわせて実装済みで未記載だった opt-in の anchor 切れ検出を追記した
- [x] 01-concept.md §3.2 の core / mutate 分類のずれ整理（2026-07-25、01-concept.md の用語更新 v0.16.6 の follow-up）: 分類軸「Vault の Markdown ファイルを書き換えるか（= Git 差分に出るか）」を §3.2 に明記して完了（`ddce9e4`）。capture が疑っていた「書き込み経路の一群がまとめて欠落」という前提は誤りだった — 欠陥は列挙漏れではなく分類軸が書かれていないことで、軸を書けば `build`/`update`（`.mdhop/` は gitignore 済みなので Git 差分に出ない）が core である理由も、`set`/`delete --rm` が mutate 側であることも導ける。`set`/`delete`/`init-meta` の列挙追加は見送り: 両リストは「など」で閉じており網羅を主張していないうえ、コマンド仕様の正本は `docs/specs/overview.md`（`03b6522` で宣言）なので、01-concept.md に列挙を増やすことは正本の複製にあたる。`init-meta --write` が書くのは `mdhop.yaml` で vault ノートではないため、この軸の対象外
- [x] 01-concept.md:24 の `rename` 表記を直すかの確定（2026-07-25、01-concept.md の用語更新 v0.16.6 の follow-up）: **イベント記述として原文のまま残す**と確定（ファイル変更なし）。Goal Review で判定が割れた件（他系統 reviewer = 同一ファイル内の貫通漏れ、自系統 reviewer = イベント記述で許容）は後者を採用。根拠: 24 行目が属する §2.1 の見出しは「LLM/Coding Agent の困りごと」で、兄弟項目も「grep 依存になる」「曖昧になる」という事象の列挙でありコマンド名を一切含まない。81 行目の「ファイル名変更時にリンクを追従させたい（move）」は需要→コマンドの対応付けなので `（move）` が付くのが正しく、課題提示である 24 行目とは役割が違う。したがって 81 行目が更新済みであることは 24 行目の貫通漏れの証拠にならない。`rename` は `docs/` 全体でこの 1 箇所のみで、コマンド語彙としての残存もない

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
