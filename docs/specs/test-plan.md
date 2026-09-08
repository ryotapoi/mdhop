# テスト設計表（軽量版・厳密モード）

コマンドごとに「必要なテスト」を箇条書きで整理する。
詳細な入力/期待結果/テストデータは各コマンド実装前に拡張する。

## build

- 正常系: Vault内の全Markdownを解析してDBを作成
- 再build: 既存DBを上書き
- 失敗系: 曖昧リンクが1件でもあればエラー
- 失敗系: 失敗時は成果物を消すが、既存DBは残る
- 失敗系: vault外への相対パスリンクはエラー
- 解析対象: `**/*.md` のみ、`.mdhop/` 配下は除外
- case-insensitive basename: `[[note]]` → `Note.md` に解決
- case-insensitive basename衝突 + basenameリンク → ambiguousエラー
- ルート優先: basename重複 + ルート直下にファイルあり → build成功、basename リンクはルートに解決
- ルート優先なし: basename重複 + ルートになし → ambiguousエラー（従来通り）
- ルート優先ラウンドトリップ: build 2回で結果が同一
- basename衝突あり + パス指定リンクのみ → エラーにならない
- 複数ユーザーエラー（曖昧+escape混在）が最大N件まで収集されること
- 1件時は従来フォーマット維持（後方互換）
- 上限到達時の打ち切りとサマリーメッセージ
- mtime: 全noteにos.Statの値が設定される
- edges: wikilink / markdown link → 対応するedgeが作成される
- backlink: B→Aのedgeが正しく作成される
- 相対パス解決: `./Target.md` → `dir/Target.md`, `../Root.md` → `Root.md`
- `/` 始まりリンク: vault ルート相対で解決
- self-edge: `[[#Heading]]` → source=target=自ファイル、subpath保持
- phantom: 未解決リンク先はphantomノード（exists_flag=0, path=NULL）
- phantomのnameは元表記を保持（node_keyのみlower）
- alias付きphantom: `[[Missing|alias]]` → name="Missing"
- inline tag → tagノード + edge (link_type=tag)
- frontmatter tag → tagノード + edge (link_type=frontmatter)、行番号はファイル全体の行番号
- nested tag展開: `#a/b/c` → `#a`, `#a/b`, `#a/b/c`
- code fence内のtagは無視
- 複数ファイルの同一tagは同じtagノードを共有
- inline tag Unicode: `#あいうえお`, `#my-tag` → 認識される
- inline tag 先頭数字: `#123` → 認識されない
- inline tag ネスト Unicode: `#parent/子タグ` → 展開動作
- inline tag 終端: ピリオド・General Punctuation で終端
- 統合テスト: note数・phantom数・tag数・edge数の検証
- 冪等性: 2回buildで結果が同一
- build除外: `build.exclude_paths` に一致するファイルがインデックスから除外される
- build除外: 除外ファイルへのパスリンクが phantom になる
- build除外: 除外ファイルへの basename リンクが phantom になる
- build除外: basename 重複の片方が除外されて曖昧リンクが解消される
- build除外: 除外ファイル内のタグがインデックスに入らない
- build除外: mdhop.yaml なしで Build が正常動作
- build除外: 空の exclude_paths で全ファイルがインデックスされる
- build除外: `[` を含むパターンでエラー

## resolve

- `[[Note]]` の解決（単一候補）
- `[[Note#Heading]]` / `[[Note#^block]]` の subpath
- `[[path/to/Note]]` の解決（Vault相対）
- `[[./Note]]` / `[[../Note]]` の解決
- `[text](note.md)` は `[[note]]` と同一扱い（basename解決）
- `[text](./note.md)` / `[text](../note.md)` は相対パス解決
- `[text](/note.md)` はVault相対
- source に実在しないリンクはエラー
- 曖昧候補はエラー
- ルート優先: basename重複でもルートファイルに解決

## query

