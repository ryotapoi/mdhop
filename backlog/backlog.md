# Backlog

## この backlog の運用ルール

### バージョンと見出し

- バージョン番号は SemVer に従う。1.0 未満では機能追加と互換性を壊す変更を minor、修正のみを patch とする。1.0 以降では機能追加を minor、修正のみを patch、互換性を壊す変更を major とする
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

## v0.17.0

### `meta-check --kind auto` による混在参照の検証

- [x] `meta-check` に `--kind auto` を追加し、同じ frontmatter key に path / wikilink / URL が混在していても 1 回で検証できるようにする
  - 既存の `--kind path` / `--kind wikilink` と既定値 `path` は維持し、互換性を壊さない
  - 検査対象は既存どおり `meta` table に格納された YAML scalar と scalar list item の値とし、graph edge や source frontmatter の raw scan 結果を合流しない
  - frontmatter の wikilink は Obsidian の property 形式に従い、引用符で囲まれた値だけを対象とする。YAML parser が引用符を外した後の値を wikilink として判定し、bare `[[Note]]` は対象に含めない
  - 値ごとに trim したうえで、空値と `://` を含む URL は従来どおり skip、`[[` で始まる値は wikilink、それ以外は path として検査する
  - path 判定では、ディレクトリ参照・相対パス・vault escape を含む既存の解決規則を維持する
  - wikilink 構文として解釈できない値は `not_wikilink`、構文は正しいが解決できない wikilink と存在しない path は `not_found` として区別する。`ambiguous` / `vault_escape` も既存どおり維持する
  - `sources:` に引用符付きの実在 wikilink、実在する raw path、URL が混在する成功例、bare wikilink を検査対象に含めない例、各 reason の失敗例を core / CLI の回帰テストに追加する
  - help と `docs/specs/overview.md` を同期し、`auto` が Obsidian 互換の property 値を一つの入力経路から検査することを明記する

### frontmatter wikilink 抽出の Obsidian 準拠

- [x] frontmatter の wikilink は、Obsidian が property link として扱う引用符付き YAML scalar / list item の中だけから抽出する
  - double quote / single quote で囲まれた `[[Note]]` は従来どおり `frontmatter_wikilink` edge として扱う
  - bare `key: [[Note]]` と bare list item `- [[Note]]` は YAML 上の nested sequence であり、frontmatter のリンクとして扱わない。edge・phantom・`meta-check` issue を生成せず、graph / reachable / 書き換え系コマンド / mutation 時のリンク検証の対象にも含めない
  - Markdown 本文内の `[[Note]]` は従来どおり wikilink として扱い、今回の変更対象に含めない
  - scalar / list、double quote / single quote / bare の組み合わせを parser・build・書き換え系の回帰テストで固定し、bare 対応を前提とする既存テストを Obsidian 互換の期待値へ更新する
  - `docs/specs/overview.md` と frontmatter wikilink 抽出に関する ADR の事実記述を同期し、bare wikilink を将来対応として残さない

### llm-wiki linktype matrix の quoted-only 同期

- [ ] Regenerate `llm-wiki/03-linktype-matrix.md` and update `llm-wiki/06-resolve-rewrite.md` for quoted-only frontmatter wikilink helpers
  - disposition: follow_up_soon
  - `collectFrontmatterWikilinks` / `wikilinksFromQuotedScalar` へ置き換わった抽出 helper と ADR 0023 の事実に合わせ、削除済みの `parseFrontmatterWikilinks` 参照を除去する
  - `06-resolve-rewrite.md` の stale な行参照と quoted-only 表記を現行 source に同期する
  - `regen: full` に従い sources から再編纂し、行番号・関数名が現行 source を指すことを確認する
  - `03-linktype-matrix.md` の `sources` 完了条件に `docs/decisions/0023-frontmatter-wikilink-quoted-only.md` を含める

## v0.17.1

### quoted frontmatter wikilink rewrite when YAML decode differs from file text

- [ ] YAML decoder の値とファイル上のテキストが異なる quoted frontmatter wikilink を正しく rewrite する
  - disposition: follow_up_soon
  - double quote / single quote / multiline quoted scalar の frontmatter wikilink を対象とする
  - rewrite はファイルを更新するか、明示的に失敗する。静かな不一致を残さない
  - 回帰テストで quoted scalar の decode 差異と rewrite 動作を固定する
  - Source: Goal Review v0-17-0 (opus + sol)

### directory `delete --rm` の部分失敗を可視化

