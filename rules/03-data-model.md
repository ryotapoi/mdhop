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
  exists    INTEGER NOT NULL DEFAULT 1,
  mtime     INTEGER
);

CREATE INDEX idx_nodes_type_name ON nodes(type, name);
CREATE INDEX idx_nodes_path ON nodes(path);

CREATE TABLE edges (
  id          INTEGER PRIMARY KEY,
  source_id   INTEGER NOT NULL,
  target_id   INTEGER NOT NULL,
  link_type   TEXT NOT NULL,
  raw_link    TEXT NOT NULL,
  subpath     TEXT,
  line_start  INTEGER,
  line_end    INTEGER,
  FOREIGN KEY(source_id) REFERENCES nodes(id),
  FOREIGN KEY(target_id) REFERENCES nodes(id)
);

CREATE INDEX idx_edges_source ON edges(source_id);
CREATE INDEX idx_edges_target ON edges(target_id);
CREATE INDEX idx_edges_source_target ON edges(source_id, target_id);
```

### 1.2 meta テーブル（v0.6.0）

frontmatter メタデータを格納するテーブル。YAML リストは値ごとに 1 行展開する。

```sql
CREATE TABLE meta (
  id         INTEGER PRIMARY KEY,
  node_id    INTEGER NOT NULL,
  key        TEXT NOT NULL,
  value      TEXT NOT NULL,
  sort_value TEXT,
  value_type TEXT,
  FOREIGN KEY(node_id) REFERENCES nodes(id)
);

CREATE INDEX idx_meta_node_id ON meta(node_id);
CREATE INDEX idx_meta_key_sort_value ON meta(key, sort_value);
```

カラム設計:
- `value`: frontmatter の生値（表示・LIKE 検索用）
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
| number | sign prefix + 零埋め整数部 20 桁 + 小数部 8 桁。正=`1` prefix、負=`0` prefix + 9 の補数 |
| semver | `v` 除去 + 各セグメント 5 桁零埋め（例: `1.2.3` → `00001.00002.00003`） |
| ordered | `mdhop.yaml` の定義順インデックスを 5 桁零埋め（例: 3 番目 → `00003`） |

正規化失敗時: 元の値をそのまま sort_value に使う（string フォールバック）。インデックス更新系コマンド（build/add/update）で警告を出力する。

---

## 2. ノード（nodes）

### 2.1 Node 種別

- `note`: 実ファイル（Vault相対パスを持つ）
- `phantom`: ファイルが存在しないリンク先
- `tag`: `#tag`（frontmatter tags 含む）
- `url`: 外部URL（任意機能）

### 2.2 一意性キー

- note: `path`（Vault相対）で一意
- phantom/tag/url: 正規化した `name`（または node_key）で一意

### 2.3 推奨カラム（v2を踏襲しつつ拡張余地）

- `node_key`（UNIQUE）: `note:path:folder/A.md` 等の正規化キー
- `name`: 表示名（noteは basename、tagは #付き、phantomはリンク名）
- `path`: noteのみ（phantom/tag/urlはNULL）
- `type`: `note|phantom|tag|url`
- `exists`: noteのみ意味を持つ（phantom/tag/urlは0固定でも可）
- `mtime`: noteのみ（stale判定/差分更新に使用）

---

## 3. エッジ（edges）

### 3.1 基本

- 有向: `source(note) -> target(node)`
- `link_type`: `wikilink | markdown | tag | frontmatter | url`

### 3.2 occurrence（同一ターゲットの複数出現）

- 1ファイル内で同一ターゲットが複数回出現する場合があるため、基本は “出現ごと” にレコードを持つ
- これにより `--include-context` の精度が上がる

### 3.3 位置情報（context抽出用）

- DBには “位置情報（行番号）” のみを保存し、本文は保存しない
- 実装上の注意:
  - 行番号は編集でズレるため、**context返却時に mtime を比較して stale を検知**
  - stale の場合は:
    - そのファイルのみ自動updateして位置情報を再生成（推奨）
    - または context を省略/フォールバック検索

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
- 出力時: 最深のみ（祖先は省略）