- 起点: `--file/--tag/--phantom/--name` のいずれか
- `--name #tag` はタグ扱い、曖昧ならエラー
- `--name` のルート優先: basename重複でもルートファイルに解決
- `backlinks/tags/outgoing/twohop` の出力
- `twohop` は via→targets 構造
- `backlinks/outgoing/twohop` は type を含める
- `--fields` による出力制限（未知値はエラー）
- `--format text/json` の出力差
- `--include-head/--include-snippet` の出力
- stale（mtime不一致）検出でエラー
- `max-*` の上限適用
- `--exclude` でパス除外: backlinks/outgoing/twohop/snippet から除外パスが消える
- `--exclude` 複数パス除外
- `--exclude-tag` でタグ除外: tags から消える、twohop の via から消える
- `--exclude` でパス除外時に twohop の via/targets 両方から消える
- `--exclude` 時に phantom が消えないこと（NULL 安全性）
- エントリノード自体は除外されない
- `--no-exclude` で config の除外を無視
- CLI `--exclude`/`--exclude-tag` と config の除外がマージされる
- nil exclude（除外なし）で全結果が返る（回帰ガード）
- config ファイルなし → ゼロ Config
- config YAML 不正 → エラー
- glob パターンに `[` → エラー

## add

- 未登録の新規追加は成功
- 既存パス指定はエラー
- 追加ファイル内に曖昧リンクが含まれる場合はエラー
- `--format` バリデーション（無効値でエラー）
- デフォルト（auto-disambiguate ON）: 既存リンクの書換えが行われる
- auto-disambiguate ON でも、追加ファイルに曖昧リンクが含まれる場合はエラー
- `--no-auto-disambiguate`: 衝突時にエラー
- ルート優先: 旧一意先がルート → auto-disambiguate スキップ
- ルート優先: 追加ファイルにルートファイルあり → basename リンク非曖昧
- ルート優先: phantom promotion でルートファイルを優先

## update

- `--format` バリデーション（無効値でエラー）
- 登録済みのみ更新可能（未登録ファイルはエラー、DB変更なし）
- コンテンツ変更: 旧edges消滅、新edges生成、mtime更新
- ディスク不在+参照あり → phantom変換
- ディスク不在+参照なし → 完全削除
- ディスク不在+参照あり+同名phantom既存 → edges付替え+note削除
- 曖昧リンクを追加した更新はエラー（DB変更なし）
- ルート優先: basename重複でもルートファイルがあればエラーにならない
- vault外への相対リンク → エラー（DB変更なし）
- orphan cleanup: タグ参照がなくなったら削除
- mtime: 更新後 mtime が os.Stat と一致
- 複数ファイル同時更新
- 未解決リンク追加 → phantom生成
- 新タグ追加 → tag ノード+edge生成
- DB未作成 → エラー
- 複数ファイル指定で一部が未登録 → エラー、DB変更なし
- basename衝突2→1（片方削除で解決可能に）の遷移
- 更新時に非対象ファイルからの incoming edges が保持されること
- 同一updateで「削除対象を参照するファイル」も更新 → リンクはphantomへ向く

## move

