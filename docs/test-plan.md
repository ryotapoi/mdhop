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
- basename衝突あり + パス指定リンクのみ → エラーにならない
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
- 統合テスト: note数・phantom数・tag数・edge数の検証
- 冪等性: 2回buildで結果が同一

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

- 登録済みのみ更新可能（未登録ファイルはエラー、DB変更なし）
- コンテンツ変更: 旧edges消滅、新edges生成、mtime更新
- ディスク不在+参照あり → phantom変換
- ディスク不在+参照なし → 完全削除
- ディスク不在+参照あり+同名phantom既存 → edges付替え+note削除
- 曖昧リンクを追加した更新はエラー（DB変更なし）
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

- 旧パス未登録はエラー
- 新パス既存はエラー
- 移動後に曖昧リンクが残る場合はエラー
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
- `[[path/to/a]]` 等のパス指定リンクは既に明確なため対象外
- `--scan` は将来対応
