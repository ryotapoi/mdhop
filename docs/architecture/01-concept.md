# mdhop コンセプト資料

## 1. これは何か

mdhop は、Obsidian Vault のような **複数 Markdown ファイルのリンク関係（wikilink / markdown link / tag / frontmatter）** を事前解析してキャッシュし、**Coding Agent（Claude Code / Codex など）が grep を乱発せずに「関連ノートへ辿る」ための CLI** です。

- Vault 配下の `**/*.md` を解析して **リンクグラフ**を作る
- `(from_note, link文字列)` から **解決先ノートの Vault 相対パス**（または phantom/tag/url）を返す
- 指定ノートの **Backlinks / 2 Hop Links / Tags** を返す
- 返却は **JSON** と、LLM に貼り付けやすい **圧縮テキスト（prompt）** をサポートする

> 2 Hop Link は「共通ターゲット方式（A→X かつ B→X）」を採用する。
> X は `note | phantom | tag`（任意で url）を含められる。

---

## 2. 何を解決するのか（背景と狙い）

### 2.1 LLM/Coding Agent の困りごと

- Vault の関連ノート探索が **grep 依存**になる
  - 変更規模が大きいと時間がかかる / ノイズが多い / 文脈が膨らむ
- `[[basename]]` のリンク解決が、同名ファイル（例: `README.md`）で **曖昧**になる
- rename/move が発生したとき、リンク整合性・「最短リンク表記」を維持するのが難しい

### 2.2 mdhop が提供する“省コンテキスト能力”

- “どこを見ればよいか” を **DBクエリで即決**できる（= grep しない）
- `--fields` / `--include-content` / `--include-context` により
  - 必要なフィールドだけ返す
  - ノート冒頭やリンク周辺の **最小スニペット**を返す
- phantom/tag をノードとして扱い、存在しないノートでも関連探索（2hop含む）が可能

---

## 3. コアの思想

### 3.1 「正しさ（壊さない）」を最優先

- Obsidian 互換のリンク/タグ解釈を壊さない
  - wikilink: `[[Note]]`, `[[Note|alias]]`, `[[Note#Heading]]`, `[[Note#^block]]`
  - markdown link: `[text](note.md)`, `[text](./note.md#heading)` など
  - tag: `#tag`, `#nested/tag`、frontmatter `tags`
- コードフェンス/インラインコード等は誤検出しない（最低限の対策）

### 3.2 「読み取り系」と「書き換え系」を概念分離

- **core（参照系）**: build/update/resolve/query/diagnose など
  - 安全に読むだけ
- **mutate（書き換え系）**: reconcile/rename/canonicalize など
  - Git差分に影響しやすいので、実行頻度を低く・安全装置を付ける

※ ただし、バイナリやインストールを分ける必要はなく、コマンド/権限で分けるだけでも良い。

### 3.3 「Shortest path when possible」を CLI で制御する

原則:
- basename が Vault 内で一意 → `[[basename]]` が“最短”で安全
- basename が複数 → `[[path/to/basename]]`（Vault ルート相対）で一意化

重要:
- basename の重複が発生した瞬間（1→2）に、既存の `[[basename]]` の意味を保つため
  - 旧一意先（既存ノート）へ向いていたリンクだけを `[[path/to/basename]]` に一括変換する（= reconcile）
- 後で一意に戻ったら `[[basename]]` に戻してもよいが、Git差分が揺れるので “戻し” はオプション扱いにする

---

## 4. ユースケース（Agent目線）

### 4.1 参照系（普段のワークフロー）

- あるファイルの `[[link]]` を解決して、ノート本文へ飛ぶ
- Backlink / 2hop / tag によって、関連の強いノートを少ないコンテキストで集める
- phantom ノートの backlinks を取り、「まだ存在しないが参照されている概念」を見つける

### 4.2 書き換え系（イベント時だけ）

- 同名ノートが増えた（例: `README.md` が複数になった）
  - `[[README]]` のままだと曖昧になるので、既存リンクを `[[path/to/README]]` に変換して一意化
- ファイル名変更時にリンクを追従させたい（rename/move）

---

## 5. コマンドの全体像（イメージ）

- `mdhop build` : Vault 全量を解析してDB作成
- `mdhop update --file ...` : 指定ファイルのみ差分更新（ファイル削除も反映）
- `mdhop resolve --from A.md --link '[[X]]'` : リンク解決（曖昧なら候補返却）
- `mdhop query --file A.md` : backlinks/tags/2hop を返す（fields で絞る）
- `mdhop diagnose` : basename衝突、phantom一覧、パース失敗等を検出

（任意・mutate）
- `mdhop reconcile --file NEW.md` : 1→2 衝突時の意味保存リライト
- `mdhop canonicalize` : “戻し” を含む正規化（デフォルトOFF）

---

## 6. 成果物（期待される状態）

- Coding Agent が「次に読むべきノート」を DB から取得できる
- `[[basename]]` を最大限使いながら、曖昧性が出るケースだけ path によって一意解決できる
- phantom/tag/2hop により、grep とは異なる観点で「関連が強いノート」を取り出せる