- `--format` バリデーション（無効値でエラー）
- 旧パス未登録はエラー
- 新パス既存はエラー
- コラテラル書き換え: 移動で曖昧になる第三者 basename リンクはフルパスに自動書き換え
- コラテラル書き換え: 1ファイルに incoming + collateral が共存する場合
- コラテラル書き換え: 複数ファイルにコラテラル書き換え
- コラテラル書き換え: 対象ファイルが stale でも成功する
- outgoing basename 書き換え: 移動ファイルの basename リンクが曖昧化 → フルパスに書き換え
- outgoing basename 書き換え: ルート優先で意味が変わる場合も書き換え
- ルート優先: move前後でルートファイルが存続 → コラテラル書き換え不要
- ルート優先: basename不変 + ルート存続 → incoming rewrite スキップ
- `[[a]]` / `[x](a.md)` は一意なら維持、曖昧化/別解決なら書換え
- `[[path/to/a]]` / `[x](path/to/a.md)` は必ず書換え
- リンク書換え対象ファイルはDBから抽出
- ディレクトリ move: 基本（複数ファイル一括移動、ディスク・DB・edge 検証）
- ディレクトリ move: 空ディレクトリ → エラー
- ディレクトリ move: 移動先に既存ファイル → エラー
- ディレクトリ move: incoming rewrite（外部ファイルからのリンク書き換え）
- ディレクトリ move: collateral（basename 不変のため発生しないことの確認）
- ディレクトリ move: 複数 basename（count 不変、rewrite 不要の確認）
- ディレクトリ move: ルート優先（移動先 basename がルートに存在 → collateral スキップ）
- ディレクトリ move: outgoing basename（移動セット内ファイルへの basename リンク保持）
- ディレクトリ move: outgoing path（移動セット内ファイルへのパスリンク書き換え）
- ディレクトリ move: 移動セット内相対リンク保持
- ディレクトリ move: 外部への相対リンク書き換え
- ディレクトリ move: 外部へのパスリンク（変更なし）
- ディレクトリ move: 外部ファイル stale でも成功する
- ディレクトリ move: already-moved シナリオ
- ディレクトリ move: stale ファイル → エラー
- ディレクトリ move: ネストディレクトリ（`sub/inner/X.md` → `newdir/inner/X.md`）
- ディレクトリ move: overlap（`--from sub --to sub/inner`）→ エラー
- ディレクトリ move: vault escape → エラー
- ディレクトリ move: 移動先ファイルがディスクに存在 → エラー
- ディレクトリ move: phantom promotion
- ディレクトリ move: 登録済み note と asset を一緒に移動し、両方の DB path を更新する
- ディレクトリ move: 未登録の非 Markdown ファイルも移動する。隠しファイル・隠しディレクトリは disk-only 移動の対象外

## delete

- 登録済みのみ削除可能
- 参照あり: phantom化
- 参照なし: ノード削除
- `--format` バリデーション（無効値でエラー）
- `--rm`: ファイル存在時にディスクから削除+インデックス更新
- `--rm`: ファイル不在時も成功（冪等）
- `--rm`: 未登録ファイルはエラー、ディスク上のファイルは削除されない
- `--rm`: 参照ありファイルはディスク削除+phantom化
- ディレクトリ展開: 登録済み note と asset の両方を取得する（基本、ネスト、マッチなし、特殊文字）
- ディレクトリ delete: 登録済み note と asset を削除対象に含める。`--rm` ありではディスクからも削除し、なしではインデックスのみ更新する
- ディレクトリ delete: DB にファイルなし → エラー
- ディレクトリ delete: `--rm` なし（ファイル既削除済み）でインデックスのみ更新
- 空ディレクトリ掃除: `CleanupEmptyDirs` で空ディレクトリが再帰削除される
- 空ディレクトリ掃除: 非 `.md` ファイルが残れば停止（ENOTEMPTY）
- 空ディレクトリ掃除: vault root で停止
- ディレクトリ delete `--rm`: 登録済みファイルと DB 更新の完了後に残存 asset の削除または空ディレクトリ掃除が失敗した場合、成功出力を出さず、完了済み更新と失敗 path・原因を含むエラーを返す

## diagnose

- basename衝突一覧
- phantom一覧

## stats

- notes_total / notes_exists / edges_total / tags_total / phantoms_total

## repair

- 正常系: 壊れたパスリンク（候補 0-1 個）が basename リンクに書き換わる
- 正常系: subpath（`#Heading`）が保持される
- 正常系: wikilink と markdown link の両方が書き換わる
- dry-run: 出力は同じだがディスク変更なし
- 壊れたリンクなし → 空結果
- skipped: 候補 2+ 個のパスリンクは skipped に正しい候補パスが含まれる
- basename リンクは修復対象外
- インラインコード内のリンクは変更されない
- `build.exclude_paths` で除外されたファイルへのパスリンクは書き換えない
- vault-escape リンク（relative）が basename に書き換わる
- vault-escape リンク（absolute path）が basename に書き換わる
- vault-escape リンク + 候補 2+ → 候補数に関係なく basename 化（escape 解消優先）
- 壊れたパスリンクと escape リンクの混在
- 非 `.md` 拡張子の壊れたパスリンクが basename に書き換わる
- ドット入り basename（`Note.v1`）が正しく書き換わる（拡張子誤除去なし）

