# 変更履歴

[English](CHANGELOG.md) · 日本語

この変更履歴は、プロジェクトの [GitHub Releases](https://github.com/ryotapoi/mdhop/releases)、Git tag、コミット履歴、backlog の完了済みバージョングループを照合して再構成しています。GitHub Release がないバージョンは、tag が作成されていなくても記載しています。

## [Unreleased]

## [v0.22.1] - 2026-10-11

### 修正

- symlink の Vault root からの走査を修正し、`build`・`status` が実体パスと同じノート・asset・リンクを収集するようにした。root 自体を asset として登録せず、関連するディスク走査でも Vault 外保護を維持する。

### 性能

- 索引の配置解決を操作内で再利用し、`build`・`status` と関連する scan・ノート操作の filesystem 処理の重複を削減。索引の自己除外と symlink の扱いは維持する。

## [v0.22.0] - 2026-10-11

### 変更

- Vault 設定を `mdhop.toml` に変更し、`init-meta` の生成・更新も TOML に対応。ノートの frontmatter は引き続き YAML を使う。
- `--vault`・`--db`・`--config` で配置を独立して指定可能にし、既定索引をユーザーのキャッシュ領域へ移動。`mdhop paths` で実効的な配置を確認できる。再生成の失敗時は旧索引を保持し、対応するローカル filesystem では再生成中も並行参照できる。

### 追加

- 旧 YAML 設定を変換し、既定キャッシュ索引の再生成に成功してから旧設定・Vault 内索引を削除する `mdhop migrate` を追加。

### 修正

- scan・ノート操作での選択中索引の保護、移行の公開前の設定検証、設定更新・移行でのファイル権限保持を改善。

### アップグレード

- `mdhop.yaml` または `.mdhop/` を使う既存 Vault は `mdhop migrate --vault <path>` を実行すること。通常コマンドは旧配置を読み込まない。[旧配置からの移行](README.ja.md#旧配置からの移行)を参照。

## [v0.21.0] - 2026-10-08

### 追加

- 索引済み note 一件の tags / meta と任意の head を取得する `inspect` を追加。属性選択と head 指定は独立し、head 読取時は本文の欠落・更新を検出する。
- query に関係選択と単一関係の対象ページングを追加。入口・経由先を型付きで返し、twohop の各対象に全経由先を含め、次ページがある場合は `next_offset` を返す。
- query の表示条件（`--hide-path` / `--hide-tag`）と twohop の経由先選択（`--via*` / `--exclude-via*`）を独立させ、設定の無効化も別々に指定可能にした。query の head は返却 note、snippet は各関係の根拠となる実リンク出現を返す。

### 変更

- query の JSON を `entry`、選択した `backlinks` / `outgoing` / `2hoplink` 配列、`page` に変更。twohop は対象ごとの `relation` と `hidden_relation` を持ち、選択済みの空配列と未選択欄の省略を区別する。outgoing には索引済み親タグも含む。
- query の `--fields` / `--max-backlinks` / `--max-twohop` / `--max-via-per-target` / `--exclude` / `--exclude-tag` / `--no-exclude` を削除。既定は全対象・全経由先を返す。旧 config `exclude` は `query.via.exclude` キー不在時だけ経由先除外へ fallback し、search の挙動は維持する。

### 修正

- Markdown destination の backslash escape・HTML entity・percent escape を参照定義も含めて一貫して復号し、convert / move の書き換えで符号化されたファイル名区切りと fragment の意味を保持する。
- Markdown 表内の wikilink alias separator の escape と原文位置を保持。URL / email autolink や inline HTML で始まる表にも対応する。索引済み raw link の完全一致 resolve は source snapshot を使い、同じ原文の出現が異なる宛先・fragment を指す場合は曖昧として拒否する。
- escaped backtick 後のリンクを解析と書き換えで一貫して扱い、実際の inline code span 内のリンクは引き続き保護する。
- convert / move で wikilink alias や Markdown 表示文の内側を独立した Markdown link として書き換えないよう修正。表内の安全でない自己リンクなど、wikilink で意味を保持できないリンクは convert 時に原文を保持する。
- quoted frontmatter では backtick を含む wikilink の必須書き換えを許可。末尾の閉じ角括弧で参照先の意味が変わる書き換えは、file / index の変更前に拒否する。

### 性能

- twohop の経由先選択を backlinks 展開前に適用し、query のページ候補の保持量を制限。全経由先の関係展開は選択されたページ対象だけに行う。
- 同一 query の snippet 間で本文を共有し、抜粋行をコピーして返却結果が本文全体の buffer を保持しないようにした。
- SQL で printable ASCII の不一致 basename を事前除外し、Unicode / NFD と既存の解決規則を維持。ASCII が多い索引で allocation を削減する一方、Unicode が多い索引では走査コストが増える場合がある。

### 文書

- 要件・開発手順を整理し、README 両版と agent skill 例を更新。versioning と GitHub Release 公開を別手順にした。
- 実行時依存と Go の通知を `THIRD-PARTY-NOTICES.txt` に追加し、バイナリ配布時に `LICENSE` と同梱する手順を整備。

### アップグレード

- 既存 Vault はアップグレード後に `mdhop build` を実行すること。表文脈と index 解釈 version を保存するため、旧 index は再生成まで拒否される。
- query 呼出側を削除 flags と新出力に合わせて更新すること。関連 note の選択は `--relations`、入口の属性・head は `inspect`、ページングは単一関係の `--limit` / `--offset` を使う。[v0.21.0 への移行](README.ja.md#v0210-への移行)を参照。

## [v0.20.0] - 2026-10-04

### 追加

- Markdown 参照リンクの完全形・省略形と image 参照を索引化。同一文書内の定義から解決したリンクを outgoing、backlinks、2-hop、reachable、graph に反映し、未作成の参照先は phantom として扱う。`resolve --from ... --link ...` は索引内の使用箇所から解決する。

### 修正

- `convert` で Markdown の自己リンクに含まれる釣り合った括弧を保持し、往復変換でも fragment を壊さないよう修正。
- `move` 失敗時に書き換えたファイルの内容・permission とともに元の mtime を復元し、rollback 自体が再試行時の stale 判定を招かないよう修正。

### 変更

- head の返却行をノート全文の buffer から切り離し、大きなノートを複数検索したときのメモリ保持量を削減。
- `add`・`move` で、索引済み参照リンクの定義先が変わるか曖昧になる操作をファイル・index の変更前に拒否。参照定義は自動で書き換えない。

### アップグレード

- 既存 Vault はアップグレード後に `mdhop build` を実行すること。参照先を保存するため index schema が変わり、旧 index は再生成まで読み取り・更新できない。

## [v0.19.6] - 2026-10-02

### 変更

- 重複したテストと CLI 出力 capture の構成を整理し、異なる振る舞いの検証を維持。CLI の挙動は変更なし。

### 文書

- 位置引数、snippet のエラー、frontmatter link、書き込み時の安全性など、v0.19 の修正に合わせてコマンドヘルプ・仕様・要件・内部コマンドマップを更新。

## [v0.19.5] - 2026-10-02

### 修正

- 同じ行の wikilink と後続の Markdown link の間にある tag を保持し、tag・phantom 起点の 2-hop targets にも tag 除外を適用。
- `--where` 式の最初の演算子を条件として解釈し、値に `=` や `!=` が含まれても検索できるよう修正。LIKE pattern の末尾空白も保持。
- `meta-validate` で不正な frontmatter list 値を個別に報告し、現在のファイル行数を超える snippet 位置は error を返すよう修正。長い小数の数値 sort・比較も修正。number の保存用 sort 形式が変わったため、既存 Vault の index はアップグレード後に `mdhop build` で再生成する必要がある。
- `meta.types: null` の既存 `mdhop.yaml` に preset 型を保存できるよう修正。余剰位置引数を拒否し、`init-meta` の stdout YAML 出力失敗を error として返すよう修正。

### 文書

- `simplify` は引用符付き frontmatter wikilink を書き換え、raw `frontmatter_path` 値と tag は変更しないことを明記。

## [v0.19.4] - 2026-10-01

### 修正

- 複数 backtick で囲んだ inline code 内の link・tag を無視し、Unicode path link を build と後続の lookup で一貫して解決。
- `add`・`move` で、basename が同じだけの別 path link を無関係な phantom から昇格させないよう修正。
- note 移動時に前後空白を含む相対 Markdown link、大小文字が異なる相対参照先、大小文字だけの basename 衝突がある既存参照を保持。
- `disambiguate --scan` と link 変換時の asset link を保持。scan で dotted note basename を認識し、括弧を含む note link の変換後も参照先を保持。
- template move 完了時の text・JSON 出力に、実際の移動先を表示。

## [v0.19.3] - 2026-10-01

### 修正

- 並行 build の一時 DB と `init-meta --write` の一時設定ファイルを実行ごとに分離し、実行間の干渉と既存 `.tmp` ファイルの変更を防止。
- Vault 外を指す `add` path、外部 symlink 経由の Markdown 書き込み、Vault 外への move、未登録 asset を上書きする directory move を拒否。
- 正規化済み Unicode path に対応する実ファイルを content query・診断で参照し、NFD 名のファイルの delete に実 disk path を使用。
- `set` による flow style frontmatter の他キーの消失を防ぎ、引用符が必要な YAML key と隣接する comment・空行を保持。
- link 書き換え後の frontmatter metadata・index を更新し、disambiguate 後の phantom edge を実在する参照先に接続。再 build なしで結果が一致するよう修正。

## [v0.19.2] - 2026-10-01

### 変更

- 重複した SQL・formatter テストを整理し、内部の node upsert と stale check の共通処理を集約。CLI の挙動は変更なし。
- CLI テストの stdout・stderr capture を並行して読み、大量の出力による停止を防止。CLI の挙動は変更なし。

## [v0.19.1] - 2026-09-24

### 修正

- `set`・rewrite・move で、書き込みに失敗した file 自体も rollback 時の復元対象に含めるよう修正。
- `add` の曖昧性 error で、衝突する basename を特定できるよう修正。
- content query で、file の不存在とその他の filesystem stat error を区別するよう修正。
- `delete --rm` で、親 path が symlink を経由して Vault 外へ到達する場合の削除を防止。

### 変更

- 最低 Go version を 1.22 から 1.27.1 に引き上げ。

### ドキュメント

- requirements と command overview に、`move` における source-relative link 保持の例外を明記。

## [v0.19.0] - 2026-09-23

### 追加

- `set --list <json-array>` を追加。index を更新しながら frontmatter key を文字列リスト全体で置き換え、順序・重複・空文字列・空リストを保持する。

## [v0.18.0] - 2026-09-21

### 追加

- Vault や index を変更せず、未登録・変更済み・削除済みの note と asset の path を一覧する `status` を追加。
- 指定した frontmatter key の下で記録された link だけに、直接 backlinks / outgoing を絞り込む `query --link-key` を追加。
- `meta-check` と `meta-validate` の診断に 1 始まりの位置情報を追加し、指摘箇所を直接開けるようにした。

### 修正

- HTTP 以外の外部 URI を内部 link として index 化しないよう修正。
- code fence の marker と長さを正しく扱い、内部の link・tag・heading を解析しないよう修正。
- text / DOT 出力の書き込み失敗時に、CLI が非 0 で終了するよう修正。
- build から除外されている既存ファイルも `meta-check` が解決できるよう修正。
- build の一時 DB cleanup 失敗時に処理を続行せず、既存 index を維持するよう修正。

### 変更

- build・query・move・出力の内部経路をリファクタリングし、対象を絞った回帰テストを安定化。CLI 契約の変更は意図していない。

### ドキュメント

- `resolve` の asset 出力契約を整合させ、現行コマンドのドキュメントと test plan を更新。

## [v0.17.1] - 2026-09-14

### 追加

- `meta-check --kind auto` を追加。同じ frontmatter key に混在する path と wikilink を値ごとに判定し、一度に検証できるようにした。URL と空値は従来どおり許可する。

### 変更

- frontmatter の wikilink 抽出を Obsidian の property link に合わせた。引用符付き YAML scalar / list item 内の wikilink だけを index 化し、bare 値と block scalar 内の値は frontmatter link edge、phantom node、書き換え対象を生成しない。

### 修正

- frontmatter の全書き換え候補を、ファイル書き込み・DB 更新・path 移動より前に検証するよう修正。YAML の decode 結果と source の対応を証明できない場合は部分変更を行わず失敗し、dry-run も実行時と同じ検証結果を返す。
- ディレクトリ指定の `delete --rm` で未登録 asset の削除や空ディレクトリの cleanup に予期しない失敗が起きた場合、部分完了を成功として報告せずエラーにするよう修正。通常のファイル不在と、空でないディレクトリの扱いは維持する。

### ドキュメント

- 現行 CLI 挙動に合わせてコマンド仕様、test-plan、example agent skill を同期し、詳細なコマンド使用法を `mdhop <command> --help` に集約した。

## [v0.16.6] - 2026-07-26

### 変更

- コマンドのエラー出力を `error: <サブコマンド>: <メッセージ>` の形式に統一。個別のメッセージ側はコマンド名のプレフィックスを持たず、サブコマンド名の付与を 1 箇所に集約。エラー文言は変わるが、exit code と正常時の出力は変更なし。
- `resolve` と `query` に残っていた同義の生文字列エラーを、対応する sentinel error に揃えた。
- `init-meta --write`、`meta-check` の vault escape、diagnose formatter 出力の回帰テストを拡充し、move テストを責務別に分割。CLI の挙動は変更なし。
- link type の SQL filter、resolve の出力 field 定数、formatter の正規化を整理。CLI の挙動は変更なし。

### ドキュメント

- コンセプト資料・要件定義の `diagnose` 記述を修正。パース失敗も除外数も報告しないこと、opt-in の anchor 切れ検出が未記載だったことを実装に合わせた。
- core / mutate を分ける分類軸（Vault の Markdown ノートを書き換えるか）を明記。
- rules 側に残っていた旧語 `reconcile` / `canonicalize` を `disambiguate` / `simplify` へ更新完了。
- 当時のコマンド仕様書を正本とし、CLI ヘルプはその要約である位置づけを明記。

## [v0.16.3] - 2026-07-12

### 変更

- 内部の保守性を対象を絞ったリファクタリングで改善し、回帰契約を強化。CLI の挙動は変更なし。

## [v0.16.2] - 2026-07-12

### 修正

- rewrite と `set` 操作におけるロールバック復元と失敗報告を改善。
- ラップされたテンプレートの source lookup エラーを修正。

## [v0.16.1] - 2026-07-05

### 修正

- macOS の GitHub Actions runner を `macos-15` に固定し、`macos-latest` の移行により発生していた release CI の失敗を修正。CLI の挙動は変更なし。

## [v0.16.0] - 2026-07-05

### 変更

- `query` と `search` の複数 `--where` は、同じ metadata key に対するものを含め、常に AND で結合するよう変更。
- 明示的な OR には 1 つの `--where` 式の中で `||` を使用。既存の `!=` による除外挙動は維持。
- 新しい filter ルールに合わせて、コマンドヘルプ、仕様、要件、SQL 生成ドキュメント、回帰テストを更新。

旧来の暗黙的な同一 key の OR 挙動、たとえば `--where "status=active" --where "status=review"` に依存していた場合は破壊的変更です。

## [v0.15.0] - 2026-07-05

### 追加

- 相対日付による frontmatter 書き込みと frontmatter block の自動作成に対応する `set --date` を追加。
- `--where` filter に `||` 式を追加。
- `move --to-template` の挙動を完成させ、dry-run 計画、directory mode、日付部分の抽出、fallback、placeholder path 検証に対応。

### 変更

- 明示した `meta-validate --require` は、その実行に限り `meta.profiles` を上書きするよう変更。
- shared mover 経路、rollback failure の報告、幅広い回帰テストにより move 実行の安全性を改善。
- link resolution、resolve map 登録、utility 境界、出力 field 定数を整理。

## [v0.14.0] - 2026-07-04

### 追加

- `move` に destination template を追加し、ファイルと directory の移動先を template で指定できるようにした。

### 修正

- 単一ファイルの move を shared mover 経由に統一し、rollback failure の報告を改善。

## [v0.13.0] - 2026-07-04

### 追加

- index を更新しながら frontmatter の単一 key を安全に書き換える `set` を追加。
- path-scoped な `meta-validate` require profile を追加。
- `search` に sampling、count-only 出力、`coalesce(key1, key2, ...)` filter を追加。

### 変更

- query のデフォルト上限を core 定数へ集約。
- 各コマンドの `--help` に field、挙動メモ、使用例を追加。
- 正確なコマンド詳細を `mdhop <command> --help` に寄せ、example agent skill を簡素化。

## [v0.12.1] - 2026-06-24

### 修正

- Linux における NFD/NFC filename の Unicode 正規化済み path 処理を修正。
- 移動した相対 link が末尾 `/` 付きで書き換えられる問題を修正。

## [v0.12.0] - 2026-06-13

### 追加

- source note を選択する `repair --path` と `--exclude` filter を追加。

### 変更

- filesystem 間で path resolution を一貫させるため、index 内の path を NFC に正規化。
- `meta-check` が directory path を受け付けるよう変更。

## [v0.11.0] - 2026-06-11

### 追加

- `today-90d` などの相対日付を `--where` 比較で利用可能にした。
- `search` の計算フィールドと `meta.<key>` の出力選択を追加。
- `diagnose` に heading anchor 検査を追加。
- frontmatter の path と wikilink 参照を検証する `meta-check` を追加。
- frontmatter schema を検査する `meta-validate` を追加。

### 修正

- inline-code heading と stale target に対する anchor 検査を修正。
- 相対日付比較で date 宣言された key を必須にした。

## [v0.10.0] - 2026-06-11

### 追加

- `diagnose` に source note 用の `--path` と `--exclude` filter を追加。
- `query` に結果 path 用の `--path` filter を追加。
- `meta.link_keys` により、frontmatter の raw path 値を link edge として index 化できるようにした。
- link の到達性を検査する `reachable` を追加。
- JSON と Graphviz の subgraph を出力する `graph` を追加。

### 修正

- vault path が current directory の場合に asset が収集されない問題を修正。

## [v0.9.0] - 2026-06-10

### 変更

- search と path filter の挙動を厳密化し、glob matching を SQLite GLOB の挙動と整合。
- DB 側の basename resolution を root-priority ルールに統一し、diagnose 内部処理を改善。
- 既存コマンドの CLI テストを拡充し、example skill を更新。

主に保守性と品質を向上させるリリースで、新しい CLI command の追加はありません。

## [v0.8.0] - 2026-05-09

### 追加

- `search` に `--no-tags`、`--no-outgoing`、`--no-incoming` の isolation filter を追加。
- `--where "priority NOT EXISTS"` のような frontmatter `NOT EXISTS` filter を追加。

### 修正

- 既存 note と metadata 欠落時の `search` filter 挙動を厳密化。

## [v0.7.1] - 2026-05-09

### 変更

- プロジェクトの agent workflow で Codex Goals を有効化。

この tag は agent workflow の変更のみで、CLI の挙動は変更していません。

## [v0.7.0] - 2026-05-07

### 追加

- frontmatter wikilink の parse と rewrite を `add`、`update`、`move`、`disambiguate`、`simplify` に追加。
- `claude -p` の stream output を assistant text と tool name として表示。

### 修正

- frontmatter wikilink の scan 時に YAML comment を無視。
- YAML block scalar 内の frontmatter wikilink を保持し、block scalar body をより深い indent の行に限定。
- note の移動時に相対 frontmatter wikilink を正しく rewrite。
- Ctrl+C 時に `runner.sh` を正常停止。

## [v0.6.1] - 2026-05-06

### 変更

- internal type の境界、query と formatter のファイル構成、Go formatting の自動化、agent workflow の検査を改善。

この tag は保守と workflow の変更のみで、記録された CLI 挙動の変更はありません。

## [v0.6.0] - 2026-03-22

### 追加

- frontmatter metadata の保存、metadata type、sort value normalization を追加。
- `query` に `--where` filter と `--fields meta` を追加。
- entry を指定しない Vault 全体の note 検索向けに `search` を追加。
- frontmatter type の scaffold を生成する `init-meta` を追加。
- 同一 key の AND filter 用に `&&` 式を追加。
- 曖昧な link の build error に候補 path を追加。

## [v0.5.0] - 2026-03-18

### 変更

- move、resolve、repair、simplify、database、tag、node update の共通 helper を集約。
- directory move、collateral rewrite、asset resolution、phantom node、frontmatter parse のテストを拡充。
- 公開ドキュメントと agent workflow の構成を整理。

主に内部 architecture と保守性のリリースで、新しい CLI command の追加はありません。

## [v0.4.0] - 2026-02-26

### 追加

- wikilink と Markdown link の形式を変換する `convert` を追加。
- 冗長な path link を basename 形式へ短縮する `simplify` を追加。

### 修正

- `move` と `movedir` を妨げる外部 rewrite stale check を削除。

## [v0.3.0] - 2026-02-26

### 追加

- Markdown 以外の asset を index 化・管理できるようにし、asset link、resolution、update、move、delete、query、stats に対応。

## [v0.2.0] - 2026-02-25

### 追加

- 壊れた path link と Vault 外を指す link を修復する `repair` を追加。

## [v0.1.0] - 2026-02-23

### 追加

- Markdown link indexer と SQLite-backed CLI の初回公開リリース。
- `build`、`add`、`update`、`delete`、`move`、`disambiguate`、`resolve`、`query`、`stats`、`diagnose` を追加。
- basename が曖昧な場合の root-priority を含む厳密な link resolution と、自動 disambiguation に対応。
- Obsidian 互換 tag、Unicode 対応、JSON/text 出力、Vault 設定、index 除外 path、`delete --rm` による任意の disk file 削除に対応。
- directory move、directory delete、collateral link rewrite、Coding Agent 向け example skill を追加。
