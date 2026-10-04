# mdhop データモデル・詳細資料（現段階）

この資料は、現段階で固まっている「ノード/エッジ」「解決とクエリ」「位置情報とスニペット」の設計をまとめる。

---

## 1. DB 全体方針（SQLite）

- インデックス形式: SQLite（ローカル）
- DBには Markdown の本文TEXTを保存しない
  - `--include-head` / `--include-snippet` は、クエリ時にファイルから読み出して返す
- DBは “最小の正規化されたグラフ” を持ち、クエリで整形して返す

### 1.1 初版スキーマ（ドラフト）

以下は初期実装の最小スキーマ案。変更時は DB を再生成する前提。

```sql
CREATE TABLE nodes (
  id        INTEGER PRIMARY KEY,
  node_key  TEXT NOT NULL UNIQUE,
  type      TEXT NOT NULL,
  name      TEXT NOT NULL,
  path      TEXT,
  exists_flag INTEGER NOT NULL DEFAULT 1,
  mtime     INTEGER,
  lines     INTEGER  -- note の行数（build/update 時に確定、frontmatter 含む全体）。note 以外は NULL
);

CREATE INDEX idx_nodes_type_name ON nodes(type, name);
CREATE INDEX idx_nodes_path ON nodes(path);

CREATE TABLE edges (
  id              INTEGER PRIMARY KEY,
  source_id       INTEGER NOT NULL,
  target_id       INTEGER NOT NULL,
  link_type       TEXT NOT NULL,
  raw_link        TEXT NOT NULL,
  frontmatter_key TEXT, -- frontmatter の YAML key。本文 link は NULL
  reference_target TEXT, -- markdown_reference の定義 destination 原文（wrapper/title 除去）。他は NULL
  in_table        INTEGER NOT NULL DEFAULT 0, -- 本文 GFM 表内の出現か
  subpath         TEXT,
  line_start      INTEGER,
  line_end        INTEGER,
  FOREIGN KEY(source_id) REFERENCES nodes(id),
  FOREIGN KEY(target_id) REFERENCES nodes(id)
);

CREATE INDEX idx_edges_source ON edges(source_id);
CREATE INDEX idx_edges_target ON edges(target_id);
CREATE INDEX idx_edges_source_target ON edges(source_id, target_id);
```

edge の集計値（`outgoing_count` / `incoming_count`）は nodes に列を持たず、edges からの実行時集計で算出する。counts は edges の派生であり常に edges と一致させるため、lines（edges から導出できないファイル内容の事実）とは異なり永続化しない。

スキーマ変更時は既存インデックスを移行せず、`mdhop build` で再生成する。`PRAGMA user_version = 1` を現在の link 解釈 version として保存する。異なる version は読み取り・update・DB 利用 mutation の入口で明示的に拒否し、`mdhop build` を案内する。列の有無だけで旧解釈を受け入れない。`frontmatter_key`、`reference_target`、`in_table` は build/update/add/move の各エッジ再解析時に保存される。旧 schema は読み取り・更新時に `build` が必要というエラーになる。

`markdown_reference` の `raw_link` は使用箇所の原文、`reference_target` は wrapper/title を除き fragment を含む未復号の定義 destination 原文。promotion / 非rewrite guard は共通 Markdown destination parser に原文を一度だけ渡す。decoded target は raw と混ぜず、resolver で再復号しない。`in_table` は原文を再解釈する promotion / rewrite に表文脈を渡す。source/target/subpath/line は使用箇所の関係と位置を保持する。本文・未使用定義・title・定義専用 table は保存しない。exact raw resolve は source と原文の一致する全 edge を読み、target/subpath が同じなら解決し、異なるなら曖昧エラーにする。query は保存 edge を読み、add/move の phantom promotion は保存 destination を再解決する。定義の lookup は index 更新時のみ行う。

### 1.2 meta テーブル（v0.6.0）

frontmatter メタデータを格納するテーブル。YAML リストは値ごとに 1 行展開する。

