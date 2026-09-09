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

- [x] Regenerate `llm-wiki/03-linktype-matrix.md` and update `llm-wiki/06-resolve-rewrite.md` for quoted-only frontmatter wikilink helpers
  - disposition: follow_up_soon
  - `collectFrontmatterWikilinks` / `wikilinksFromQuotedScalar` へ置き換わった抽出 helper と ADR 0023 の事実に合わせ、削除済みの `parseFrontmatterWikilinks` 参照を除去する
  - `06-resolve-rewrite.md` の stale な行参照と quoted-only 表記を現行 source に同期する
  - `regen: full` に従い sources から再編纂し、行番号・関数名が現行 source を指すことを確認する
  - `03-linktype-matrix.md` の `sources` 完了条件に `docs/decisions/0023-frontmatter-wikilink-quoted-only.md` を含める

### 既存機能を保つコードの簡約

- [x] 不要な状態・旧 API・中継処理を削除し、理解・変更・検証の負担を減らす
  - DB を変更しないリンク検証は解決パスを直接返し、一時 ID・双方向マップ・採番状態をなくす。解決順序の共有は維持する
  - 書き換えエントリのファイル別振り分けを既存の適用関数へ集約し、呼び出し元の重複処理と移動用ラッパーを削除する
  - 未使用の `ExpandMoveTemplate` を削除し、有用なテストは現行の `PlanMoveTemplate` / `MoveTemplate` を検証する形に移す
  - asset 対応前の `HasNonMDFiles` と専用テストを削除し、対応する test-plan を現行の directory move/delete 契約へ同期する
  - 関連する設計記録とリンク解決ガイドを同期し、共有ヘルパー表を現ソースから再生成する
  - ユーザー向け機能、保存データ、CLI の入出力契約、曖昧リンクの拒否・ルート優先・失敗時の復元保証を維持する
  - 集中テスト、`go test ./...`、`go build ./...`、`go vet ./...` を通し、変更前後の実バイナリで出力・終了コード・ファイル・DB を比較する

## v0.17.1

### YAML decode 差異による frontmatter wikilink の書き換え不一致

目的は、書き換えが成功したのにファイルと DB が食い違い、再 build でリンク先が変わる不具合を防ぐこと。Source: Goal Review v0-17-0 (opus + sol)、disposition: follow_up_soon。

各項目を 1 Change とし、1 → 2 → 3 の順に進める。

共通制限: 既存の抽出規則を維持し、YAML 全形式への対応や無関係な整理へ広げない。変更に直接必要な文書だけを同じ Change で同期し、検証は `docs/rules/verification.md` に従う。

再利用資料: main の未コミット差分は旧版。後続修正版と評価記録は `/private/tmp/mdhop-v0171-repair`（記録は配下の `tmp/workflow/v0-17-1-repair/changes/yaml-frontmatter-rewrite/`）。既報の回帰を確認し、今回の項目に必要な差分だけを選ぶ。未コミット差分や既存の完了印を合格済みとは扱わない。

- [x] 1. quoted frontmatter の書き換え候補と source の対応を、副作用のない変換として検証する
  - 予定した出現だけを更新し、同じ規則で再抽出した結果が予定と一致する候補を返す。対象外のテキスト・リンクと重複件数を保ち、対応不能な decode/source 差異や置換後の YAML 解釈差異は明示的に拒否する
  - 通常の quoted scalar / list の更新を維持する。未対象の decode 差異や、本文だけを更新する際の無関係な不正 frontmatter を新たな拒否理由にしない
  - 既報の誤置換・過剰拒否を検出する局所的な回帰テストで確認する。実更新経路への接続は 2 で行う

- [x] 2. 実更新の全候補を副作用前に検証する（1 に依存）
  - 共通 rewrite・scan の実更新と Move / MoveDir / template move に接続する。移動では外部ファイルと移動ノート自身の書き換えを同一操作として検証する
  - 対応不能な候補があれば、最初の file / DB 更新・移動・出力先作成より前にエラーにし、操作前の状態を保つ。後続候補の失敗を、先行更新の rollback だけで処理しない
  - 成功後のファイルと関連 DB edge が再 build 後も整合すること、および複数ファイル・移動時の事前拒否を確認する。既存のエラー出力・復元保証・dry-run の計画表示と無変更を維持する

- [x] 3. dry-run でも実行時と同じ対応不能候補を拒否する（2 に依存）
  - scan と template move の dry-run にも同じ対応検証を適用し、書き換え不能なら非ゼロ終了と stderr で明示し、成功結果を出さない
  - 同じ入力状態で実行時と対応検証の成否が一致し、成功・失敗とも file / DB / path を変更しないことを確認する。将来の I/O 障害まで成功保証に含めない

### directory `delete --rm` の部分失敗を可視化

