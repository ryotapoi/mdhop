# 外部仕様（統合版）

このドキュメントは、ユーザー視点の挙動・互換性・制約・非目標をまとめる。
実装詳細や内部構造は書かず、詳細はテストとコードに寄せる。
必要最小限の説明に留める。
詳細な内部設計は `docs/architecture/` と `docs/adr/` に置く。

## 対象と前提

- 対象は Obsidian Vault 相当のディレクトリ配下の `**/*.md`
- 主な利用者は CLI と Coding Agent
- ローカル完結（SQLite）で動作する

## データ配置と設定

- Vault 直下に `.mdhop/` を作成し、SQLite などの実行時データを配置する
- `.mdhop/` の主なファイルは `index.sqlite` とする
- 将来的に `.mdhop/meta.json` を置く場合は、スキーマバージョンやインデックス作成情報を保持する
- 設定ファイル: Vault 直下の `mdhop.yaml`（YAML 形式）
  - ファイルがなければデフォルト設定（除外なし）で動作する
  - `build` セクション: build 時のファイル除外
  - `exclude` セクション: query 結果のフィルタ

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
```

## コマンドと挙動（厳密モード前提）

- `mdhop build` : Vault 全体を解析しインデックスを作成する
- `mdhop update --file ...` : 登録済みファイルのみを更新する
  - `--file` は複数回指定できる
- `mdhop add --file ...` : 新規追加を反映する（未登録のみ）
- `mdhop move --from A.md --to B.md` : ファイル移動を反映する
- `mdhop delete --file ...` : ファイル削除を反映する（登録済みのみ）
- `mdhop disambiguate --name a` : 曖昧リンクをフルパスへ書き換える
- `mdhop resolve --from A.md --link '[[X]]'` : リンク解決を行う
- `mdhop query --file A.md` : 起点ノートの関連情報を返す
- `mdhop query --tag tag` : タグ起点の関連情報を返す
- `mdhop query --phantom name` : phantom 起点の関連情報を返す
- `mdhop query --name name` : note/phantom/tag を意識せず関連情報を返す
- `mdhop diagnose` : basename 衝突、phantom 一覧を検出する
- `mdhop stats` : ノート数・リンク数などの統計情報を返す

### モード

- 既定は **厳密モード**（曖昧時はエラー）
- 互換モード（Obsidian互換）は将来追加する
  - 詳細は `docs/external/obsidian-compat.md`

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

### resolve/query/diagnose/stats の出力

- `--format json|text` : 出力形式を指定する（default: text）
- `--fields <comma-separated>` : 出力フィールドを制限する
  - resolve: `type,name,path,exists,subpath`
  - query: `backlinks,tags,twohop,outgoing,head,snippet`
  - diagnose: `basename_conflicts,phantoms`
  - stats: `notes_total,notes_exists,edges_total,tags_total,phantoms_total`
    - `edges_total` は出現回数ベースの総数

### フィールド定義

#### resolve

- `type`: `note|phantom|tag|url`
- `name`: 表示名（noteはbasename、tagは`#`付き）
- `path`: Vault相対パス（noteのみ）
- `exists`: noteの存在フラグ
- `subpath`: `#Heading` / `#^block`（あれば）

#### query

- `backlinks`: 起点ノートへリンクしているノート一覧
- `outgoing`: 起点ノートからの外向きリンク一覧
- `twohop`: 共通ターゲット方式の関連ノート一覧（`via` ごとに `targets` を返す）
- `tags`: 起点ノートが持つタグ一覧
- `head`: ノート先頭N行（`--include-head`）
- `snippet`: リンク周辺の前後N行（`--include-snippet`）

#### diagnose

- `basename_conflicts`: basename衝突の一覧
- `phantoms`: phantom名一覧

#### stats

- `notes_total`: note総数
- `notes_exists`: exists=true の note 数
- `edges_total`: edges総数（出現回数ベース）
- `tags_total`: tag総数
- `phantoms_total`: phantom総数

### query の追加オプション

- `--file <path>` : ノート起点
- `--tag <name>` : タグ起点（`#` は任意）
- `--phantom <name>` : phantom 起点
- `--name <name>` : 起点を自動判定（`#tag` はタグ扱い、曖昧ならエラー。ルート優先例外あり）
- `--include-head <N>` : ノート冒頭 N 行を返す（frontmatterを除外し、先頭の空行を全て省く）
- `--include-snippet <N>` : リンク周辺の前後 N 行ずつを返す（合計 2N+1 行）
- `--max-backlinks <N>` : Backlinks の上限（default: 100）
- `--max-twohop <N>` : 2hop の上限（default: 100）
- `--max-via-per-target <N>` : 2hop の共通ターゲットごとの上限（default: 10）
- `--exclude <glob>` : 指定パターンに一致するパスを結果から除外する（複数回指定可）
- `--exclude-tag <tag>` : 指定タグを結果から除外する（複数回指定可、`#` 付き推奨）
- `--no-exclude` : `mdhop.yaml` の除外設定を無視する

