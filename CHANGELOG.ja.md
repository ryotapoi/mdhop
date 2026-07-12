# 変更履歴

この変更履歴は、プロジェクトの [GitHub Releases](https://github.com/ryotapoi/mdhop/releases) と Git tag を照合して再構成しています。GitHub Release が作成されていない tag 付きバージョンも、tag 間のコミット履歴から記載しています。

## [Unreleased]

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