### 4.3 2 Hop Links（共通ターゲット方式）

定義:
- `A -> X` かつ `B -> X` を満たす B を 2hop とする
- X は `note|phantom|tag|url(任意)` を含む
- A/B は原則 note（sourceになれるのは実ファイルのみ）

phantom クエリ用 seed:
- outbound: `targets(A)`
- inbound: `sources(A)`（= backlinks）
- auto: noteなら outbound、phantomなら inbound

ノイズ対策:
- 上限で切る（max_*）
- via の degree が大きすぎるものを除外できる（via_max_degree）

### 4.4 メタデータフィルタ（--where）

SQL 生成パターン:
- 同一キーの条件 = OR（1 つの `SELECT m.node_id FROM meta m WHERE m.key = ? AND ...` にまとめる）
  - 例外: `&&` 構文（1 つの `--where` 内で ` && ` 区切り）で指定された条件は同一キーでも AND。各条件を個別サブクエリにして `INTERSECT` で結合する
- 異なるキーの条件 = AND（各キーの subquery を `INTERSECT` で結合）
- フィルタ適用: backlinks/outgoing/twohop の結果ノードに `AND n.id IN (...)` を付加

演算子と対象カラム:
- `=`: `sort_value` で完全一致
- `~`（LIKE）: `value` で部分一致
- `>`, `<`, `>=`, `<=`: `sort_value` で比較 + `value_type` ガード（型宣言済みキーのみ意味のある比較が可能）
- `!=`: `NOT IN` subquery
- EXISTS（演算子なし）: `key` 存在チェック

演算子・構文の詳細は overview.md 参照。

注意: phantom/tag/asset ノードは meta テーブルにエントリを持たないため、`--where` 指定時に常にフィルタアウトされる。

### 4.5 メタデータ取得

`--fields meta` で起点ノードの全 frontmatter メタデータを返す（opt-in。挙動は overview.md 参照）。

SQL パターン: `SELECT key, value, sort_value, value_type FROM meta WHERE node_id = ? ORDER BY key, value`

---

## 5. “Shortest path” と曖昧性制御（DB視点）

### 5.1 曖昧解決ポリシー

- `note_resolution.ambiguous`:
  - `error`（デフォルト）: 候補一覧を返し、静かに誤解決しない
  - `lexicographic`
  - `shortest_path`

### 5.2 needs-path の導出

- basename = `name`（noteの場合）に対して
- `COUNT(note where name=basename and exists=true)` が 2以上なら path必須
  - **例外**: ルート直下にそのファイルがある場合、`[[basename]]` はルートファイルに解決されるため path 不要

この導出は DB から可能なので、固定的な lockfile は必須ではない。
ただし “例外ルール（常にpath必須）” を入れたい場合は設定として上書きできるようにしてもよい。

### 5.3 reconcile（衝突遷移 1→2）に必要な情報

- “衝突前” に basename が一意だったこと（preCount=1）
- “衝突後” に複数になったこと（postCount>=2）
- 衝突前の一意先（incumbent）を特定できること
  - 差分更新の中で preCount を観測する（フック不要でも可能）

---

## 6. スニペット系（Cosense風の把握のため）

### 6.1 include-content（ノート冒頭）

- 各ノートの先頭 N 行を返す（Nは設定/オプション）
- phantom/tag は content 無し

### 6.2 include-context（リンク周辺）

- DBの位置情報を使い、リンク周辺 N 行を返す
- scope/pick/max で制御
  - backlinksだけ / twohopも / 全部
  - first/last/all
  - ペアあたり最大N件

---

## 7. 今後の拡張ポイント

- URLノードの正式対応（現在は任意）
- 相対パスのより高度な扱い（`./` / `../`）
- 書き換え系（mutate）を “安全装置つき” で拡張
- alias / 表示テキストで検索できる `find` 系の追加
- メタデータの拡張（集計クエリ、メタデータベースのソート）
