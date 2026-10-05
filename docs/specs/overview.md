# 外部仕様（統合版）

このドキュメントは、ユーザー視点の挙動・互換性・制約・非目標をまとめる。
実装詳細や内部構造は書かず、詳細はテストとコードに寄せる。
必要最小限の説明に留める。
詳細な内部設計は `docs/rules/` と `docs/decisions/` に置く。
このドキュメントがコマンド仕様の正本であり、CLI ヘルプ（`cmd/mdhop/*.go` の help 文字列）はその要約（従）で、詳細はここを参照する位置づけとする。

## 対象と前提

- 対象は Obsidian Vault 相当のディレクトリ配下の `**/*.md`（note）と非 `.md` ファイル（asset）
- asset: 画像・PDF 等の非 `.md` ファイル。build 時に全走査して DB に登録する
  - 隠しファイル・隠しディレクトリ（`.` 始まり）は asset 走査の対象外
  - `.mdhop/` ディレクトリは走査対象外
  - asset は source にならない（outgoing edge を持たない）。edge を生成するのは note のみ
  - asset の DB 登録は build 時のみ行う。build 後にディスクへ追加された asset は次の build まで phantom として扱われる
- 主な利用者は CLI と Coding Agent
- ローカル完結（SQLite）で動作する
- fenced code block は 3 個以上の backtick または tilde で開始し、同種かつ開始時以上の連続数で、末尾が空白のみの行で閉じる。内部の link、tag、heading は解析しない。

## データ配置と設定

- Vault 直下に `.mdhop/` を作成し、SQLite などの実行時データを配置する
- `.mdhop/` の主なファイルは `index.sqlite` とする
- 将来的に `.mdhop/meta.json` を置く場合は、スキーマバージョンやインデックス作成情報を保持する
- 設定ファイル: Vault 直下の `mdhop.yaml`（YAML 形式）
  - ファイルがなければデフォルト設定（除外なし）で動作する
  - `build` セクション: build 時のファイル除外
  - `exclude` セクション: search の除外。query では `query.via.exclude` キー不在時だけ経由先除外に流用
  - `query.hide` / `query.via` セクション: 関連対象の表示と twohop 経由先の選択（下記参照）

```yaml
build:
  exclude_paths:
    - "daily/*"
    - "templates/*"

exclude:
  paths:
    - "daily/*"
    - "templates/*"
  tags:
    - "#daily"
    - "#template"

meta:
  types:
    date: date           # スカラー形式
    priority: number     # スカラー形式
    version: semver      # スカラー形式
    status:              # ordered 形式
      ordered: [backlog, todo, doing, done]
  link_keys:             # raw path 値を graph edge にする frontmatter key
    - related
    - sources
```

## コマンドと挙動（厳密モード前提）

サブコマンドは位置引数を受け取らない。flag 解析後に引数が残る場合（`--` 後を含む）は、処理開始前に引数 error を返す。先に解析される help や不正 flag は、その解析結果を返す。

- `mdhop build` : Vault 全体を解析しインデックスを作成する
- `mdhop update --file ...` : 登録済みファイルのみを更新する
  - `--file` は複数回指定できる
- `mdhop set --file A.md --key reviewed --value 2026-07-03` : frontmatter の単一キーを書き換え、インデックスを更新する
- `mdhop set --file A.md --key reviewed --date today-90d` : 相対日付を `YYYY-MM-DD` に展開して frontmatter に書き込む
- `mdhop set --file A.md --key aliases --list '["a","b"]'` : JSON string array を frontmatter の sequence として書き換え、インデックスを更新する
- `mdhop add --file ...` : 新規追加を反映する（未登録のみ）
- `mdhop move --from A.md --to B.md` : ファイル移動を反映する（note / asset 両対応）
- `mdhop move --from A.md --to-template "99-Archive/{client|others}/{updated:year}/{basename}"` : note の frontmatter 値から移動先を展開して移動する
- `mdhop move --from dir/ --to-template "99-Archive/{client|others}/{updated:year}/{basename}" --dry-run` : ディレクトリ配下 note のテンプレート移動計画を表示する
- `mdhop move --from dir/ --to newdir/` : ディレクトリ単位の移動を反映する
- `mdhop delete --file ...` : ファイル削除を反映する（note / asset 両対応、登録済みのみ）
- `mdhop delete --file dir/` : ディレクトリ配下の全登録済みファイル（note + asset）を削除する
- `mdhop disambiguate --name a` : 曖昧リンクをフルパスへ書き換える
- `mdhop simplify` : 冗長なパスリンクを basename リンクに短縮する
- `mdhop repair` : 壊れたパスリンクと vault-escape リンクを basename リンクに書き換える
- `mdhop convert --to wikilink|markdown` : wikilink と markdown link を相互変換する
- `mdhop resolve --from A.md --link '[[X]]'` : リンク解決を行う
- `mdhop query --file A.md` : 起点ノートの関連情報を返す
- `mdhop query --tag tag` : タグ起点の関連情報を返す
- `mdhop query --phantom name` : phantom 起点の関連情報を返す
- `mdhop query --name name` : note/phantom/tag を意識せず関連情報を返す
- `mdhop inspect --file A.md` : 索引済み note 一件の tags / meta と任意の本文冒頭を返す
- `mdhop search --where "status=active"` : ノートをメタデータ条件で検索する
- `mdhop diagnose` : basename 衝突、phantom 一覧を検出する
- `mdhop status` : ディスクと現在の索引を比較し、未登録・変更済み・削除済みの note / asset を一覧にする。索引・ファイルは変更しない
- `mdhop meta-check --key sources` : frontmatter の指定キーが参照するパスを検査する
- `mdhop meta-validate --require status` : frontmatter の必須キーと型を検証する
- `mdhop reachable --from A.md --path "docs/*"` : 入口 note からリンクで到達できる / できない note を列挙する
- `mdhop graph --path "docs/*"` : リンクグラフを誘導部分グラフとして JSON / Graphviz dot で出力する
- `mdhop stats` : ノート数・リンク数などの統計情報を返す
- `mdhop init-meta --preset` : frontmatter 型定義の scaffold を生成する

### 書き換え対象の境界

- `set` とリンク書き換え系（add の自動 rewrite、move、disambiguate、simplify、repair、convert）は、書き込み候補の実体が symlink 経由で vault 外を指す場合、書き込み前にエラーにする。末尾ファイルと祖先ディレクトリの symlink が対象。候補が複数ある場合も外部候補を含む操作では部分更新しない。
- vault 内を指す symlink と、symlink として指定された vault root は利用できる。復元でも vault 外への書き込みを拒否し、復元できない場合は rollback failure を報告する。意図的な並行 symlink 差替えと hardlink はこの保証の対象外。

### モード

- 既定は **厳密モード**（曖昧時はエラー）
- 互換モード（Obsidian互換）は将来追加する

### 厳密モードの曖昧リンクルール

- **曖昧リンク**: basename 解決が必要で、複数候補があるリンク。
- 厳密モードでは、**曖昧リンクの存在を禁止**する。
  - `[[a]]` と `[x](a.md)` は同一扱い。候補が複数なら曖昧。
  - basename 衝突（同名ノートの複数存在）は**それ自体ではエラーにしない**。
  - ただし、曖昧リンクが残る場合はエラー。
  - **ルート優先例外**: basename 重複時でもルート直下にそのファイルがあれば `[[basename]]` はルートファイルに解決（曖昧ではない）。
  - `--include-head` / `--include-snippet` で stale（mtime 不一致）が検出された場合はエラー。

### 共通オプション

- `--vault <path>` : Vault ルートを指定（省略時はカレントディレクトリ）

### status の出力

- `--format json|text` : 出力形式を指定する（default: text）
- JSON は常に `untracked`、`modified`、`deleted` の path 配列を持つ。path は Vault 相対・forward slash・NFC、各配列は辞書順
- `untracked` は現在の build 対象（note / asset、`build.exclude_paths` 適用後）で索引にないファイル。query の hide / via 条件と search の `exclude` は適用しない
- `modified` は登録済み note / asset のディスク上の mtime が索引の mtime と秒精度で異なるファイル。本文 hash・サイズ・同一秒内の変更は検査しない
- `deleted` は登録済み note / asset がディスク上にないファイル。現在の build 除外に一致する登録済みファイルも比較対象とする
- status は差分の有無で失敗せず、索引・vault・設定を更新しない

### resolve/diagnose/stats/reachable の出力

- `--format json|text` : 出力形式を指定する（default: text）
- `--fields <comma-separated>` : 出力フィールドを制限する
  - resolve: `type,name,path,exists,subpath`
  - diagnose: `basename_conflicts,asset_basename_conflicts,phantoms`
  - stats: `notes_total,notes_exists,edges_total,tags_total,phantoms_total,assets_total`
    - `edges_total` は出現回数ベースの総数
  - reachable: `reachable,unreachable`
    - `from`（正規化済み entry path）は `--fields` に関係なく JSON に常に含まれる。`routes` は `--route` 指定時のみ含まれる
