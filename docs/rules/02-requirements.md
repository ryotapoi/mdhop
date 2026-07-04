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
  - tag（本文 + frontmatter tags）
  - frontmatter 内リンク（設定で指定したキーのみ）
  - frontmatter メタデータ（scalar 値と scalar 配列要素を meta テーブルに格納。null・マッピングはスキップ。型宣言に基づき sort_value を正規化）
- 型設定: `mdhop.yaml` の `meta.types` で frontmatter キーの型を宣言（5 型: string, number, date, semver, ordered）。形式は docs/specs/overview.md 参照
- 誤検出対策（最低限）:
  - コードフェンス/インラインコード内のタグ抽出を抑止
  - 見出し `# Heading` を tag として扱わない
  - URL の `#fragment` を tag と誤認しない
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
- `link`（`[[...]]` / `[]()` / `#tag` / `https://...`）

出力:
- `type`: `note | phantom | tag | url`
- `name`: 表示名
- `path`: note の Vault 相対パス（note以外は null）
- `exists`: note のみ有効
- `subpath`: `#Heading` / `#^blockId`（あれば）

解決ルール:
- `[[Note]]`: basename一致を Vault 全体から探索
  - 候補1件 → OK
  - 候補複数 → “曖昧”として扱い、候補一覧を返す（デフォルトは error）
- `[[path/to/Note]]`: Vaultルート相対パスとして解決（拡張子省略可）
- `[[./Note]]`, `[[../Note]]`: `from_note` のディレクトリ基準で解決
- Markdown link:
  - `/` 始まり: Vault ルート相対
  - `./` / `../` 始まり: `from_note` 基準
  - `/` を含むがプレフィックスなし（例: `sub/C.md`）: パスとして解決
  - `/` を含まない（例: `Design.md`）: basename 解決（`[[note]]` と同一扱い）

### 2.4 ノート取得（query）

- 指定ノート（`--file` or `--note`）の以下を返す:
  - `Backlinks`
  - `Tags`
  - `2 Hop Links`（共通ターゲット方式）
- phantom をクエリ対象に含める
  - phantom クエリ時の two-hop seed は inbound/auto をサポート
- 出力順/ノイズ対策:
  - priority（backlink > tags > two-hop(link) > two-hop(tag deep) > two-hop(tag shallow)）
  - 上限 (`max_backlinks`, `max_twohop`, `max_via_per_target`) で切る
  - ハブ via を避けるオプション（via_max_degree）

- メタデータフィルタ（`--where`）:
  - frontmatter の値によるノードフィルタリング
  - 複数 `--where` の同キー条件 = OR、異キー条件 = AND。1 つの `--where` 内では ` && ` 区切りで AND（日付範囲等）、` || ` 区切りで OR。` && ` と ` || ` の混在は未対応でエラー
  - 比較演算子は型宣言済みキーで型安全な比較（sort_value ベース）
  - 演算子・構文は docs/specs/overview.md 参照

### 2.5 省コンテキスト出力

- `--format json|text`
- `--fields`: 出力フィールド選択（未知指定はエラー）
- `--include-head`: ノート冒頭 N 行を返す
- `--include-snippet`: リンク周辺 N 行を返す
  - DBには本文TEXTを保存しない（位置情報のみ）
  - query 時にファイルから切り出す
  - stale（mtime不一致）なら、そのファイルのみ自動update するか、contextを省略する（実装方針で選択）

### 2.6 diagnose（事故検出）

- basename 衝突一覧
- phantom 一覧（未解決リンク）
- 除外数、パース失敗一覧

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

---

## 3. “Shortest path” 制御（basename衝突の扱い）

### 3.1 原則

- basename が Vault 内で一意 → `[[basename]]` が推奨
- basename が複数 → `[[path/to/basename]]` が必須（曖昧性排除）
  - **ルート優先例外**: Vault ルート直下にそのファイルがある場合、`[[basename]]` はルートファイルに解決される（曖昧ではない）

### 3.2 衝突発生（1→2）時の意味保存リライト（reconcile）

目的:
- 追加/リネームによって basename が複数になった瞬間に、
  それまで `[[basename]]` が指していた “旧一意先” を維持する

要件:
- インクリメンタル（差分）更新を前提に、衝突の遷移を検知できること
- リライト対象の source ファイル集合を DB から引けること（grep全走査を避ける）
- 書き換えは Obsidian互換を壊さない（alias/subpath/embed保持、コード内除外）
- ルート優先例外との相互作用: 旧一意先がルート直下の場合、basename リンクはルートに解決し続けるためリライト不要

### 3.3 “短く戻す” 正規化（canonicalize / shorten）

- basename が再び一意になった場合に `[[basename]]` へ戻すことは可能
- ただし Git差分が揺れるため、デフォルトOFFでよい（手動コマンド/CIガードで運用）

---

## 4. 非機能要件

- 安全性:
  - mutate系は誤爆防止（環境変数や `--yes` の要求など）
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