### 除外フィルタの仕様

- 適用範囲: query のみ（stats/diagnose は対象外）
- 除外対象: backlinks, outgoing, tags, twohop（via と targets 両方）, snippet
- エントリノード自体は除外されない（`--file daily/D.md --exclude "daily/*"` は正常動作）
- `mdhop.yaml` の `exclude` と CLI の `--exclude`/`--exclude-tag` はマージして適用する
- パス除外の glob パターン:
  - SQLite GLOB 互換。`*` は任意文字列（`/` を含む）にマッチ、`?` は 1 文字にマッチ
  - case-sensitive（`Daily/*` は `daily/D.md` にマッチしない）
  - `[...]` 文字クラスは未サポート（パターンに `[` を含むとエラー）
  - `**` は不要（`*` が `/` にもマッチするため）
- タグ除外: 完全一致、case-insensitive
- twohop の除外: 除外タグ/パスに一致する via はエントリごと削除される

### コマンド詳細（必須/任意）

- `build`
  - 必須: なし
  - 任意: `--vault`
  - 補足: 曖昧リンクが存在する場合は **エラー**（厳密モード）
  - 補足: `mdhop.yaml` の `build.exclude_paths` に一致するファイルはインデックスから除外される
    - 除外ファイルへのリンクは phantom ノードとして扱われる
    - 除外ファイル内のタグはインデックスに含まれない
    - query の `exclude.paths` とは独立（build 除外はインデックス作成前にフィルタ、query 除外はクエリ結果をフィルタ）
- `update`
  - 必須: `--file`（複数回指定可）
  - 任意: `--vault`, `--format`
  - 補足: 更新後の内容に、曖昧リンクが含まれる場合は **エラー**
    - 対象: `[[a]]` / `[x](a.md)` など basename 解決が必要なリンク
- `add`
  - 必須: `--file`（複数回指定可）
  - 任意: `--vault`, `--format`
  - 補足: 既存ファイルが指定された場合はエラー
  - 補足: 追加ファイル内に曖昧リンクが含まれる場合は **エラー**
  - `--auto-disambiguate` : 衝突が発生する場合に、既存リンクを自動でフルパス化して
    **意味を保てる時だけ許可**する（厳密モードでは失敗しない前提）
- `move`
  - 必須: `--from`, `--to`
  - 任意: `--vault`, `--format`
  - 補足: ディスク上のファイル移動も行う（移動先ディレクトリは自動作成）
  - 補足: `--from` がディスクになく `--to` がディスクにある場合、既に移動済みとみなしてリンク書き換え+DB更新のみ行う
  - 補足: `--to` がディスク上に既に存在する場合は **エラー**（上書き防止）
  - 補足: 移動に伴い、リンクは必要に応じて書き換える
    - `[[a]]` / `[x](a.md)` は、移動後も一意に同じノートを指すなら書き換えない
    - 曖昧になる／別ノートに解決される場合はフルパスに自動書き換え（第三者ファイルのリンクも対象）
    - 移動ファイル自身の outgoing basename リンクも、解決先が変わる場合はフルパスに書き換え
    - `[[path/to/a]]` / `[x](path/to/a.md)` などパス指定は必ず書き換える
    - 移動元ファイル内の相対リンクは新位置からの相対パスに書き換える
  - 補足: 移動元・書き換え対象ファイルの mtime が DB と一致しない場合は **エラー**（stale 検出）
- `delete`
  - 必須: `--file`（複数回指定可）
  - 任意: `--vault`, `--format`, `--rm`
  - `--rm`: ファイルをディスクから削除してからインデックスを更新する
  - 補足: 未登録ファイルが指定された場合はエラー（`--rm` でもファイルは削除されない）
- `disambiguate`
  - 必須: `--name`
  - 任意: `--target`, `--file`, `--vault`, `--format`
  - 補足: `--name` が一意なら自動で対象決定。複数ある場合は `--target` 必須。
  - 補足: `--file` 指定時は対象ファイルのみ書き換える
  - 補足: `--scan` を指定すると DB を使わずに全ファイルを走査して書き換える（初期救済用）
  - 補足: `--scan` は `build.exclude_paths` に従う（除外ファイルは候補にも走査対象にもならない）
- `resolve`
  - 必須: `--from`, `--link`
  - 任意: `--vault`, `--format`, `--fields`
- `query`
  - 必須: `--file` または `--tag` または `--phantom` または `--name`
  - 任意: `--vault`, `--format`, `--fields`, `--include-head`, `--include-snippet`,
    `--max-backlinks`, `--max-twohop`, `--max-via-per-target`,
    `--exclude`, `--exclude-tag`, `--no-exclude`
- `diagnose`
  - 必須: なし
  - 任意: `--vault`, `--format`, `--fields`
- `stats`
  - 必須: なし
  - 任意: `--vault`, `--format`, `--fields`

## update の削除挙動

