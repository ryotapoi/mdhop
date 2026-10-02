# mdhop 要件資料

## 1. 対象と前提

- 対象: Obsidian Vault 相当のディレクトリ配下の `**/*.md`
- 主要利用者: Coding Agent（Claude Code / Codex など）および VSCode 等の補助スクリプト
- 目的: grep を乱用せず、リンクグラフに基づき関連ノートへ効率的にアクセスする

---

## 2. 機能要件

### 2.1 インデックス構築

- Vault 全体を解析し、SQLite にインデックスを作る
- 解析対象:
  - wikilink（alias / heading / block 含む）
  - markdown link（相対/絶対/URL任意）
  - Markdown 参照リンク（full/collapsed/shortcut、同一文書内の定義で解決）
  - tag（本文 + frontmatter tags）
  - frontmatter 内の引用符付き wikilink（`tags` 以外のキー）と、`meta.link_keys` で指定したキーの raw path 値
  - frontmatter メタデータ（scalar 値と scalar 配列要素を meta テーブルに格納。null・マッピングはスキップ。型宣言に基づき sort_value を正規化）
- 型設定: `mdhop.yaml` の `meta.types` で frontmatter キーの型を宣言（5 型: string, number, date, semver, ordered）。形式は docs/specs/overview.md 参照
- 誤検出対策（最低限）:
  - backtick / tilde の同種 delimiter で始まり、開始時以上の連続数の delimiter で閉じるコードフェンスの内部、およびインラインコード内のリンク・tag・heading 抽出を抑止
  - 見出し `# Heading` を tag として扱わない
  - URL の `#fragment` を tag と誤認しない
  - `http(s)`, `mailto`, `ftp`, および `scheme://` 形式の外部 URI を内部リンクとして扱わない。opaque な `foo:bar` は内部名として扱う
- phantom:
  - 解決不能な内部リンクを phantom node として保持し、`note -> phantom` を張る

### 2.2 差分更新

- `update --file ...` で指定ファイルのみ再解析してDBに反映
- ディスクから消えたファイルを指定した場合は delete と同じ扱い:
  - 参照あり → phantom 変換（outgoing edges 削除、ノードを phantom に変換）
  - 参照なし → ノード完全削除
- meta テーブルも差分更新対象（update: 全削除→再挿入、add: 挿入、delete: 削除）

### 2.3 リンク解決（resolve）

入力:
- `from_note`（Vault相対パス）
- `link`（`[[...]]` / `[]()` / Markdown 参照原文 / `#tag` / `https://...`）

出力:
- `type`: `note | phantom | tag | url | asset`
- `name`: 表示名
- `path`: note / asset の Vault 相対パス（それ以外の type では出力しない）
- `exists`: note / asset の存在フラグ
- `subpath`: `#Heading` / `#^blockId`（あれば）

解決ルール:
- `[[Note]]`: basename一致を Vault 全体から探索
  - 候補1件 → OK
  - 候補複数 → 曖昧としてエラー。ただし Vault ルート直下に対象がある場合はそのファイルに解決する
- `[[path/to/Note]]`: Vaultルート相対パスとして解決（拡張子省略可）
- `[[./Note]]`, `[[../Note]]`: `from_note` のディレクトリ基準で解決
- Markdown link:
  - `/` 始まり: Vault ルート相対
  - `./` / `../` 始まり: `from_note` 基準
  - `/` を含むがプレフィックスなし（例: `sub/C.md`）: パスとして解決
  - `/` を含まない（例: `Design.md`）: basename 解決（`[[note]]` と同一扱い）

- Markdown 参照リンクは、使用原文に一致する source の保存 edge から解決する。JSON は通常 link と同じ shape。未定義・未索引の使用原文はエラーとなる。定義変更は update 後に反映し、query/resolve は本文を再走査しない。記法の詳細と非 rewrite 制限は `docs/specs/overview.md` を正本とする

### 2.4 ノート取得（query）

- 指定ノート（`--file`）の以下を返す:
  - `Backlinks`
  - `Tags`
  - `2 Hop Links`（共通ターゲット方式）
- phantom をクエリ対象に含める
  - phantom クエリ時の two-hop seed は inbound/auto をサポート
- 出力順/ノイズ対策:
  - priority（backlink > tags > two-hop(link) > two-hop(tag deep) > two-hop(tag shallow)）
  - 上限 (`max_backlinks`, `max_twohop`, `max_via_per_target`) で切る
- `--link-key <key>` は direct な backlinks / outgoing を指定 frontmatter key 由来のリンクに限定する