- graph はこのグループに含まれない: `--format json|dot`（default: json）で、`text` と `--fields` は持たない（出力は機械処理・可視化ツール向けの全量）

### フィールド定義

#### resolve

- `type`: `note|phantom|tag|url|asset`
- `name`: 表示名（noteはbasename、tagは`#`付き、assetはファイル名）
- `path`: Vault相対パス（note/assetのみ）
- `exists`: note/assetの存在フラグ
- `subpath`: `#Heading` / `#^block`（あれば）

#### diagnose

- `basename_conflicts`: note の basename 衝突一覧
- `asset_basename_conflicts`: asset の basename 衝突一覧
- `phantoms`: phantom 名一覧

#### stats

- `notes_total`: note総数
- `notes_exists`: exists=true の note 数
- `edges_total`: edges総数（出現回数ベース）
- `tags_total`: tag総数
- `phantoms_total`: phantom総数
- `assets_total`: asset総数

### query の CLI 契約

入口指定の `--file` / `--tag` / `--phantom` / `--name`、`--vault`、`--format text|json` は維持する。入口は従来同様一つだけ指定する。

| 引数 | 確定する意味 |
| --- | --- |
| `--relations backlinks,outgoing,twohop` | 省略時は三関係すべて。指定時は列挙した関係のみ。順番は出力欄の順番を変えない。重複、未知名、空要素はエラー。 |
| `--limit N` / `--offset N` | **関係を一つだけ明示選択したときのみ**指定可能。limit 省略は全件、offset 省略は 0。`N>0` / `N>=0` の整数。offset 単独なら残り全件。複数関係や関係指定省略との併用はエラー。専用 `--all`、`--count` は設けない。 |
| `--via type:value` | 経由先を型と識別子で一つ選ぶ。型は `note` / `asset` / `phantom` / `tag`。note / asset は Vault 相対パス、phantom は名前、tag は `#` の有無を許すタグ名。指定型で正規化後完全一致。複数指定、未知型、空値はエラー。指定先が未登録・型違い・入口から未参照なら twohop は空で、他の via への fallback や入口解決由来のエラーにはしない。直接関係は制限しない。 |
| `--via-path GLOB` / `--via-tag TAG` | 経由先の包含条件。path は note / asset、tag は tag 型に適用する。各引数は繰返し可能で、設定値とも OR、path と tag も OR。条件なしならすべて。`--via` と併用時はその一件との積集合。値はカンマ分割しない。 |
| `--exclude-via-path GLOB` / `--exclude-via-tag TAG` | 繰返し可能な経由先の除外条件。path は note / asset、tag は tag 型に適用し、包含条件より優先する。ここで外した経由先は関係を生成しない。値はカンマ分割しない。 |
| `--hide-path GLOB` / `--hide-tag TAG` | 繰返し可能な表示上の除外。note 対象・note 経由先には path、tag 対象・tag 経由先には tag を適用する。隠した経由先から対象を発見する関係自体は残す。asset / phantom は hide-path の対象外。入口オブジェクトは隠さない。値はカンマ分割しない。 |
| `--no-config-hide` / `--no-config-via` | その目的の設定値だけを無視する。CLI 指定値は残る。`--no-config-via` は旧 exclude の fallback も無視する。 |
| `--path GLOB` / `--where EXPR` | 従来同様、返す対象だけを絞る。経由先には掛けない。path を持たない tag / phantom は `--path` の対象外で残す。`--where` 指定時、tag / asset / phantom は一致しない。note の欠損属性は `NOT EXISTS` 等の既存評価に従う。 |
| `--link-key KEY` | 従来同様、backlinks / outgoing の対象リンク出現だけを絞る。twohop の入口→via と対象→via の両側に適用しない。outgoing のタグも該当する直接リンク出現でなければ落ちる。 |
| `--include-head N` | 返した関連 note の本文冒頭 N 行を付ける。`N>0`。省略時は head を取得しない。 |
| `--include-snippet N` | 返した関係の根拠リンク行と前後 N 行を付ける。`N>=0`、0 はリンク行のみ。省略時は snippet を取得しない。値 0 と引数省略を区別する。 |

例:

```sh
mdhop query --file Plan.md --format json
mdhop query --file Plan.md --relations twohop --via note:topics/設計方針.md --format json
mdhop query --file Plan.md --relations backlinks --limit 20 --offset 20 --format json
mdhop query --file Plan.md --hide-path 'archive/*' --exclude-via-tag '#private' --format text
```

`--relations` は関連の種類、`--via*` は twohop の手がかり、`--hide*` は出力上の見せ方を指定する。`--via` で一つ選んでも backlinks / outgoing は消えない。twohop だけ欲しい場合は `--relations twohop` も指定する。

旧 query の `--fields`、`--max-backlinks`、`--max-twohop`、`--max-via-per-target`、`--exclude`、`--exclude-tag`、`--no-exclude` は削除し、移行 alias を設けない。search の同名引数と動作は変更しない。旧上限は一対一の置換ではなく、必要な関係を一つ選んで `--limit` / `--offset` を使う。

### inspect の CLI 契約

`mdhop inspect --file Plan.md [--fields tags,meta] [--include-head N] [--format text|json] [--vault PATH]` は、索引上に存在する note 一件のタグと frontmatter 属性を取得し、指定時に本文冒頭を付ける。索引が無い場合は既存の読み取りコマンドと同じエラーにし、自動 build はしない。asset、phantom、索引に存在しない note はエラー。入口の関連探索はしない。既定の format は query と同じ text、vault は `.` とする。

`--fields` 省略時は `tags,meta`。明示指定時は指定した属性だけで、重複・未知名・空要素はエラー。`--include-head N` は属性選択から独立した preview 指定で、`N>0`。head だけの属性モードは設けない。空の tags は `[]`、meta は `{}` で返し、指定した欄の有無が値の空と混同しない。tags は葉タグだけを表示するため、query outgoing に含まれる索引済み親タグと異なることがある。入力文字列のタグ階層を再構成する機能は加えない。

meta は索引済みの属性だけを既存の `map[string][]string` 形式で返し、索引対象・値の意味は現行設定に従う。frontmatter 生 YAML の全属性取得や新しいメタデータ型は導入しない。JSON の `entry` は query と同じ typed NodeInfo、選択した `tags` / `meta` および指定した `head` のみを付ける。

inspect は明示した一件の情報を返すため、query の hide / via 条件や旧 exclude の表示除外は適用しない。head 未指定なら本文の鮮度確認のためにファイルを読まず、索引上の tags / meta を返す。head 指定時のファイル欠落・更新検知は下記の本文読取契約に従う。

```sh
mdhop inspect --file Plan.md --format json
mdhop inspect --file Plan.md --fields tags --include-head 5 --format text
```

たとえば `Plan.md` に `#planning`、索引済み `status: draft`、frontmatter 後の本文 `# 計画` がある場合:

```json
{"entry":{"type":"note","name":"Plan","path":"Plan.md","exists":true},"tags":["#planning"],"meta":{"status":["draft"]},"head":["# 計画"]}
```

これは `mdhop inspect --file Plan.md --include-head 1 --format json` の例。同じ入力の text は次の形で、選択した欄だけ出す。

```text
entry:
  note "Plan.md"
tags:
  tag "#planning"
meta:
  "status": "draft"
head:
  "# 計画"
```

### query の対象メタデータ条件