- ディスクから消えたファイルを `--file` で指定した場合は delete と同じ扱い
  - 参照がある場合: phantom に変換
  - 参照がない場合: ノードを完全に削除

## delete の削除挙動

- 指定ファイルが削除されていた場合は、参照の有無で扱いが変わる
  - 参照がある場合: phantom として扱う
  - 参照がない場合: ノードを完全に削除する

## リンク解釈（互換性）

- wikilink: `[[Note]]`, `[[Note|alias]]`, `[[Note#Heading]]`, `[[Note#^block]]`
- markdown link: `[text](note.md)`, `[text](./note.md#heading)`
  - `note.md` は `[[note]]` と同一扱い
- tag: `#tag`, `#nested/tag`, `#日本語タグ`, `#my-tag`, frontmatter `tags`
  - ネストタグは祖先に展開される: `#a/b/c` → `#a`, `#a/b`, `#a/b/c` の各タグが resolve 可能
- url: `https://...`（将来拡張）
- frontmatter 内リンクは指定キーのみ（設定で制御）
- frontmatter の `aliases` は初期バージョンでは解析しない

## resolve のルール（要点）

- resolve は `from_note` にそのリンクが実際に存在する場合のみ解決する
- 解決結果は必ず1つになる（曖昧な場合はエラー）
- `[[Note]]`: basename を Vault 全体から探索
  - 候補1件なら解決
  - 複数なら曖昧としてエラー（ルート優先例外あり）
- `[[#Heading]]` : 同一ファイル内の見出しとして解決（`from_note` を返す）
- `[[path/to/Note]]`: Vault ルート相対で解決（拡張子省略可）
- `[[./Note]]`, `[[../Note]]`: `from_note` のディレクトリ基準で解決
- Markdown link:
  - `/` 始まり: Vault ルート相対
  - `./` / `../` 始まり: `from_note` 基準
  - `/` を含むがプレフィックスなし（例: `sub/C.md`）: パスとして解決
  - `/` を含まない（例: `Design.md`）: basename 解決（`[[note]]` と同一扱い）
  - Vault 外へ出るパスは厳密モードではエラー

### resolve の一致モード

- 既定は正規化一致
  - alias を除去した一致
  - wikilink と markdown link の同一ターゲット一致
  - basename 一致（ただし曖昧ならエラー）

## query のルール（要点）

- Backlinks, Tags, TwoHop, Outgoing を返す
- 2 Hop は「共通ターゲット方式（A->X かつ B->X）」
- TwoHop は **経由対象（via）を必ず返す**（例: `A <-via- X -> B` の X）
- Outgoing は起点ノートからの外向きリンク一覧
- phantom をクエリ対象に含める
- 出力は priority と上限指定でノイズを抑える
  - `--max-backlinks`（default: 100）
  - `--max-twohop`（default: 100）
  - `--max-via-per-target`（default: 10）
  - 並び順の詳細は将来定義する

### ミューテーション系の出力

- `--format json|text`（default: text）
- `--fields` は不要（結果はフラットで小さい）
- text では空スライスのセクションを省略、JSON では `[]` を出力する
- delete: `deleted`, `phantomed`
- update: `updated`, `deleted`, `phantomed`
- add: `added`, `promoted`, `rewritten`
- move: `from`, `to`, `rewritten`
- disambiguate: `rewritten`

## 出力形式

- `--format json | text`
- `--fields` で出力項目を選択
- query の `backlinks/outgoing/twohop` は **type を含めて出力**する
  - `note` の場合は `name/path/exists` を含む
  - `phantom/tag` は `name` を含む
  - `twohop` は経由対象 `via` と、その `targets` を必ず含む
- `--include-head` はノート冒頭 N 行を返す（`head` フィールド）
- `--include-snippet` はリンク周辺 N 行を返す（`snippet` フィールド）
  - `head/snippet` は `--fields` の指定名
  - 将来: `head` / `snippet` 単体指定時の専用フォーマットを検討する

### query 出力例（twohop）

text:
```
twohop:
- via: note: Notes/Design.md
  targets:
  - note: Notes/Spec.md
  - note: Notes/Plan.md
- via: phantom: MissingConcept
  targets:
  - note: Notes/Spec.md
```

json:
```
{
  "twohop":[
    {
      "via":{"type":"note","name":"Design","path":"Notes/Design.md","exists":true},
      "targets":[
        {"type":"note","name":"Spec","path":"Notes/Spec.md","exists":true},
        {"type":"note","name":"Plan","path":"Notes/Plan.md","exists":true}
      ]
    },
    {
      "via":{"type":"phantom","name":"MissingConcept"},
      "targets":[{"type":"note","name":"Spec","path":"Notes/Spec.md","exists":true}]
    }
  ]
}
```

## 制約と非目標

- コードフェンス/インラインコード内の誤検出は最小限に抑止する
- DB に本文は保持しない（位置情報のみ保持）
- 生成物の手編集は行わない（生成ロジックを修正する）