- メタデータフィルタ（`--where`）:
  - frontmatter の値によるノードフィルタリング
  - 複数 `--where` フラグは常に AND。OR は 1 つの `--where` 内の ` || ` 区切りで明示する。1 つの `--where` 内では ` && ` 区切りで AND（日付範囲等）、` || ` 区切りで OR。` && ` と ` || ` の混在は未対応でエラー
  - 比較演算子は型宣言済みキーで型安全な比較（sort_value ベース）
  - 演算子・構文は docs/specs/overview.md 参照

### 2.5 省コンテキスト出力

- `--format json|text`
- `--fields`: 出力フィールド選択（未知指定はエラー）
- `--include-head`: ノート冒頭 N 行を返す
- `--include-snippet`: リンク周辺 N 行を返す
  - DBには本文TEXTを保存しない（位置情報のみ）
  - query 時にファイルから切り出す
  - stale（mtime不一致）または保存済みのリンク位置が現在のファイル行数を超える場合はエラーにする

### 2.6 diagnose（事故検出）

- basename 衝突一覧
- phantom 一覧（未解決リンク）
- `--fields anchors` で見出しアンカー切れ検出（opt-in）

### 2.7 ノート検索（search）

- 起点不要の全ノード検索。vault 全体から条件に合致する note を返す
- `--where` でメタデータフィルタ（query と同じ構文・演算子）
- `--path` でパス glob 包含フィルタ、`--exclude` でパス glob 除外フィルタ
- `--sort` で meta キーによるソート（昇順/降順）。NULL は常に末尾
- `--limit` / `--offset` でページング。total フィールドに適用前の総件数を返す
- `--fields meta` で frontmatter メタデータを追加出力（opt-in）
- `--include-head N` でノート先頭N行を追加出力
- コマンド詳細は docs/specs/overview.md 参照

### 2.8 型スキャフォールディング（init-meta）

- frontmatter メタデータの型定義（`mdhop.yaml` の `meta.types`）を自動生成
- プリセット出力 + Vault スキャンによる型推定
- DB 不要（ファイル走査ベース）。build 前に実行可能
- コマンド詳細は docs/specs/overview.md 参照

### 2.9 インデックス状態の確認（status）

- 未登録・変更済み・削除済みの note / asset を、索引やファイルを変更せずに一覧する

### 2.10 frontmatter 診断

- `meta-check` は、`build.exclude_paths` により索引・本文解析の対象外である実在ファイルも参照先候補として検査できる
- 各 finding は source の行位置を返し、既存 index で行情報が必要な場合は再 build を要求する

---

## 3. “Shortest path” 制御（basename衝突の扱い）

### 3.1 原則

- basename が Vault 内で一意 → `[[basename]]` が推奨
- basename が複数 → `[[path/to/basename]]` が必須（曖昧性排除）
  - **ルート優先例外**: Vault ルート直下にそのファイルがある場合、`[[basename]]` はルートファイルに解決される（曖昧ではない）
- 書き換え後のリンクは Vault 相対パスに統一する（`./` / `../` プレフィックスは使わない）
  - **move の例外**: 移動する note 自身に既にある `./` / `../` 起点の相対リンクは、移動後の note から同じ参照先（参照先も移動する場合はその移動後の位置）を指す相対形式を保つ。相対表記が変わらなければ書き換えない

### 3.2 衝突発生（1→2）時の意味保存リライト（disambiguate）

目的:
- 追加/リネームによって basename が複数になった瞬間に、
  それまで `[[basename]]` が指していた “旧一意先” を維持する

要件:
- インクリメンタル（差分）更新を前提に、衝突の遷移を検知できること
- リライト対象の source ファイル集合を DB から引けること（grep全走査を避ける）
- 書き換えは Obsidian互換を壊さない（alias/subpath/embed保持、コード内除外）
- ルート優先例外との相互作用: 旧一意先がルート直下の場合、basename リンクはルートに解決し続けるためリライト不要

### 3.3 “短く戻す” 正規化（simplify）

- basename が再び一意になった場合に `[[basename]]` へ戻すことは可能
- ただし Git差分が揺れるため、デフォルトOFFでよい（手動コマンド/CIガードで運用）

---

## 4. 非機能要件

- 安全性:
  - mutate系は Vault 外への書き込み・削除や意図しない上書きを拒否する。コマンドごとの事前条件は `docs/specs/overview.md` に従う
  - 曖昧解決はデフォルト error（静かに誤解決しない）
- 性能:
  - build は Vault 全体走査
  - update は指定ファイルのみ
  - query/resolve は DB + 必要なら限定的なファイル読み（context用）
- 移植性:
  - ローカルSQLiteで完結
  - OS依存のパス表現は Vault 相対パスに正規化して出力
- Git運用:
  - DBは通常コミットしない（Vault 直下の `.mdhop/` を ignore）
  - 生成物は再現可能にする（設定とバージョンをメタに保持）