```sql
CREATE TABLE meta (
  id         INTEGER PRIMARY KEY,
  node_id    INTEGER NOT NULL,
  key        TEXT NOT NULL,
  value      TEXT NOT NULL,
  line       INTEGER NOT NULL,
  sort_value TEXT,
  value_type TEXT,
  FOREIGN KEY(node_id) REFERENCES nodes(id)
);

CREATE INDEX idx_meta_node_id ON meta(node_id);
CREATE INDEX idx_meta_key_sort_value ON meta(key, sort_value);
```

カラム設計:
- `value`: frontmatter の生値（表示・LIKE 検索用）
- `line`: 値の開始位置（ファイル全体で 1 始まり）。値と同じ build/update 時点の snapshot から保存する
- `sort_value`: 型ごとに正規化された比較用文字列（→ 1.3 参照）
- `value_type`: 型名（string/date/number/semver/ordered）。比較演算子の型ガードに使用

インデックス設計:
- `idx_meta_node_id`: ノード単位の削除・取得用
- `idx_meta_key_sort_value`: `--where` フィルタ用

### 1.3 sort_value 正規化

目的: 文字列辞書順で型安全な大小比較を実現する。

5 型の正規化ルール:

| 型 | 正規化 |
|---|---|
| string | そのまま |
| date | ISO 8601 文字列にパース（複数のレイアウトを許容） |
| number | 小数部の末尾 0 を除去し、整数部を 20 桁、小数部を最小 8 桁へ零埋めする。有効桁は丸めず保持。正=`1` prefix + 整数部 + `.` + 小数部、負=`0` prefix + 整数部・小数部の 9 の補数 + 終端 `:` |
| semver | `v` 除去 + 各セグメント 5 桁零埋め（例: `1.2.3` → `00001.00002.00003`） |
| ordered | `mdhop.yaml` の定義順インデックスを 5 桁零埋め（例: 3 番目 → `00003`） |

正規化失敗時: 元の値をそのまま sort_value に使う（string フォールバック）。インデックス更新系コマンド（build/add/update）で警告を出力する。

number の負数終端 `:` は全数字より辞書順で後に置かれ、小数部の桁数が異なる場合も数値順と辞書順を一致させる。末尾 0 の有無で等値は変わらず、符号付き 0 は正の 0 に揃える。整数部の上限は入力 20 桁で、科学表記は非対応。旧 number 正規化形式の index は `mdhop build` で再生成する必要がある。旧形式の保存値と新形式の検索条件を混在させない。

---

## 2. ノード（nodes）

### 2.1 Node 種別

- `note`: 実 Markdown ファイル（Vault相対パスを持つ）
- `asset`: 実ファイルの asset（Vault相対パスを持つ）
- `phantom`: ファイルが存在しないリンク先
- `tag`: `#tag`（frontmatter tags 含む）

### 2.2 一意性キー

- note: `note:path:<Vault相対path>` の `node_key` で一意
- asset: `asset:path:<Vault相対path>` の `node_key` で一意
- phantom/tag: 正規化した `name` を含む `node_key` で一意
- path / node_key / basename key は NFC 正規化した表現で保持・比較する。既存 index に NFD path が残っている場合の完全移行は `build` による再生成で行う

### 2.3 推奨カラム（v2を踏襲しつつ拡張余地）

- `node_key`（UNIQUE）: `note:path:folder/A.md`、`asset:path:images/A.png` 等の正規化キー
- `name`: 表示名（noteは basename、tagは #付き、phantomはリンク名）
- `path`: note/asset の Vault相対パス（phantom/tag は NULL）
- `type`: `note|asset|phantom|tag`
- `exists_flag`: DB列。note/asset の存在状態を持ち、phantom/tag は 0
- `mtime`: note/asset の更新時刻（stale判定/差分更新に使用）
- `lines`: note のみ。asset を含む他の種別は本文・行数を持たない

公開 JSON の存在状態は `exists` であり、DB列名の `exists_flag` とは区別する。

---

## 3. エッジ（edges）