## simplify

- 正常系: unique note のパスリンク（wikilink/markdown）が basename リンクに短縮される
- 正常系: unique asset のパスリンクが basename リンクに短縮される
- skipped: ambiguous note（2+ 候補、ルート優先なし）は skipped に候補パス付きで報告
- skipped: ambiguous asset（2+ 候補）は skipped に候補パス付きで報告
- 相対パス: `[[../sub/B]]`, `[[./E]]` が正しく解決・短縮される
- dry-run: 出力は同じだがディスク変更なし
- basename リンクは対象外（既に短い形式）
- インラインコード内のリンクは変更されない
- subpath（`#Heading`）が保持される
- alias（`|alias`）が保持される
- markdown fragment（`#section`）が保持される
- self-link（`[[#Heading]]`）はスキップ
- ルート優先（note）: root file 指すリンクのみ simplify、非 root は変更しない
- ルート優先（asset）: root asset がある場合の挙動
- 壊れたパスリンク（存在しないファイル）はスキップ
- vault-escape リンクはスキップ
- `--file` で対象ファイルを制限
- `build.exclude_paths` に従い除外ファイルはスキャンされない
- markdown link の .md 拡張子有無が保持される
- tag/frontmatter リンクは対象外
- asset-note namespace 衝突: 同名 note 存在時に asset パスリンクは短縮しない
- `--file` に存在しないファイルを指定 → エラー

## convert

- markdown → wikilink: 基本変換、subpath 保持、alias 判定
- wikilink → markdown: 基本変換、subpath 保持、alias 展開
- URL リンクは変換対象外
- tag / frontmatter は変換対象外
- code fence 内のリンクは変更されない
- inline code 内のリンクは変更されない
- dry-run: ディスク変更なし
- 変換対象なし → 空結果
- mixed: 同一ファイルに両タイプ → 対象のみ変換
- `--file` でスコープ限定
- 除外ファイルを `--file` に指定 → エラー
- asset リンク（非 .md）の双方向変換
- self-link（`[[#H]]` ↔ `[#H](#H)`）変換
- `build.exclude_paths` に従う
- ドット付き basename（`Note.v1`）が note として認識される
- 相対パス（`./`, `../`）プレフィックス保持
- ラウンドトリップ: 双方向変換で元に戻る
- embed プレフィックス（`!`）が正しく保持される

## disambiguate

- `--format` バリデーション（無効値でエラー）
- `--name` 必須
- 候補複数時は `--target` 必須
- `--file` 指定時は対象ファイルのみ書換え
- `[[a]]` / `[x](a.md)` を対象
- `[[path/to/a]]` 等のパス指定リンクは既に明確なため対象外
- phantom を指すパスリンクも `--name` の対象に含まれる
- `--scan` でも壊れたパスリンクが対象になる
### `--scan`（DB なし走査モード）

- DB なしで basename リンクがフルパスに書き換わる
- 候補複数時は `--target` 必須
- `--target` 指定で正しく動作
- `--file` 制限が効く
- 存在しないファイルを `--file` に指定でエラー
- インラインコード内はスキップ
- `.mdhop/` ディレクトリが存在しなくても動作する
- `--name` の大小文字不問
- パス指定リンクは対象外
- `--target` 不一致時のエラー
- ルート優先: ルートファイルが target → rewrite 結果が変化なし（0件）
- `--scan` が `build.exclude_paths` に従う（除外ファイルが候補・走査対象にならない）

## set