- `--where <expr>` : frontmatter メタデータによるフィルタ（複数回指定可）
  - 演算子: `=`, `!=`, `~`（LIKE）, `>`, `<`, `>=`, `<=`, EXISTS（演算子なし）, NOT EXISTS
    - 比較演算子が値の中にも現れる場合は、式の左端にある演算子でキーと値を分ける（例: `title=a!=b` はキー `title`、値 `a!=b` の等値比較）
    - `~` の右辺（LIKE パターン）は前後の空白を保持する。トリムされるのはキー名のみ
  - 例: `--where "status=active"`, `--where "priority>1"`, `--where "status"`, `--where "status!=done"`, `--where "priority NOT EXISTS"`
  - 左辺には `coalesce(key1, key2, ...)` を書ける。比較演算子では、左から順に最初に存在するキーの値を使って比較する
    - 例: `--where "coalesce(reviewed, updated)<=today-1y"` → reviewed があれば reviewed、なければ updated を基準に古い note を探す
    - `coalesce(reviewed, updated)` は reviewed または updated のどちらかが存在する note、`coalesce(reviewed, updated) NOT EXISTS` はどちらも存在しない note にマッチする
    - 型安全な比較は各キー自身の `meta.types` 宣言を使う（reviewed と updated で宣言型が異なっていても、それぞれ自分の型でガードされる）。ただし相対日付（`today` 等）を使った場合は全キー共通で date 型として扱う
  - 結合ルール:
    - 複数 `--where` フラグ → 常に AND（キーの一致・不一致を問わず、全フラグの条件にマッチ）
    - 1 つの `--where` 内で ` && ` 区切り → AND（同一キーでも AND。日付範囲等に使用）
    - 1 つの `--where` 内で ` || ` 区切り → OR（異なるキーでも OR）
    - 1 つの `--where` 内で ` && ` と ` || ` を混在させた場合は **エラー**（優先順位・括弧解釈は未対応）
    - 例: `--where "status=active" --where "priority>1"` → status が active かつ priority が 1 より大きい
    - 例: `--where "status=active" --where "status=review"` → status が active かつ review（スカラー値では通常マッチしない）
    - 例: `--where "created>=2025-02-01 && created<=2025-02-28"` → created が 2 月の範囲内
    - 例: `--where "status=active || status=review"` → status が active または review
    - ` && ` / ` || ` は前後スペース必須（スペースなしの `&&` / `||` は区切りとみなされない）
  - フィルタ対象: backlinks, outgoing, twohop の結果ノード（エントリノード自体はフィルタされない）
  - phantom/tag/asset は meta テーブルにエントリを持たないため、`--where` 指定時に常にフィルタアウトされる
  - `mdhop.yaml` の `meta.types` で型宣言されたキーは比較演算子で型安全な比較が可能
    - number は長い小数も丸めずに比較し、末尾の 0 の有無は等値・並び順に影響しない。正規化形式と既存 index の再生成条件は `docs/rules/03-data-model.md` に従う
  - 相対日付: 比較演算子の右辺に `today` / `today-90d` / `today+1d` / `today-2w` / `today-3m` / `today-1y` を書ける（単位 `d`=日, `w`=週, `m`=月, `y`=年）。実行時のローカル日付を基準に絶対日付へ展開する
    - 例: `--where "updated<today-90d"` → 90 日以上更新されていない note
    - 相対日付は date として比較する。比較は保存済み `sort_value` に対し `value_type='date'` ガード付きで行うため、左辺キーは `meta.types` で `date` 宣言されている必要がある。未宣言キーは `value_type='string'`・文字列正規化された `sort_value` で保存されており、ガードに弾かれてマッチしない

### query 設定と照合規則

```yaml
exclude:                         # 既存。search では従来どおり使う
  paths: ["archive/*"]
  tags: ["#private"]
query:
  hide:
    paths: ["archive/*"]
    tags: ["#private"]
  via:
    include:
      paths: ["topics/*"]
      tags: ["#planning"]
    exclude:
      paths: ["topics/secret.md"]
      tags: ["#private"]
```

各目的内で config と CLI を足す。path 条件、tag 条件、それぞれの複数値は OR。via の包含と除外は独立し、除外が優先する。`query.via.exclude` **キー自体が無い場合だけ**旧トップレベル `exclude` を via 除外に流用する。空の `query.via.exclude: {paths: [], tags: []}` は明示的な上書きとして扱う。`query.hide` へ旧 exclude は流用しない。`query.via.include` の有無は fallback の判定に影響しない。

GLOB は Vault 相対、大小文字を区別する。`*` は `/` をまたぎ、`?` は一文字、`[]` 文字クラスは非対応としてエラー。`archive/*` は配下を再帰的に照合し、`archive/` 単独を配下指定とみなさない。CLI では `'*'` をシェル展開されないよう引用する。型付きの完全一致 `--via` は GLOB ではない。tag は先頭 `#` を揃えて大文字小文字を区別せず完全一致で比較する。

### search の除外フィルタ

- 検索結果の node に `exclude.paths` / `exclude.tags` と CLI `--exclude` を適用する。config と CLI は合成し、`--no-exclude` は config だけを無視する。
- query の hide / via 設定は適用しない。glob / tag の照合規則は上記と共通。

### コマンド詳細（必須/任意）

- `build`
  - 必須: なし
  - 任意: `--vault`
  - 補足: 曖昧リンクが存在する場合は **エラー**（厳密モード）
  - 補足: 並行 build はそれぞれ専有の一時 DB を完成させてから公開し、最後に成功した置換が index として残る。失敗した build は他実行の一時 DB や公開済み index を変更しない。
  - 補足: `mdhop.yaml` の `build.exclude_paths` に一致するファイルはインデックスから除外される
    - 除外ファイルへのリンクは phantom ノードとして扱われる
    - 除外ファイル内のタグはインデックスに含まれない
    - query の hide / via 条件とは独立（build 除外は索引作成前に適用）
    - mutation コマンド（`add` / `update` / `delete` / `move`）は `build.exclude_paths` を参照せず、DB 状態に対して動作する。不整合（例: `add --file daily/D.md`）は次の `build` で解消される
- `update`
  - 必須: `--file`（複数回指定可）
  - 任意: `--vault`, `--format`
  - 補足: 更新後の内容に、曖昧リンクが含まれる場合は **エラー**
    - 対象: `[[a]]` / `[x](a.md)` など basename 解決が必要なリンク
- `set`
  - 必須: `--file`, `--key`, および `--value` / `--date` / `--list` のちょうど一つ
  - 任意: `--vault`, `--format`
  - 補足: 1 コマンドで 1 ファイル 1 キーだけを書き換える。flow mapping の同一行の他キーと、引用が必要な YAML key の意味を保持する
  - 補足: frontmatter がないファイルは、ファイル先頭に frontmatter block を新規作成して対象キーを書き込む
  - 補足: 対象キーがない場合は、frontmatter の閉じ `---` の直前に新規キーを追加する（flow mapping では既存 mapping に追加する）
  - 補足: `--value` は相対日付展開せず、渡された値をそのまま YAML 値として書き込む
  - 補足: `--date` は `today` / `today-90d` / `today+1y` など search と同じ相対日付構文を `YYYY-MM-DD` に展開して書き込む
  - 補足: `--list` は JSON string array を受け取り、順序・重複・空文字列を保持して YAML sequence として書き込む。`[]` は空の YAML sequence を書き込む。JSON 出力の `value` は `--list` では string array（`[]` を含む）、`--value` / `--date` では string のまま。text 出力の list 値は compact JSON array 形式
  - 補足: `--value` / `--date` / `--list` の複数指定または全省略は **エラー**。`--value` と `--date` による scalar write は既存 sequence を拒否し、`--list` は既存 scalar・block sequence・flow sequence を置換する
  - 補足: scalar write は既存キーのリスト形式値を **エラー** とする
  - 補足: scalar write は既存キーの値が複数行にまたがる場合（折り返しプレーンスカラー、複数行 quoted scalar、block scalar）を **エラー** とする。値の後の空行・独立コメントは値の行数に含めず保持する
  - 補足: 対象キーが frontmatter 内に複数回出現する（重複キー）場合は **エラー**
- `add`
  - 必須: `--file`（複数回指定可）
  - 補足: 正規化後に vault 外へ出る相対 path は操作全体をエラーとし、ファイルの読込・登録・書き換えを行わない
  - 任意: `--vault`, `--format`, `--no-auto-disambiguate`
  - 補足: 既存ファイルが指定された場合はエラー
  - 補足: 追加ファイル内に曖昧リンクが含まれる場合は **エラー**
  - 補足: basename 衝突が発生する場合、既存リンクを自動でフルパス化する（意味を保てる場合のみ）。`--no-auto-disambiguate` で無効化
  - 補足: 既存の basename リンクが phantom を参照しており、追加ファイルが同じ basename を複数持つ場合は auto-disambiguate ON でも **エラー**（安全に書き換え先を決定できないため）
