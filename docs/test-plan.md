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
- `--auto-disambiguate` あり: 既存リンクの書換えが行われる
- `--auto-disambiguate` ありでも、追加ファイルに曖昧リンクが含まれる場合はエラー
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
- コラテラル書き換え: 対象ファイルが stale → エラー
- outgoing basename 書き換え: 移動ファイルの basename リンクが曖昧化 → フルパスに書き換え
- outgoing basename 書き換え: ルート優先で意味が変わる場合も書き換え
- ルート優先: move前後でルートファイルが存続 → コラテラル書き換え不要
- ルート優先: basename不変 + ルート存続 → incoming rewrite スキップ
- `[[a]]` / `[x](a.md)` は一意なら維持、曖昧化/別解決なら書換え
- `[[path/to/a]]` / `[x](path/to/a.md)` は必ず書換え
- リンク書換え対象ファイルはDBから抽出

## delete

- 登録済みのみ削除可能
- 参照あり: phantom化
- 参照なし: ノード削除
- `--format` バリデーション（無効値でエラー）
- `--rm`: ファイル存在時にディスクから削除+インデックス更新
- `--rm`: ファイル不在時も成功（冪等）
- `--rm`: 未登録ファイルはエラー、ディスク上のファイルは削除されない
- `--rm`: 参照ありファイルはディスク削除+phantom化

## diagnose

- basename衝突一覧
- phantom一覧

## stats

- notes_total / notes_exists / edges_total / tags_total / phantoms_total

## disambiguate

- `--format` バリデーション（無効値でエラー）
- `--name` 必須
- 候補複数時は `--target` 必須
- `--file` 指定時は対象ファイルのみ書換え
- `[[a]]` / `[x](a.md)` を対象
- `[[path/to/a]]` 等のパス指定リンクは既に明確なため対象外
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