### 3.1 基本

- 有向: `source(note) -> target(node)`
- `link_type`: `wikilink | markdown | markdown_reference | tag | frontmatter | frontmatter_wikilink | frontmatter_path`
  - `frontmatter_wikilink`: Obsidian property link と同様、**引用符で囲まれた YAML scalar / list item 値**に現れた `[[...]]`（`tags` キー以外。double quote / single quote）
    - bare `key: [[Note]]` と bare list item `- [[Note]]` は YAML 上の nested sequence であり edge 化しない
    - block scalar（`key: |` / `key: >`）内の `[[...]]` も対象外
  - `frontmatter_path`: `meta.link_keys` で宣言された key の raw path 値（URL・wikilink 値は除く）

### 3.2 occurrence（同一ターゲットの複数出現）

- 1ファイル内で同一ターゲットが複数回出現する場合があるため、基本は “出現ごと” にレコードを持つ
- これにより `--include-snippet` の精度が上がる

### 3.3 位置情報（context抽出用）

- DBには “位置情報（行番号）” のみを保存し、本文は保存しない
- 実装上の注意:
  - 行番号は編集でズレるため、**context返却時に mtime を比較して stale を検知**
  - stale の場合は `ErrSourceStale` を返す。query は自動 update せず、利用者が `mdhop update` 等でインデックスを更新する

> 書き換え（mutate）用途では、位置情報に依存せず「該当ファイルを再パースして置換」すればよい。
> 位置情報はあくまで “スニペット抽出のキャッシュ” として位置づける。

### 3.4 raw_link / subpath の扱い（候補）

- `subpath`（`#Heading` / `#^block`）は resolve 結果として返すが、DB上は:
  - occurrenceごとに raw_link を持つ
  - または `subpath` カラムを追加
  のどちらでもよい
- 重要なのは「alias/subpath/embed を壊さずに扱えること」

---

## 4. 主要クエリの考え方

### 4.1 backlinks

- `B -> A` を持つ B を返す
- SQL: `SELECT source_id FROM edges WHERE target_id = :A`

### 4.2 tags（ノートが持つタグ）

- `A -> #tag` の target 群を返す
- SQL例: `SELECT target_id FROM edges WHERE source_id=:A AND link_type='tag'`

ネストタグの扱い:
- 格納時: `#a/b/c` を見つけたら `#a`, `#a/b`, `#a/b/c` へ全てエッジ
- inspect の tags 表示時: 最深のみ（祖先は省略）
- query の outgoing / twohop 経由候補: 索引済み親タグも含める

### 4.3 2 Hop Links（共通ターゲット方式）

定義:
- `A -> X` かつ `B -> X` を満たす B を two-hop とする
- X は `note|asset|phantom|tag` を含む
- A/B は原則 note（sourceになれるのは実ファイルのみ）

全入口で outbound（`targets(A)`）のみを使う。tag / asset / phantom は source edge を持たないため twohop は空となる。inbound / auto の特例は設けない。

対象を typed node ID でまとめて全経由先を返す。via 条件で経由を選び、hide は発見後の表示だけを変える。一関係の有効対象を安定順にページ取得し、本文を読む前に返却対象を確定する。count / total や実行間の index 同時更新保証は設けない。詳細は `docs/specs/overview.md` 参照。

### 4.4 メタデータフィルタ（--where）

SQL 生成パターン:
- 複数 `--where` フラグ = AND。キーの一致・不一致を問わず、各フラグを node_id subquery にして `INTERSECT` で結合する
- `&&` 構文（1 つの `--where` 内で ` && ` 区切り）で指定された条件 = AND。各条件を個別サブクエリにして `INTERSECT` で結合する
- `||` 構文（1 つの `--where` 内で ` || ` 区切り）で指定された条件 = OR。各条件を個別サブクエリにして `UNION` で結合し、その `--where` 全体を他の `--where` 条件とは `INTERSECT` で結合する
- 1 つの `--where` 内で ` && ` と ` || ` を混在させた式は、優先順位・括弧解釈を持たないためエラー
- `coalesce(key1, key2, ...)` は 1 条件として上記の AND / OR 結合に参加する。SQL では優先順ごとに branch を作り、低優先キーの branch には高優先キーが同一 node に存在しないことを `NOT EXISTS` でガードする。各 branch の `value_type` ガードはそのキー自身の `meta.types` 宣言を使う（相対日付使用時は全キー共通で `date` 型を強制）
- フィルタ適用: backlinks/outgoing/twohop の結果ノードに `AND n.id IN (...)` を付加

