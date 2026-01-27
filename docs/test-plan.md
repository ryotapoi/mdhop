# テスト設計表（軽量版・厳密モード）

コマンドごとに「必要なテスト」を箇条書きで整理する。
詳細な入力/期待結果/テストデータは各コマンド実装前に拡張する。

## build

- 正常系: Vault内の全Markdownを解析してDBを作成
- 失敗系: 曖昧リンクが1件でもあればエラー
- 失敗系: 失敗時は成果物を消すが、既存DBは残る
- 解析対象: `**/*.md` のみ
- `.mdhop/` 配下は除外

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

## query

- 起点: `--file/--tag/--phantom/--name` のいずれか
- `--name #tag` はタグ扱い、曖昧ならエラー
- `backlinks/tags/outgoing/twohop` の出力
- `twohop` は via→targets 構造
- `backlinks/outgoing/twohop` は type を含める
- `--fields` による出力制限（未知値はエラー）
- `--format text/json` の出力差
- `--include-head/--include-snippet` の出力
- stale（mtime不一致）検出でエラー
- `max-*` の上限適用

## add

- 未登録の新規追加は成功
- 既存パス指定はエラー
- 追加ファイル内に曖昧リンクが含まれる場合はエラー
- `--auto-disambiguate` あり: 既存リンクの書換えが行われる
- `--auto-disambiguate` ありでも、追加ファイルに曖昧リンクが含まれる場合はエラー

## update

- 登録済みのみ更新可能
- 削除済みファイル指定時の扱い（exists=false）
- 曖昧リンクを追加した更新はエラー
- stale検出（include-head/snippet時）でエラー

## move

- 旧パス未登録はエラー
- 新パス既存はエラー
- basename衝突が発生する移動はエラー
- `[[a]]` / `[x](a.md)` は一意なら維持、曖昧化/別解決なら書換え
- `[[path/to/a]]` / `[x](path/to/a.md)` は必ず書換え
- リンク書換え対象ファイルはDBから抽出

## delete

- 登録済みのみ削除可能
- 参照あり: phantom化
- 参照なし: ノード削除

## diagnose

- basename衝突一覧
- phantom一覧

## stats

- notes_total / notes_exists / edges_total / tags_total / phantoms_total

## disambiguate

- `--name` 必須
- 候補複数時は `--target` 必須
- `--file` 指定時は対象ファイルのみ書換え
- `[[a]]` / `[x](a.md)` を対象
- `[[path/to/a]]` の同一対象も対象
- `--scan` は将来対応