- [ ] directory 指定の `delete --rm` で未登録 asset または空 directory の cleanup に予期しない失敗があった場合、成功扱いにせず部分完了として報告する
  - 登録済みファイルのディスク削除と DB 更新が完了した後の失敗であることを示し、処理前の状態へ戻ったように見せない
  - 安全に継続できる cleanup は続け、失敗した path・操作・原因を可能な限り一度の実行で収集する
  - cleanup failure が1件以上あれば非ゼロ終了し、stdout に成功時の text / JSON result を出さない。stderr には部分完了と収集した失敗を、AI が後続処理を止めて残存 path を確認できる形で出す
  - `NotExist` は削除済みとして許容し、hidden directory の skip と非空 directory での上位 cleanup 停止は現在どおり正常に扱う。それ以外の walk・remove・directory inspection failure は握り潰さない
  - 成功時の `DeleteResult` と JSON schema は変更せず、構造化された partial result は追加しない
  - root / permission の実行環境に依存しない失敗注入で、登録済みファイルと DB は更新済み、削除失敗した未登録 asset は残存、error は部分完了と失敗 path を示すことを回帰テストで固定する
  - `go test ./...` が通ることを確認する

### 正本・入口文書・backlog の現行契約同期

- [ ] 現行コード・CLI help・仕様を基準に、正本と作業入口に残る過去の契約を同期する
  - `docs/rules/01-concept.md` と `docs/rules/03-data-model.md` から旧 query context flag と未実装の `note_resolution.ambiguous` 設定を除き、現在の query field、strict な曖昧解決、root 優先規則を記載する
  - node model に asset を含め、SQLite schema の列名を実装どおり `exists_flag` とする
  - `AGENTS.md` と `CLAUDE.md` の maintenance-audit 案内を、常に全 phase を実行する現在の workflow と矛盾しない状態にする
  - `delete --rm` の transaction 順見直し項目を「自動 rollback はないが、`--rm` なしの再実行で DB を復旧できる」と直し、実装優先度は再評価しない
  - ユーザー向け挙動と保存データは変更しない

### obsolete asset rejection helper の削除と test-plan 同期

- [ ] asset 対応前の `HasNonMDFiles` と専用テストを削除し、`docs/specs/test-plan.md` を現在の CLI surface に同期する
  - directory move/delete は非 Markdown asset を拒否せず、登録済み・未登録の扱いに従って一緒に操作する現在の契約を記載する
  - `set`、`search`、`reachable`、`graph`、`meta-check`、`meta-validate`、`init-meta` について、最小正常系、主要な失敗系、安定契約を判定できる項目を追加する
  - production から参照されない helper と、その存在だけを固定するテストを残さない
  - `go test ./...` が通ることを確認する

### `llm-wiki` の sources からの再編纂

- [ ] `llm-wiki/` の全ページを各 `regen` 区分に従って現 source から再編纂する
  - `regen: full` は手動の行番号修正ではなく sources から再抽出し、compiled page も現在の責務と導線から更新する
  - 記載する関数位置・呼び出しサイトが現在の source を指し、存在しない行や関数へ誘導しないことを確認する
  - index から全ページへ到達でき、frontmatter の `sources` と本文中の source path が実在することを確認する

### MoveDir rollback test の環境非依存化

- [ ] MoveDir の途中 rename 失敗を既存の `moveRename` seam から注入し、root を含む全実行環境で rollback を検証する
  - 実効 UID による `t.Skip` と directory permission に依存する失敗生成をなくす
  - 失敗後に移動元・移動先、移動ファイル本文、外部 rewrite、DB node/edge が実行前の状態へ戻ることを確認する
  - `go test ./internal/core/ -run 'TestMoveDir_Rollback'` と `go test ./...` が通ることを確認する

### mtime test の固定 sleep 除去

- [ ] stale detection のテストで使う固定 `time.Sleep(1100 * time.Millisecond)` を、明示的な mtime 設定へ置き換える
  - `os.Chtimes` 等で DB 記録値と異なる mtime を作り、stale / non-stale の判定対象は変えない
  - query、move、disambiguate の対象テストから固定 sleep がなくなることを確認する
  - `go test ./...` が通ることを確認する

### vault escape 判定の単一化

- [ ] Build と Repair が共有する vault escape 判定を一つの predicate に集約する
  - relative link、vault-relative path、basename の現在の判定結果を変えない
  - Repair 固有の対象リンク選択と rewrite 方針は共通 predicate へ混ぜない
  - Build の validation error と Repair の rewrite / skip を既存テストで固定し、`go test ./...` が通ることを確認する

### example skill の references 見直し

- [ ] `examples/skills/mdhop/references/` が、コマンド詳細の正本を `mdhop <command> --help` とする現在の skill 方針では不要になっているか見直し、重複した reference を撤去する
  - `references/query.md` / `references/commands.md` の内容を `SKILL.md` と各コマンドの `--help` に照合し、reference にしかない必須の安全制約・コマンド選択基準がないことを確認する
  - 固有情報が残っている場合、正確なフラグ・既定値・出力仕様・例は CLI help に、agent が最初に必要とする選択基準と横断ルールだけは `SKILL.md` に移してから reference を削除する
  - `SKILL.md` は薄い入口として、用途からコマンドを選ばせた後に正確な使い方を `mdhop <command> --help` から取得させる構成を維持する
  - `SKILL.md` の `References` 節と不要になった `references/` を削除し、README・配布物・リポジトリ内にリンク切れや古い参照が残っていないことを確認する

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