演算子と対象カラム:
- `=`: `sort_value` で完全一致
- `~`（LIKE）: `value` で部分一致
- `>`, `<`, `>=`, `<=`: `sort_value` で比較 + `value_type` ガード（型宣言済みキーのみ意味のある比較が可能）
- `!=`: `NOT IN` subquery
- EXISTS（演算子なし）: `key` 存在チェック
- NOT EXISTS: 既存 note のうち `key` を持たないものを返す
- `coalesce(...)` の EXISTS は指定キーのいずれかの存在、NOT EXISTS は指定キーがすべて存在しないことを表す

演算子・構文の詳細は docs/specs/overview.md 参照。

注意: phantom/tag/asset ノードは meta テーブルにエントリを持たないため、`--where` 指定時に常にフィルタアウトされる。

### 4.5 メタデータ取得

inspect は既定で索引済み note 一件の metadata を返し、`--fields meta` で属性を限定できる。query は入口 metadata を返さない。search の `--fields meta` は従来どおり opt-in。値は索引済みの `map[string][]string` とし、詳細は `docs/specs/overview.md` 参照。

SQL パターン: `SELECT key, value, sort_value, value_type FROM meta WHERE node_id = ? ORDER BY key, value`

---

## 5. “Shortest path” と曖昧性制御（DB視点）

### 5.1 曖昧解決

- basename 解決は、候補が一意でなければエラーにする。設定で候補を選択する方針は持たず、静かに誤解決しない
- 同名候補に Vault ルート直下の note があれば、その note を優先して解決する

### 5.2 needs-path の導出

- basename = `name`（noteの場合）に対して
- `COUNT(note where name=basename and exists_flag=1)` が 2以上なら path必須
  - **例外**: ルート直下にそのファイルがある場合、`[[basename]]` はルートファイルに解決されるため path 不要

この導出は DB から可能なので、固定的な lockfile は必須ではない。
ただし “例外ルール（常にpath必須）” を入れたい場合は設定として上書きできるようにしてもよい。

### 5.3 disambiguate（衝突遷移 1→2）に必要な情報

- “衝突前” に basename が一意だったこと（preCount=1）
- “衝突後” に複数になったこと（postCount>=2）
- 衝突前の一意先（incumbent）を特定できること
  - 差分更新の中で preCount を観測する（フック不要でも可能）

---

## 6. スニペット系（Cosense風の把握のため）

### 6.1 `--include-head`（ノート冒頭）

- `--include-head <N>` で query の返却 note / inspect の指定 note の本文先頭 N 行を返す（frontmatter と先頭空行を除く）
- note だけが本文を持つ。asset/phantom/tag は head を持たない

### 6.2 `--include-snippet`（リンク周辺）

- query の `--include-snippet <N>` で DB の edge 出現の位置情報を使い、根拠行と前後 N 行を生行として返す。outgoing は入口、backlinks は対象→入口、twohop は対象→可視 via の本文を読む
- preview 未指定では本文を読まず、指定時も hidden / ページ外 / 先読みの不要本文は読まない
- head は `--include-head`、snippet は `--include-snippet` を指定したときだけ返す

---

## 7. 今後の拡張ポイント

- 相対パスのより高度な扱い（`./` / `../`）
- 書き換え系（mutate）を “安全装置つき” で拡張
- alias / 表示テキストで検索できる `find` 系の追加
- メタデータの拡張（集計クエリ、メタデータベースのソート）