- `move`
  - 必須: `--from`, `--to` または `--to-template`
  - 任意: `--vault`, `--format`, `--dry-run`
  - `--dry-run`: `--to-template` 指定時のみ使用可。展開済み move plan を既存の move 出力形式で返す前に、実行時と同じリンク書き換え候補の対応検証を行う。ディスク・DB は変更しない（将来の I/O 成功は保証しない）
  - `--to-template`: 登録済み note の indexed frontmatter と source filename から移動先を展開する。`--to` とは同時指定不可。directory mode では配下の登録済み note 全件を展開し、全件の展開・destination 検証が成功してから batch move する
    - 構文:
      - `{field}`: source note の frontmatter key `field` の値
      - `{field|fallback}`: `field` が存在しない場合は fallback literal を使う
      - `{updated:year}` / `{updated:month}` / `{updated:day}`: date として正規化済み sort_value から年・月・日を使う
      - `{updated:year|2099}`: `updated` が存在しない場合は fallback literal を使う。`updated` が存在するが date として parse できない場合は fallback せずエラー
      - `{basename}`: source path のファイル名（例: `Alpha.v1.md`）
    - エラー:
      - fallback なしの missing field はエラー
      - 参照 field が複数値（YAML sequence 等）を持つ場合は、静かに 1 件を選ばずエラー
      - date partial extraction が date として parse できない場合はエラー
      - placeholder 展開値に `/` が含まれる場合はエラー（ディレクトリ区切りは template literal の `/` のみ）
      - 展開結果が空、絶対パス、vault 外へ escape するパス、または directory path になる場合はエラー
      - directory mode で 1 件でも展開・destination 検証に失敗した場合は全体を中止し、部分実行しない
    - 展開後の destination path を既存の `move` に渡すため、vault safety、上書き防止、stale 検出、リンク書き換え、frontmatter raw path guard は通常の `move` と同じ
  - 補足: ディスク上のファイル移動も行う（移動先ディレクトリは自動作成）
  - 補足: ディスク上で移動する全ファイルの移動先（未登録 asset を含む）の既存祖先を symlink 解決し、実体が vault 外へ到達する場合は、リンク書き換え・ディレクトリ作成・移動より前に全体を拒否する。未作成の多段ディレクトリ、vault 内 symlink、symlink の vault root は利用できる。`--to-template --dry-run` も同じ移動先検証を行う
  - 補足: `--from` がディスクになく `--to` がディスクにある場合、既に移動済みとみなしてリンク書き換え+DB更新のみ行う
  - 補足: `--to` がディスク上に既に存在する場合は **エラー**（上書き防止）
  - 補足: 移動に伴い、リンクは必要に応じて書き換える
    - `[[a]]` / `[x](a.md)` は、移動後も一意に同じノートを指すなら書き換えない
    - 曖昧になる／別ノートに解決される場合はフルパスに自動書き換え（第三者ファイルのリンクも対象）
    - 移動ファイル自身の outgoing basename リンクも、解決先が変わる場合はフルパスに書き換え
    - `[[path/to/a]]` / `[x](path/to/a.md)` などパス指定は必ず書き換える
    - Vault 相対パスへの統一原則に対する move の例外として、移動する note 自身に既にある `./` / `../` 起点の wikilink・frontmatter wikilink・Markdown link は相対形式を保つ。移動後の note から元の参照先（参照先も移動する場合は移動後の位置）を指す相対パスに再計算し、表記が変わらなければ書き換えない
  - 補足: 移動元ファイルの mtime が DB と一致しない場合は **エラー**（stale 検出）。書き換え対象の外部ファイルは stale チェックしない（文字列マッチによる安全な書き換えのため）
  - 補足: 移動途中のエラーではディスク変更を best-effort でロールバックし、書き換えたファイルの内容・permission・元の mtime を復元する。ロールバック自体が失敗した場合は、返却エラーに復元できなかったファイル・移動し戻せなかったファイルと、手動復旧後に `mdhop build` を実行するヒントを含める
  - ディレクトリモード: `--from` が末尾 `/` またはディスク上ディレクトリの場合、配下の全 `.md` ファイルを一括移動する
    - `--to` も自動的にディレクトリとして扱う（`--to` が `.md` で終わる場合はエラー）
    - 全ファイルの移動先を確定してからリンク書き換えを1回だけ行う（中間状態の曖昧性問題を回避）
    - 移動セット内ファイル間のリンク（相対リンク含む）も正しく書き換える
    - ディスク状態は全ファイルが一貫している必要がある（normal と already-moved の混在はエラー）
    - 移動元と移動先ディレクトリが包含関係になる指定（例: `--from sub --to sub/inner`）はエラー
    - ディレクトリ配下の非 `.md` ファイル（asset）も一緒に移動する
- `delete`
  - 必須: `--file`（複数回指定可）
  - 任意: `--vault`, `--format`, `--rm`
  - `--rm`: 登録済みファイルをディスクから削除してからインデックスを更新する
  - 補足: 未登録ファイルが指定された場合はエラー（`--rm` でもファイルは削除されない）
  - ディレクトリモード: `--file` に末尾 `/` またはディスク上ディレクトリを指定すると、DB に登録された配下の全ファイル（note + asset）を一括削除する
    - DB にファイルが登録されていないディレクトリはエラー
    - `--rm` 時は登録済みファイルと DB の更新後、配下の残存非 Markdown ファイル（未登録 asset など）を削除してから空ディレクトリを再帰的に掃除する。隠しディレクトリ内のファイルと `.md` ファイル（未登録 Markdown を含む）は残す
    - 登録済みファイルと DB の更新が完了した後に、残存 asset の走査・削除または空ディレクトリ掃除で予期しないエラーが起きた場合は、成功出力を出さず、完了済みの更新と失敗 path・原因を示すエラーにする
- `disambiguate`
  - 必須: `--name`
  - 任意: `--target`, `--file`, `--vault`, `--format`
  - 補足: `--name` が一意なら自動で対象決定。複数ある場合は `--target` 必須。
  - 補足: `--file` 指定時は対象ファイルのみ書き換える
  - 補足: `--scan` を指定すると DB を使わずに全ファイルを走査して書き換える（初期救済用）
  - 補足: `--scan` は `build.exclude_paths` に従う（除外ファイルは候補にも走査対象にもならない）
  - 補足: phantom を指す壊れたパスリンクも `--name` の対象に含める（`repair` の後の個別解決用）
- `repair`
  - 必須: なし
  - 任意: `--vault`, `--format`, `--dry-run`, `--path`, `--exclude`
  - 補足: DB 不要（ファイル走査ベース）。build 前に実行可能
  - 補足: `--path` / `--exclude` は source note を path glob で絞る。候補探索は対象外 note も含む vault 全体（`build.exclude_paths` 適用後）で行う
  - 補足: 壊れたパスリンク（target が存在しない wikilink/markdown）と vault-escape リンクを basename リンクに自動書き換え
  - 補足: vault-escape リンクは候補数に関係なく常に basename 化（escape 解消が最優先。その後 ambiguous になるなら `disambiguate` で対応）
  - 補足: 壊れたパスリンクは basename の候補が 0-1 個のみ修復。2 個以上はスキップ（`disambiguate` で個別解決する）
  - 補足: basename リンク（`[[X]]`）は対象外（パスリンクのみ）
  - 補足: リンク先ファイルがディスク上に存在する場合はスキップ（`build.exclude_paths` で除外されたファイルへのリンクを壊さない）
  - 補足: `--dry-run` は実行時と同じ書き換え候補の対応検証を行い、ディスク変更せず結果のみ返す（将来の I/O 成功は保証しない）
  - 補足: repair 後に `build` を実行してインデックスを作成・更新する
  - 補足: repair 後に build が曖昧リンクで失敗する場合は `disambiguate` で対応する
  - 補足: URL リンク、tag/frontmatter リンクは対象外
- `simplify`
  - 必須: なし
  - 任意: `--vault`, `--format`, `--dry-run`, `--file`
  - 補足: DB 不要（ファイル走査ベース）
  - 補足: パスリンク（相対・絶対）の basename がユニーク、またはルート優先で解決可能な場合に basename リンクに短縮する
  - 補足: basename リンクは対象外（既に短い形式）
  - 補足: 壊れたリンク・vault-escape リンクはスキップ（`repair` で対応）
  - 補足: asset のパスリンクは、note namespace に同名 basename が存在しない場合のみ短縮する
  - 補足: `--file` で対象ファイルを制限できる（複数回指定可）
  - 補足: `--dry-run` は実行時と同じ書き換え候補の対応検証を行い、ディスク変更せず結果のみ返す（将来の I/O 成功は保証しない）
  - 補足: simplify 後に `build` を実行してインデックスを更新する
  - 補足: `build.exclude_paths` に従う
  - 補足: quoted frontmatter wikilink（引用符付き YAML scalar / list item 値内の wikilink）も、上記の短縮条件を満たす場合に対象となる
  - 補足: URL リンク、tag、frontmatter の raw path 値（`frontmatter_path`）は対象外
- `convert`
  - 必須: `--to`（`wikilink` or `markdown`）
  - 任意: `--vault`, `--format`, `--dry-run`, `--file`（複数回指定可）
  - 補足: DB 不要（ファイル走査ベース）。build 前に実行可能
  - 補足: wikilink ↔ markdown link を相互変換する。embed（`![[x]]` ↔ `![x](x)`）も対象
  - 補足: Markdown の自己リンクは fragment 内の釣り合った括弧を保持して変換する（例: `[節](#Heading (detail))` → `[[#Heading (detail)|節]]`）。閉じ括弧が不足するリンクは変更しない
  - 補足: URL リンク、tag、frontmatter リンクは対象外
  - 補足: `build.exclude_paths` に従う（除外ファイルは走査しない）
  - 補足: `--file` 指定時は対象ファイルのみ変換する
  - 補足: `--dry-run` は実行時と同じ書き換え候補の対応検証を行い、ディスク変更せず結果のみ返す（将来の I/O 成功は保証しない）
  - 補足: convert 後に `build` を実行してインデックスを作成・更新する
- `resolve`
  - 必須: `--from`, `--link`
  - 任意: `--vault`, `--format`, `--fields`