- `--file` と `--key`、および `--value` / `--date` のちょうど一方が必須。空値・不正な相対日付はエラー
- 1 回の実行で 1 ファイルの 1 scalar key だけを更新し、`--value` は相対日付展開せず YAML 値として書く。`--date` は相対日付を `YYYY-MM-DD` に展開する
- frontmatter がなければ先頭に作成し、key がなければ閉じ `---` の直前に追加する。結果は file/key/value/created を JSON/text で返す
- 未登録・ディスク不在・stale な対象はエラー。対象 key の list、複数行値、重複 key も拒否する
- 成功時はファイルと index を更新する。index 更新失敗時は本文・permission・mtime を復元し、復元失敗もエラーで報告する

## search

- 起点不要で、実在する note のみを検索する
- `--where` 複数指定は AND、式内の `||` は OR。相対日付も条件に使える
- `--path` は包含（複数指定は OR）、`--exclude` と config の除外を適用し、`--no-exclude` で config 除外だけを無効化する。`--no-tags` / `--no-outgoing` / `--no-incoming` で孤立条件を絞れる
- meta key / computed field の昇降順 sort、limit/offset のページングを適用し、total はページング前の件数を返す
- `--sample` はフィルタ後・ページング前から無作為に返し、sort/limit/offset と併用不可。`--count` は件数だけを JSON/text で返し、fields/head/sample/sort/ページングと併用不可
- `--fields` と `--include-head` は追加出力。空結果は JSON で total=0 / items=[]、text で `total: 0` を返す
- unknown field、負の `--limit` / `--offset`、明示指定した `--sample <= 0`、不正 glob はエラー

## reachable

- 実在する note の `--from` が必須。未登録 path / asset / 不正 glob はエラー
- navigation link（wikilink / markdown / frontmatter link）だけを辿り、tag edge は到達性に数えない
- `--path` / `--exclude` で対象 note 集合を絞り、reachable と unreachable に分ける。起点自身は対象内なら 0 hop で reachable
- 対象外 note は結果に含めないが、到達経路の中継には使う。`--route` は最短 route を追加する
- `--fields`、JSON/text 出力を確認する。path/exclude は CLI 引数だけであり config の除外は適用しない

## graph

- 実在する note / asset を node 集合とし、両端が集合内の link 出現ごとに edge を返す誘導部分グラフ
- tag node / edge は出力せず、`--include-phantoms` で集合内 note が参照する phantom だけを追加できる
- 既定 JSON と DOT を出力し、text format と `--fields` は非対応。node ID は出力内で edge の source / target を参照する
- `--path` / `--exclude`、空結果、不正 glob / format を確認する。ID の build 間安定性は要求しない
- path/exclude は CLI 引数だけであり config の除外は適用しない

## meta-check

- `--key` は必須かつ複数指定可。`--kind path|wikilink|auto`（既定 path）で値を解釈する
- scalar / list を値単位で検査し、空値と URL は skip する。path の末尾 `/` は実在ディレクトリなら有効
- 未解決値を issues として JSON/text に返し、reason は not_found / ambiguous / vault_escape / not_wikilink。issues の検出自体はコマンドエラーとしない
- `--path` / `--exclude` は source note を glob で絞り、config の除外は適用しない。`--key` 未指定、無効な kind、不正 glob はエラー

## meta-validate

- `--require` は非空 key を要求し、欠落・空値は missing。複数指定できる
- meta.profiles は path 条件別の required key を定め、CLI の `--require` はその実行時だけ profile の必須 key を置き換える
- 非 string の meta.types は type / enum を検査し、`--require` の有無にかかわらず継続する。require / profile / 非 string type がなければエラー
- `--path` / `--exclude` は source note を glob で絞り、config の除外は適用しない。結果は violations として JSON/text に返し、違反の検出自体はコマンドエラーとしない
- 現在の meta 設定で build された DB を前提にする

## init-meta

- `--preset` / `--scan` の少なくとも一方が必須で、DB なしで実行できる
- scan は frontmatter から型を推定し、tags / aliases は対象外。preset と併用時は scan 結果を優先する
- 既定では YAML を stdout に出しファイルを変更しない。`--no-comment` はコメントを省く
- `--write` は mdhop.yaml を更新し、既存の設定と既存 meta.types を保持する。追加・skip は stderr に通知する
