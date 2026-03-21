# Backlog

## v0.6.0: frontmatter メタデータの SQLite 格納と構造化クエリ

目的: Coding Agent のコンテキスト管理コスト削減。管理情報を SQLite に集約し、CLI 経由でアクセスパターンを制限する

### 設計判断

- DB スキーマ: EAV（`meta` テーブル）。`meta(node_id, key, value, sort_value, value_type)`
- 型システム: 設定ファイル（`mdhop.yaml`）で宣言。宣言なし = string。ランタイムの自動推定はしない
- サポート型: string（デフォルト）, number, date, semver, ordered（ユーザー定義順序）
- sort_value 列: 型ごとに正規化した比較用値を格納。SQLite の辞書順比較で全型の `>` `<` が動く
- 正規化: date は ISO8601 ゼロパディング、number はゼロパディング、semver は `v` strip + 各セグメント 5 桁、ordered は配列インデックス、string はそのまま
- 正規化失敗時: string フォールバック + build 時に警告
- tags 二重管理: 共存（edges + meta）。グラフ探索とフィルタで目的が異なる
- parseLinks 返り値: `parseResult` struct に変更。一貫性重視
- ネスト YAML: 第一階層のみ。Agent ユースケースではフラットな値が主
- --where 構文: `key=value`, `key!=value`, `key~value`(LIKE), `key>value`, `key<value`, `key>=value`, `key<=value`, `key`(EXISTS)。同キー=OR, 異キー=AND

### 設定例

```yaml
# mdhop.yaml
meta:
  types:
    date: date
    created: date
    modified: date
    updated: date
    due: date
    deadline: date
    priority: number
    weight: number
    order: number
    version: semver
    severity:
      ordered: [low, middle, high, critical]
```

### タスク（作業順）

1. [x] `parseLinks` の返り値を `parseResult` struct に変更
   - リファクタリング（機能変更なし）。全呼び出し箇所（13+）を `.Links` アクセスに変更
2. [x] `parseFrontmatter` の全キー対応
   - `FrontmatterEntry{Key, Value, Line}` 追加。第一階層スカラー + リスト（要素展開）。ネストマップはスキップ
   - `parseResult` に `Meta` フィールド追加
3. [x] 型システム + 設定読み込み
   - `mdhop.yaml` の `meta.types` セクションの読み込み（config.go）
   - 型ごとの sort_value 正規化ロジック（string, number, date, semver, ordered）
   - 正規化失敗時の string フォールバック + 警告
4. [x] `meta` テーブル + DB ヘルパー
   - `CREATE TABLE meta (id, node_id, key, value, sort_value, value_type)` + インデックス
   - `insertMeta`, `deleteMetaByNode`, `queryMetaByNode` ヘルパー
5. [x] Build で meta 格納
   - `parsedFile` struct に meta 追加、型設定に従って sort_value を生成、挿入
6. [x] Update / Add / Delete で meta 対応
   - Update: deleteMetaByNode → 再挿入。Delete: removeOrPhantomize で meta 削除
7. [x] `--where` クエリ（core 層）
   - `WhereClause` パーサー（`=`, `!=`, `~`, `>`, `<`, `>=`, `<=`, EXISTS）
   - `QueryOptions.Where`、SQL 生成（sort_value 列で比較）、各 query 関数に meta フィルタ統合
8. [x] CLI `--where` + meta 出力
   - `--where` フラグ（multiString）、`--fields meta` 対応（JSON/text）
9. [x] `init-meta` コマンド
   - `--preset`: 推奨型定義（date×10, number×4, semver×1）を stdout に出力
   - `--scan`: Vault 全 frontmatter を走査し型を推定（80% 閾値、コメント付き）
   - `--write`: `mdhop.yaml` に直接書き込み（既存設定とマージ、既存キーは保持）
   - `--no-comment`: コメント省略（Agent 向け）
   - `--preset --scan` 併用: scan 優先（データドリブン > curated）
10. [ ] ドキュメント更新
    - rules/overview.md, 03-data-model.md, 02-requirements.md

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