- `query`
  - 必須: `--file` / `--tag` / `--phantom` / `--name` のちょうど一つ
  - 任意引数と制約: [query の CLI 契約](#query-の-cli-契約)参照
- `inspect`
  - 必須: `--file`
  - 任意: `--vault`, `--format`, `--fields tags,meta`, `--include-head`
  - 属性選択と本文読取: [inspect の CLI 契約](#inspect-の-cli-契約)参照
- `search`
  - 必須: なし
  - 任意: `--vault`, `--format`, `--fields`, `--where`, `--path`, `--exclude`, `--no-exclude`,
    `--sort`, `--include-head`, `--limit`, `--offset`, `--sample`, `--count`, `--no-tags`, `--no-outgoing`, `--no-incoming`
  - 補足: 起点不要の全ノード検索。`type='note' AND exists_flag=1` のノードのみ対象
  - 補足: `--where` でメタデータ条件フィルタ（query の `--where` と同じ構文）
  - 補足: `--path` でパス glob 包含フィルタ（repeatable, OR 結合）
  - 補足: `--no-tags` でタグ edge を持たない note のみに絞る
  - 補足: `--no-outgoing` で outgoing edge（タグ edge を含む）を持たない note のみに絞る
  - 補足: `--no-incoming` で incoming edge を持たない note のみに絞る
  - 補足: `--sort key` で昇順、`--sort -key` で降順。未指定時は path 順。key には meta key と computed field（`lines` / `outgoing_count` / `incoming_count`）を指定できる
  - 補足: `--limit` / `--offset` でページング。total フィールドに limit/offset 適用前の総件数を返す
  - 補足: `--sample N` でフィルタ適用後・limit/offset 適用前の候補から無作為に N 件を返す。`N >= 候補総数` の場合は候補全件を返す。`--limit` / `--offset` / `--sort` とは併用不可（sort を無視して返すと気づきにくいため明示エラーにする）。total フィールドは sample 適用前の総件数を返す
  - 補足: `--count` でフィルタ適用後の件数のみ返す。text は `count: N`、JSON は `{"count": N}`。`--fields` / `--include-head` / `--sample` / `--sort` / `--limit` / `--offset` とは併用不可
  - 補足: `--fields meta` で frontmatter メタデータを全 key 追加出力（opt-in）
  - 補足: `--fields meta.<key>` で特定 frontmatter key のみ追加出力（複数指定可）。`meta` と併用すると全 key を出力
  - 補足: `--fields lines` / `outgoing_count` / `incoming_count` で computed field を追加出力（opt-in）。`lines` は build/update 時に確定したファイル全体の行数（frontmatter 含む）、`outgoing_count` / `incoming_count` は edges の集計値（tag edge を含む）
  - 補足: `--include-head N` でノート先頭N行を追加出力
  - 補足: `--exclude`, `--no-exclude` は search 用の `exclude` 設定と CLI 除外を扱う。query の hide / via 条件とは独立
- `diagnose`
  - 必須: なし
  - 任意: `--vault`, `--format`, `--fields`, `--path`, `--exclude`
  - 補足: `--path` / `--exclude` は source note（リンクを書いている側の note）を path glob で絞る（複数回指定可、glob 仕様は除外フィルタと同じ）
  - 補足: フィルタ指定時、`phantoms` は対象 note から参照されている phantom のみ、`basename_conflicts` / `asset_basename_conflicts` は対象 note からの basename 形式リンクが指す衝突グループのみ（リンク解決リスクがあるもの）を返す
  - 補足: `--path` / `--exclude` は CLI 引数のみで動作し、`mdhop.yaml` の `exclude` 設定は diagnose に適用されない。フィルタ未指定時の挙動は従来どおり
  - 補足: `--fields anchors` で anchor 切れ検出（`broken_anchors`）を有効化する。これは **opt-in**（`--fields` 未指定時は他フィールドと違って出力されない。対象 note をディスクから読むため）。`[[note#見出し]]` / `[text](note.md#fragment)` の fragment が target note（実在 note）の見出しに存在しないものを報告する
  - 補足: anchor 一致は Obsidian 互換正規化（`#` 除去・句読点／記号除去・空白畳み込み、大小文字とアクセントは保持）。block reference（`#^id`）と setext heading は対象外（ATX heading のみ）。target が phantom / asset のものは対象外（note 切れは phantom 検出側の領分）
  - 補足: 見出し内のバッククォート（インラインコード）はアンカー化前に剥がされず、見出しテキスト全体（バッククォート含む）からアンカーを生成する。リンク側の fragment には元々バッククォートが付かないため、両者は句読点除去後のテキストで一致する
- `meta-check`
  - 必須: `--key`（検査する frontmatter key。複数回指定可）
  - 任意: `--vault`, `--format`, `--kind`, `--path`, `--exclude`
  - 補足: 指定 key の frontmatter 値が vault 内 path / wikilink として実在するかを検査し、解決できない値を `issues` として返す
  - 補足: `--kind path`（既定）は値を raw path として解釈（markdown link と同じ解決規則: `./` `../` は note 起点、`/` 含みは vault 相対、なしは basename 解決）。`--kind wikilink` は `[[...]]` として解釈する。`--kind auto` は値ごとに trim 後、空値と URL（`://` を含む）を skip し、`[[` で始まる値を wikilink、それ以外を path として検査する（Obsidian property 形式の混在 scalar / list を 1 回で検査する用途）
  - 補足: `--kind path` で値が末尾 `/` の場合はディレクトリ参照として扱い、ディスク上に実在するディレクトリなら issue にしない。存在しない場合は `not_found`
  - 補足: 値はリスト・スカラーを問わず meta テーブルで値ごとに展開済みのため、`--kind` に list / scalar の区別はない
  - 補足: URL 値（`://` を含む）と空値は許可（issue にしない）。`reason` は `not_found` / `ambiguous`（basename 多重解決）/ `vault_escape` / `not_wikilink`（`--kind wikilink`、または `--kind auto` で wikilink と判定した値が有効な `[[...]]` でない）
  - 補足: 検査元は索引済みの実在 source note の meta 値のみ。`build.exclude_paths` で索引から除外した実在 note / asset も、本文を解析・登録せず参照先候補として解決する。除外候補も basename の曖昧性判定に含め、既存のルート優先規則を適用する
  - 補足: `--path` / `--exclude` は source note を path glob で絞る（CLI 引数のみ）。source 範囲外または `build.exclude_paths` で除外された実在参照先も解決候補に含める
  - 出力: 各 issue は既存の `source_path` / `key` / `value` / `reason` に加えて、値開始位置の 1 始まり整数 `line` を JSON に返す。text は `location: <source_path>:<line>` も返す。位置は値と同じ index snapshot の vault 相対 path であり、既存 index は `mdhop build` で再生成する
- `meta-validate`
  - 必須: なし（ただし `--require` も `mdhop.yaml` の `meta.profiles` も `meta.types`（string 以外の宣言）もどれも無い場合はエラー。検査対象が存在しない）
  - 任意: `--vault`, `--format`, `--require`（複数回指定可）, `--path`, `--exclude`
  - 補足: frontmatter が宣言済み schema に準拠するかを検査し、違反を `violations` として返す。meta-check（値の参照先が実在するか）とは別コマンド（ADR 0019）
  - 補足: `--require <key>` は対象 note に当該 key の非空値が無い場合 `missing` を報告する。空値・null の frontmatter 値は index 時に落ちるため、`key:`（値なし）も `missing` 扱い（key 欠落と同じ欠陥）
  - 補足: `mdhop.yaml` の `meta.profiles` で path パターン別の必須 key を宣言できる。`path` 省略時は全 note 対象、`path` 指定時は `--path` と同じ glob 表現で source note を絞る。複数条件がある場合は profile を複数書く。`--require` を明示した実行では、その実行時の必須 key は `--require` で指定した key に置換され、`meta.profiles` の必須 key は検証されない。mdhop.yaml には書き戻されない（`meta.profiles` を恒久的に変更したい場合はファイルを直接編集する）
  - 補足: `mdhop.yaml` の `meta.types` で `date` / `number` / `semver` 宣言された key の値が型として解釈できない場合 `type` を、`ordered` 宣言の key の値が一覧外の場合 `enum` を報告する。`string` / 未宣言の key は型・enum 制約を持たないため対象外
  - 補足: list の値は要素ごとに検査し、不正な各値をそれぞれの `value` として報告する
  - 補足: 型／enum 検査は `--require` の有無に関わらず常に走る（`meta.types` 宣言が根拠）。`--require` は欠落検査を追加するだけ
  - 補足: `--path` / `--exclude` は source note を path glob で絞る（CLI 引数のみ。`mdhop.yaml` の `exclude` 設定は適用されない）
  - 出力: 各 violation は既存の `source_path` / `key` / `value` / `reason` に加えて、値開始位置の 1 始まり整数 `line` を JSON に返す。text は `location: <source_path>:<line>` も返す。`missing` は実在する値の位置ではないため、ノート先頭の編集開始位置として常に `line: 1` を返す。位置は index snapshot の vault 相対 path であり、既存 index は `mdhop build` で再生成する
- `reachable`
  - 必須: `--from`（vault 相対の note path。asset / 未登録 path はエラー）
  - 任意: `--vault`, `--format`, `--fields`, `--path`, `--exclude`, `--route`
  - 補足: `--from` の note から outgoing リンクを BFS で辿り、対象 note 集合（`type='note' AND exists_flag=1` に `--path` / `--exclude` glob を適用。`--path` 未指定は全 note）を reachable / unreachable に分けて返す
  - 補足: 辿る link_type は `wikilink` / `markdown` / `markdown_reference` / `frontmatter_wikilink` / `frontmatter_path`。tag 系（`tag` / `frontmatter`）は辿らない（tag を共有するだけでは到達扱いにしない）
  - 補足: `--from` 自身は対象集合内なら reachable に含まれる（0 hop）。対象集合外の note は走査の中継にはなるが、reachable / unreachable のどちらにも出ない
  - 補足: `--route` で reachable な各 note への最短経路を `routes` として追加出力する（中継 note は対象集合外でも経路に現れる）
  - 補足: `--path` / `--exclude` は CLI 引数のみで動作し、`mdhop.yaml` の `exclude` 設定は適用されない
- `graph`
  - 必須: なし
  - 任意: `--vault`, `--format`（`json|dot`, default: json）, `--path`, `--exclude`, `--include-phantoms`
  - 補足: node 集合は実在する note / asset（`--path` / `--exclude` glob を適用。`--path` 未指定は全件）。edge はその誘導部分グラフ（両端が node 集合内の link 出現のみ。同一ペアでも出現ごとに 1 edge）
  - 補足: tag node は出力しない（tag edge も出力されない）。tag を含めた可視化が必要になったら将来拡張する（ADR 0016）
  - 補足: `--include-phantoms` で node 集合内の note から参照されている phantom を node / edge に含める（default は除外）
  - 補足: node の `id` は出力スコープの参照キー（edge の `source` / `target` が指す）。build をまたいだ安定性は保証しない
  - 補足: node の出力順は type（asset → note → phantom）、次に path（phantom は name）の昇順
  - 補足: dot のラベルは note / asset が path、phantom が `(phantom) <name>`
  - 補足: `--path` / `--exclude` は CLI 引数のみで動作し、`mdhop.yaml` の `exclude` 設定は適用されない
- `stats`
  - 必須: なし
  - 任意: `--vault`, `--format`, `--fields`
- `init-meta`
  - 必須: `--preset` または `--scan`（少なくとも一方）
  - 任意: `--vault`, `--write`, `--no-comment`
  - 補足: DB 不要（ファイル走査ベース）。build 前に実行可能
  - 補足: `--preset` は推奨型定義（date×10, number×4, semver×1）を出力する
  - 補足: `--scan` は vault の全 .md ファイルの frontmatter を走査し、各キーの型を推定する
    - 型推定: 値の 80% 以上が date/number/semver にマッチすれば推定。それ以外は string
    - カーディナリティが 10 以下の string キーは ordered 型候補としてコメントで提示する
    - `tags`, `aliases` キーは型推定対象から除外される（well-known な特殊キー）
  - 補足: `--preset --scan` 併用時は scan 結果を優先する（データドリブン > curated）
  - 補足: デフォルトは stdout に YAML を出力。`--write` で `mdhop.yaml` に直接書き込む
  - 補足: `--write` 時、既存の `build`/`exclude` セクションは保持。既存の `meta.types` キーは上書きせず、`meta.types: null` は空の型定義として扱い生成した型を追加する
  - 補足: `--no-comment` はコメント（推定根拠、ordered 候補、preset 表示）を省略する

## update の削除挙動

- ディスクから消えたファイルを `--file` で指定した場合は delete と同じ扱い
  - 参照がある場合: phantom に変換
  - 参照がない場合: ノードを完全に削除

## delete の削除挙動

- 指定ファイルが削除されていた場合は、参照の有無で扱いが変わる
  - 参照がある場合: phantom として扱う
  - 参照がない場合: ノードを完全に削除する
- `--rm`（`RemoveFiles`）は vault の外に出るパスの削除を拒否する（`path escapes vault` エラー）

## リンク解釈（互換性）

- wikilink: `[[Note]]`, `[[Note|alias]]`, `[[Note#Heading]]`, `[[Note#^block]]`
- markdown link / image: `[text](note.md)`, `[text](./note.md#heading)`, `![alt](image.png)`
  - `note.md` は `[[note]]` と同一扱い。destination の構文 delimiter を原文で認識し、backslash で escape された括弧は開閉として数えない。inline title / angle destination の新規対応は含まない
  - destination は原文を1回走査し、ASCII punctuation の backslash escape と semicolon 付きの有効な HTML5 named / decimal（1–7桁）/ hexadecimal（1–6桁）文字参照を復号する。基準は [CommonMark 0.31.2](https://spec.commonmark.org/0.31.2/#entity-and-numeric-character-references)。非 punctuation の backslash と無効な参照は保持する。`\&amp;` は literal `&amp;`、`&amp;amp;` は `&amp;` で止まり、生成された文字列を再走査しない
  - lexical 復号後に外部 URI を判定し、内部 destination の最初の `#` で path / fragment を分離する。その後、各成分の valid `%HH` を1回復号する（mdhop 固有の local file 対応規約）。`+` と不正な percent triplet は保持する。`A%20B.md` は `A B.md`、`A%2520B.md` は literal `%20` を含む `A%20B.md`、`A%23B.md#H%23I` は path `A#B.md` と fragment `#H#I` になる。percent 復号で生まれた scheme / `#` を再分類・再分離しない
  - decoded path に NFC、basename / relative / path 分類、Vault 外参照検証を適用する。encoded `..` / separator も strict validation の対象。resolver は decoded 値を再復号しない
  - fragment-only destination は graph を作らず、convert の自己リンク処理だけで同じ解釈を使う。wikilink、frontmatter wikilink、frontmatter raw path にこの復号を適用しない
- [GFM 形式の表](https://github.github.com/gfm/#tables-extension-)の wikilink: header と delimiter row の cell 数が一致する表の header / body で、`[[X\|表示]]` を target `X`、alias「表示」とする
  - 先頭末尾 pipe の省略、alignment colon、空行や別 block、fence/frontmatter/code span を扱う。本文走査が対応する文脈に限定し、CommonMark 全体の block parser ではない。pipe のある行だけで表とは判定しない
  - 表外と frontmatter は従来の first `|` 分離を保ち、同原文の target は `X\`。表内外の異なる意味を同一へ丸めない。raw link と行位置を保持し、表文脈を index に保存する
- Markdown 参照リンク: full `[説明][guide]`、collapsed `[guide][]`、shortcut `[guide]`。image reference `![alt][guide]` も内部リンク関係を作り、raw 原文は通常 image と同じく `!` を除く
  - 同一文書の独立行 `[guide]: B.md` を先に収集し、使用箇所を定義の destination へ結び付ける。前方・後方定義とも有効。同じ行の複数出現は個別の edge になる
  - 定義は行頭 0〜3 space、colon 後の space/tab、bare destination または `<destination>`、同一行の任意 title（`"title"` / `'title'` / `(title)`）に対応する。bare destination 内の escape されていない括弧は釣り合っている必要がある。wrapper と title は destination に含めない
  - label は前後の space/tab を除き、連続する内部 space/tab を 1 space に畳み、Unicode lowercase（Go の `strings.ToLower`）で照合する。空 label は不可。同じ正規化 label の有効定義は最初を使う
  - 複数行 label/definition/title、block container 内の定義、入れ子 bracket、label/title の HTML entity / backslash escape の全面対応は対象外。CommonMark 全文法準拠ではない
  - frontmatter、fenced code、inline code 内の参照は除外する。定義行自体と使用された参照の表示文字列は tag を作らない。未使用定義と未定義参照は edge/phantom を作らない。未定義 full/collapsed を部分的な shortcut として解釈しない
  - destination の解決は通常 Markdown link と同じ。未作成 target は label でなく destination 由来の phantom。URL と fragment のみの destination は graph に含めない
  - edge type は `markdown_reference`。outgoing/backlinks/共通ターゲット方式の twohop、reachable、graph に含む。定義の自動 rewrite は行わず、move/disambiguate/repair/simplify/convert の書き換え対象外
  - 保存定義 target の解決先を変える、または曖昧にする add/move（source/target/directory move を含む）は file/DB 更新前にエラー。意味が変わらない操作と未作成 target の一意な promotion は許可する。必要なら定義を手で更新する
- tag: `#tag`, `#nested/tag`, `#日本語タグ`, `#my-tag`, frontmatter `tags`
  - ネストタグは祖先に展開される: `#a/b/c` → `#a`, `#a/b`, `#a/b/c` の各タグが resolve 可能
- 外部 URI: `http://...` / `https://...`、`mailto:...`、`ftp:...`、および `scheme://...` 形式は内部リンクとして解析しない（scheme は大小文字を区別しない）。未知の opaque `foo:bar` は既存どおり内部名として扱う
- frontmatter 内 wikilink: `tags` キー以外の全キーを対象に、Obsidian property link と同様 **引用符で囲まれた YAML scalar / list item 値** のみから `[[...]]` を解析する（double quote / single quote）
  - bare `key: [[Note]]` と bare list item `- [[Note]]` は YAML 上の nested sequence であり、frontmatter のリンクとして扱わない（edge・phantom・meta-check issue を生成しない）
  - rewrite を伴う実更新は、書き込み・DB 更新・move より前に、予定した引用符付き source と YAML decode 後の値の対応を検証する。対応を証明できない候補が一つでもあれば操作全体を変更せずエラーにする
- frontmatter の raw path 値（`meta.link_keys` 設定時のみ）: 宣言した key の値を `frontmatter_path` の edge として解析する
  - 解決規則は markdown link と同じ（`./` `../` は note 起点、`/` を含めば vault 相対パス、含まなければ basename 解決）
  - 外部 URI（`http://...` / `https://...`、`mailto:...`、`ftp:...`、および `scheme://...` 形式）と wikilink 値（`[[...]]`、frontmatter_wikilink として解析済み）はスキップ。scheme は大小文字を区別せず、未知の opaque `foo:bar` は raw path として扱う
  - 値全体を path とみなす（`#` fragment の分離はしない）
  - 厳密モードの検証対象（vault escape・曖昧 basename は build / update / add / move / move 配下の再解析でエラー）。解決できない値は phantom になる
  - `link_keys` 未設定なら従来挙動（raw path 値は edge にならない）
  - `tags` は `link_keys` に指定できない（設定エラー）
  - 制約: raw path 値はリンク構文ではないため、`move` / `disambiguate` / `simplify` / `repair` / `convert` の書き換え対象外。raw path 値の解決先が変わってしまう `add` / `move` は操作前にエラーになる（frontmatter 値を手で直してから再実行する）
- frontmatter の `aliases` は初期バージョンでは解析しない

### 再出力と既存 index

- Markdown rewrite は decoded path / fragment の literal `%`、`#`、`&`、backslash、括弧、backtick などを percent encode して再解析時の意味を保つ。拡張子、relative move、表示文字列、subpath、embed の方針は従来どおり。表内 wikilink の alias separator は `\|` を保持する
- convert は decoded Markdown target を wikilink にする。literal `#` / `|` / `]]` や target/subpath の backtick などで現行本文 scanner が同じ意味を表現できない場合は原文を保持する。必須 rewrite の参照先を wikilink として表現できない場合は file/DB 更新前にエラーにする。参照定義と frontmatter raw path の自動 rewrite は追加しない
- 単一行 quoted frontmatter wikilink の target/subpath は backtick を保持して必須 rewrite できる。本文の code span 制約による拒否と区別し、原文対応と YAML 値の意味を保存できない candidate は file/DB 更新前に拒否する
- index の解釈 version が異なる既存 DB は、読み取り・update・DB 利用 mutation の入口で `mdhop build` を案内するエラーになる。in-place migration / partial update で旧新の解釈を混在させず、build が成功した一時 DB で置換する。CLI flags と stdout JSON field は変更しない

## resolve のルール（要点）

- resolve は `from_note` にそのリンクが実際に存在する場合のみ解決する
- 全 link は source と exact raw が一致する保存 edge を優先する。複数出現が同一 target/subpath なら解決し、異なる意味なら曖昧エラーにする。表文脈と定義は disk を再走査せず index snapshot から取得する
- 参照リンクは `resolve --from A.md --link '[説明][guide]' --format json` で、source と使用原文が一致する保存済み edge から既存の `type,name,path,exists,subpath` を返す。新しい JSON field は追加しない。本文・定義を再走査せず index snapshot を読むため、disk の変更・削除は明示的 update まで反映しない。索引化されていない使用原文はエラーになり、label を path として fallback しない
- 解決結果は必ず1つになる（曖昧な場合はエラー）
- `[[Note]]`: basename を Vault 全体から探索（note → asset → phantom の順）
  - 候補1件なら解決
  - 複数なら曖昧としてエラー（ルート優先例外あり）
  - note と asset は別の basename キー空間（note は拡張子除去、asset は拡張子込み）
- `[[#Heading]]` : 同一ファイル内の見出しとして解決（`from_note` を返す）
- `[[path/to/Note]]`: Vault ルート相対で解決（拡張子省略可）
- `[[./Note]]`, `[[../Note]]`: `from_note` のディレクトリ基準で解決
- Markdown link:
  - `/` 始まり: Vault ルート相対
  - `./` / `../` 始まり: `from_note` 基準
  - `/` を含むがプレフィックスなし（例: `sub/C.md`）: パスとして解決
  - `/` を含まない（例: `Design.md`）: basename 解決（`[[note]]` と同一扱い）
  - Vault 外へ出るパスは厳密モードではエラー
- head / snippet / anchor 診断 / directory meta-check / delete は、NFC の index path に対応する実際の disk 表記を解決して読み取り・削除する
- add / move / DB 利用 disambiguate の frontmatter wikilink 書き換え後は、meta 値・値開始行・型も現在の内容へ更新する。disambiguate で書き換えた phantom 参照は選択した実在 note へ付け替え、`--file` 範囲外の参照は保持する
- path 比較は NFC 正規化後に行う（NFD の実ファイル名と NFC の参照値が混在しても同一ノードに解決される）

### resolve の一致モード

- exact raw がない場合は従来の正規化一致（参照リンクは exact raw のみ）
  - alias を除去した一致
  - wikilink と markdown link の同一ターゲット一致
  - basename 一致（ただし曖昧ならエラー）

## query の関係と返却順

`Plan.md` が `topics/設計方針.md` と `#planning` を参照し、`Guide.md` が `Plan.md`・`topics/設計方針.md`・`#planning` を参照し、`Review.md` が `Plan.md` を参照する場合:

- backlinks: `Plan.md ← Guide.md` と `Plan.md ← Review.md`。入口へのリンク元。
- outgoing: `Plan.md → topics/設計方針.md` と `Plan.md → #planning`。入口が参照する先。タグも索引上の通常のリンク先として含める。
- twohop: `Plan.md → topics/設計方針.md ← Guide.md` と `Plan.md → #planning ← Guide.md`。`Guide.md` は対象として一件、経由先は二件。JSON キーは `2hoplink` とする。

全入口タイプで twohop は **入口→経由先←対象** のみ。従来の非 note 入口に対する `入口←経由元→対象` 特例は廃止する。tag / asset / phantom 入口は索引で outgoing を持たないため、twohop は空、backlinks は従来どおり返る。タグの階層親も索引済みのリンク先なら outgoing に含め、twohop の経由候補と一致させる。

1. 実行時点の索引から、選択した関係の候補リンクを取得する。直接関係には `--link-key`、twohop には型付き経由指定・包含・除外を適用する。twohop は入口から出る各リンク先を経由先とし、そこへリンクする別対象を集める。X と Y を共有する対象で X だけ via 除外した場合は Y 経由で残り、許可された via がゼロになった場合だけ消える。入口自身は関連対象から外す。`build.exclude_paths` 等により索引にない関係は復元しない。外部 URL は対象外。DB schema / 索引更新方法は変更しない。
2. 対象に `--path` / `--where` を適用する。hide 対象を除く。twohop では hidden via を **対象の発見からは除かず**、後で構造化した経由先欄からのみ隠す。隠した経由先が全件でも対象は残し、`relation: []` と `hidden_relation: true` を返す。
3. 各関係内で typed node ID により対象を一件にまとめる。backlinks と twohop の両方にいる `Guide.md` は両欄に一回ずつ載る。twohop で同じ対象に複数の経由先があれば `relation` に全件載せ、経由ごとの旧件数上限は設けない。
4. 各欄を独立に安定順へ並べる。path がある node を Vault 相対 path の Unicode NFC コードポイント順、path がない node をその後ろへ type・NFC name 順。最後に安定した typed node ID で同順位を解く。経由先も同順。ロケール依存順にはしない。
5. 一関係を明示した場合だけ、その欄の対象単位で offset 件を飛ばし limit 件を返す。空・hidden・重複候補は枠を使わない。limit 未指定なら残り全部。`next_offset` は次の有効対象が存在するときだけ次に渡す整数、それ以外は null。存在判定用の一件は本文プレビューを読まない。total / count は計算しない。実行をまたぐ索引の保存・更新検知・整合保証・専用エラーは設けない。
6. 最後に、返す対象の head と、その対象との関係を示す snippet を必要に応じて読む。outgoing の snippet の読み取り元は対象ではなく入口の本文となる。複数関係を全件返す場合も選択した関係に必要な本文だけ読む。表示から隠した via に紐づく snippet は生成しない。

### ミューテーション系の出力

- `--format json|text`（default: text）
- `--fields` は不要（結果はフラットで小さい）
- text では空スライスのセクションを省略、JSON では `[]` を出力する
- delete: `deleted`, `phantomed`
- update: `updated`, `deleted`, `phantomed`
- set: `file`, `key`, `value`, `created`
- add: `added`, `promoted`, `rewritten`
- move（単体）: `from`, `to`, `rewritten`（`--to-template` の場合、実行結果の `to` は実際の移動先 path、dry-run は計画した移動先 path）
- move（ディレクトリ）: `moved[]`（`from`, `to` の配列）, `rewritten`（`--to-template` の実行結果は実際の移動先、dry-run は計画した移動先）
- disambiguate: `rewritten`
- simplify: `rewritten`, `skipped`
- repair: `rewritten`, `skipped`
- convert: `rewritten`

## 出力形式

format / fields はコマンド別の契約に従う。query は relations、inspect は属性 fields を選び、search 等の fields は従来どおり。JSON stdout は単独で parse 可能とし、警告は stderr に出す。

### query の JSON と text

JSON stdout は agent 向けインターフェース。警告は stderr。キー `backlinks` / `outgoing` / `2hoplink` は選択した関係だけ出す。選択した欄が空なら必ず `[]` とし、未選択欄はキー自体を省略する。`omitempty` による空欄消失を避ける。関係の選択が一つでも三つでも同じ形式を使う。

上の利用例に対する JSON（ページ指定なし）:

```json
{
  "entry": {"type":"note","name":"Plan","path":"Plan.md","exists":true},
  "backlinks": [
    {"type":"note","name":"Guide","path":"Guide.md","exists":true},
    {"type":"note","name":"Review","path":"Review.md","exists":true}
  ],
  "outgoing": [
    {"type":"note","name":"設計方針","path":"topics/設計方針.md","exists":true},
    {"type":"tag","name":"#planning"}
  ],
  "2hoplink": [
    {"type":"note","name":"Guide","path":"Guide.md","exists":true,
     "relation":[
       {"type":"note","name":"設計方針","path":"topics/設計方針.md","exists":true},
       {"type":"tag","name":"#planning"}
     ],"hidden_relation":false}
  ],
  "page":{"offset":0,"limit":null,"next_offset":null}
}
```

`relation` は対象が入口と共有する経由先であり、対象との別関係種別ではない。ノート以外も `type` と `name` で識別する。asset は path と exists、phantom は name を持ち path を持たない。未作成リンクの phantom と実在ファイルを混同しない。`exists` は実在確認可能な note / asset だけに付ける。hide された経由先の識別子は `relation` に出さず、存在だけ `hidden_relation` で示す。例えば上例で `--hide-tag '#planning'` を付けると `Guide` 自体は残り、`relation` は設計方針だけ、`hidden_relation` は true になる。トップレベル独立 `tags`、入口の `head` / `meta` / `snippet`、旧 `items` / `total` は query に設けない。

単一関係で `--relations backlinks --limit 1` の結果は `backlinks` が一件、他の関係キーは無し、`page: {"offset":0,"limit":1,"next_offset":1}`。次ページで末尾に達したら `next_offset: null`。`offset` が対象数以上なら選択欄 `[]`、`next_offset: null`。

text は入口を先頭に、選択した関係の見出しを backlinks → outgoing → 2hoplink の順に出し、各対象を一行に一件表示する。対象識別子と経由先識別子は JSON の文字列引用規則（ダブルクォートと escape）で囲む。コンマ・タブ・改行を含む path でも区切りと混同しない。型は識別子の外に置く。twohop の対象の次に `relation:` 行、その下に経由先を一行ずつ置き、非表示経由がある場合は別行に `hidden_relation: true` と書く。最後に page の三値を表示する。プレビュー指定時は対象または経由先の直下に `head:` / `snippet:` を追加し、snippet は `source_path`、行範囲、引用した生行をインデントして並べる。機械で読む用途は JSON を用いる。

未選択の関係は見出しごと省略する。選択済みで対象が0件の場合だけ、その見出しの下に `（該当なし）` と表示する。

```text
entry:
  note "Plan.md"
backlinks:
  note "Guide.md"
  note "Review.md"
outgoing:
  note "topics/設計方針.md"
  tag "#planning"
2hoplink:
  note "Guide.md"
    relation:
      note "topics/設計方針.md"
      tag "#planning"
page:
  offset: 0
  limit: null
  next_offset: null
```

### query / inspect の本文プレビュー

`head` は本文の frontmatter と直後の空行を飛ばした先頭 N 行。見出し抽出や自動要約ではない。query では返した関連対象が note の場合だけ、その対象に `head: ["..."]` を付ける。inspect では指定 note に同じ内容を付ける。search の head 契約も維持する。

head 未指定時と note 以外の対象では `head` キーを省略する。指定した note の本文が空なら `head: []` とする。snippet も未指定ならキーを省略し、指定時は対応するリンク出現を配列で返す。空の配列と未指定を `omitempty` で混同しない。

`snippet` は関係を立証する実リンク出現の生テキストと前後 N 行。frontmatter 内のタグやリンクも特別整形せず、該当行をそのまま返す。JSON では対象との対応を失わないよう、backlinks / outgoing 対象に `snippet` 配列、twohop では各 `relation` 経由先に `snippet` 配列を付ける。各要素は `source_path`、1 始まりで両端を含む `start_line` / `end_line`、生の行配列 `lines` を持つ。同じ edge 出現は一回、別の出現は一つずつ返し、前後文脈が重なっても統合しない。複数出現はファイル・行・索引 edge ID で安定化する。同じ対象でも条件に合うリンク出現だけを採る。

- `Plan.md → topics/設計方針.md` の outgoing: `Plan.md` 内の設計方針へのリンク箇所。
- `Guide.md → Plan.md` の backlinks: `Guide.md` 内の Plan へのリンク箇所。tag / asset / phantom 入口の backlinks も実際の参照元本文。
- `Plan.md → topics/設計方針.md ← Guide.md` の twohop: `Guide.md` 内の設計方針へのリンク箇所。入口 Plan 側のリンク箇所はここでは重ねて返さない。

twohop の入口側根拠が必要なら、経由先を入口として backlinks を `--path Plan.md` で絞る別 query で取得できる。tag 経由なら `--tag planning --relations backlinks --path Plan.md --include-snippet 0`。この補完は通常出力の必須機能にしない。`--path` / `--where` は返却対象の選択だけに掛け、snippet の参照元へ再適用しない。`--link-key` は直接関係の出現だけを絞り、twohop の snippet を落とさない。落ちた対象の出現、ページの先読み一件、表示から隠した経由先の snippet は生成しない。hide は構造化した node / via の列挙を制限するが、別の可視リンクの生 snippet や head に偶然現れる同じ文字列の全文検閲まではしない。

`mdhop query --file Plan.md --relations twohop --via note:topics/設計方針.md --include-head 1 --include-snippet 0 --format json` では、`Guide.md` が 7 行目で `[[設計方針]]` を参照し、本文先頭が `# ガイド` なら、関連部分は次の形になる。`--link-key` を追加しても twohop のこの snippet は残る。

```json
{
  "entry":{"type":"note","name":"Plan","path":"Plan.md","exists":true},
  "2hoplink":[{"type":"note","name":"Guide","path":"Guide.md","exists":true,
    "relation":[{"type":"note","name":"設計方針","path":"topics/設計方針.md","exists":true,
      "snippet":[{"source_path":"Guide.md","start_line":7,"end_line":7,"lines":["[[設計方針]]"]}]}],
    "hidden_relation":false,"head":["# ガイド"]}],
  "page":{"offset":0,"limit":null,"next_offset":null}
}
```

head / snippet を指定しなければ本文ファイルを読まない。指定して選択後の本文を読む際、ファイル欠落・索引時からの更新は既存の `ErrFileNotFound` / `ErrSourceStale` の扱いを維持する。プレビュー用にページ外の対象を先読みしない。

### graph 出力例

json:
```
{
  "nodes":[
    {"id":1,"type":"note","name":"a","path":"docs/a.md"},
    {"id":2,"type":"note","name":"b","path":"docs/b.md"},
    {"id":7,"type":"phantom","name":"Ghost","path":""}
  ],
  "edges":[
    {"source":1,"target":2,"link_type":"wikilink"},
    {"source":1,"target":7,"link_type":"wikilink"}
  ]
}
```

dot:
```
digraph mdhop {
  n1 [label="docs/a.md"];
  n2 [label="docs/b.md"];
  n7 [label="(phantom) Ghost"];
  n1 -> n2;
  n1 -> n7;
}
```

## 制約と非目標

- コードフェンス/インラインコード内の誤検出は最小限に抑止する
- DB に本文は保持しない（位置情報のみ保持）
- 生成物の手編集は行わない（生成ロジックを修正する）