- [x] directory 指定の `delete --rm` で未登録 asset または空 directory の cleanup に予期しない失敗があった場合、成功扱いにせず部分完了として報告する
  - 非ゼロ終了し、stdout に成功結果を出さない。stderr に失敗した path と原因、登録済みファイルの削除と DB 更新は完了済みであることを示す
  - `NotExist`、通常の skip、成功時の出力契約は維持する

### 正本・入口文書の現行契約同期

- [x] 現行コード・CLI help・仕様を基準に、正本と作業入口に残る過去の契約を同期する
  - `docs/rules/01-concept.md` と `docs/rules/03-data-model.md` の旧 query flag、未実装の `note_resolution.ambiguous` 設定、asset の記載漏れ、列名 `exists_flag` を現行契約へ合わせる
  - `AGENTS.md` と `CLAUDE.md` の maintenance-audit 案内を現在の skill に合わせる

### test-plan の現行 CLI surface 同期

- [x] `docs/specs/test-plan.md` に不足している重要な現行 CLI 契約を補う
  - 確認対象: `set`、`search`、`reachable`、`graph`、`meta-check`、`meta-validate`、`init-meta`

### `llm-wiki` の古い参照と説明の修正

- [x] `llm-wiki/` の作業を誤誘導する古い参照・説明を、該当ページの `regen` に従って修正する

### MoveDir rollback test の環境非依存化

- [x] MoveDir の rename 失敗時の復元保証を、実行権限に左右されず検証できるようにする
  - 現在 root で skip されるテストが守るファイル・本文・外部リンク・DB の復元について、既存テストで不足する検証を補う

### mtime test の固定 sleep 除去

- [x] query、move、disambiguate の stale detection テストから固定待機を減らす
  - stale / non-stale の検出は維持する

### example skill の references 見直し

- [x] `examples/skills/mdhop/references/` と CLI help の重複管理を減らす
  - `references/query.md` と `references/commands.md` を `mdhop <command> --help` と照合し、不要な重複を削る
  - 必要な安全制約・コマンド選択の案内を保ち、削除箇所への参照を直す

## 登録見送り（maintenance audit。再判断トリガー付きのものだけ残す）

- where.go の single / coalesce SQL 生成 4 関数の統合（2026-07-08）: 意見が割れた（「coalesce 版に吸収すれば 4→2 関数」vs「coalesce 版は優先順位ロジックが本質的に追加されており分離が妥当」）。新演算子追加が実際に来た時に再判断。`comparisonOpSQL` 等の SQL 断片生成の一本化だけなら低リスク
- sentinel error 14 個が本番コードで未使用（2026-07-08。テスト専用 fixture 化）: exit code 分岐等の要求が来た時に初めて活きる。error 文言変更時の二重管理だけ注意。2026-07-17: 分岐機能は引き続き要望待ちのまま、同義生文字列の sentinel 揃え + errors.go コメントの実態合わせのみ v0.16.6 でタスク化
- 構造整理系 4 件（2026-07-17 deep audit）: (1) db.go の NodeType/LinkType 型抽出（types.go）+ ディレクトリ列挙クエリの query 系への移動、(2) add.go の単一 363 行関数のステップ分解、(3) move_rewrite.go 末尾（425-508 行）の相対リンク構文変換の rewrite 系への移動、(4) resolveMaps への lookup メソッド集約（あわせて link_resolver.go:64-87 と build.go:242-285 の解決フォールバック順序重複、dryLinkResolver 到達不能メソッド、resolveLink/resolvePathTarget の build.go 同居を確認）。**trigger**: 該当ファイルに機能変更が入る時に同時実施を検討。v0.16.3 で同種の挙動不変整理を実施した直後のため寝かせる。詳細は 2026-07-17 audit 記録（A1/A3）参照
- フィールド表示ゲートの text/JSON 二重（2026-07-17）: 各コマンドの「どのフィールドを出すか」の分岐が text 用と JSON 用の 2 関数にコピーされている。共通化は薄い抽象を挟んで読解経路が増えるリスクと天秤で、条件が単純な現状は据え置き。**trigger**: フィールド追加時に text と JSON の食い違い（片方だけ更新）が実際に起きたら、フィールド数の多い query/stats から再判断
- needDiskMove（bool）の move クラスタ 6 ファイル貫通（2026-07-17）: 現状 disk 状態は 2 値なので bool が妥当。**trigger**: move の disk 状態に第 3 の分類（部分移動・シンボリックリンク等）が要求された時に enum 化
- meta_check.go / meta_validate.go のリネーム（2026-07-03）: 不採用。mdhop の慣例「CLI コマンド名 = ソースファイル名」を壊し、`mdhop meta-check` の実装を探す際の新しい乖離を作る。責務の判別問題は各ファイル先頭の doc comment で解決する（v0.13.0 の meta-validate プロファイル実装時に付与）
