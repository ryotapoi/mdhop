# 汎用診断機能改善（v0.10.0 / v0.11.0 / v0.12.0）

mdhop に追加したい診断・検査系の機能群。LLM Wiki の lint 点検中に見えた不足を、特定フォルダ・特定運用名に依存しない**汎用機能**として整理する。

v0.10.0（path filter とグラフ到達性）と v0.11.0（frontmatter 検査と search 強化）の 2 リリースに分ける。境界は判断ポイントを兼ねる: v0.10.0 を実 vault に当てた結果（残る phantom、`link_keys` の効き方）を v0.11.0 の設計判断（meta-check / meta-validate の統合可否、changed の要否、anchor 検査のコスト）の材料にする。タスク番号は通し番号のまま維持する（節内の相互参照に使用）。

## 背景

2026-06-05 に LLM Wiki（`llm-wiki/`）の markdownlint とリンク状態を確認した際、次が分かった。

- `mdhop diagnose` は vault 全体の basename conflict や phantom を見られるが、vault 全体では既存 phantom が多すぎて、特定領域の点検にはノイズが大きい。
- `llm-wiki/` 内のリンク切れ、index からの到達性、frontmatter `sources:` の実在確認は、mdhop 本体ではなく ad hoc なスクリプトで確認するしかなかった。
- 今回の根本問題は「vault 全体の phantom が多すぎて、対象領域の問題が見えない」こと。

2026-06-10 に、LLM Wiki の lint 点検項目（frontmatter schema 準拠、巨大ページ、stale ページ、似たページの重複、`related:` によるナビゲーション、`[[note#見出し]]` の切れ）と mdhop の現状を突き合わせ、タスク 6〜11 を追加した。その際の現状確認:

- mdhop.yaml の `meta.types` に enum / date の型定義は既にあるが、準拠を検査する機能がない（`--where` の比較・ソートにしか使われていない）。
- frontmatter 値内の `[[...]]` は `frontmatter_wikilink` として edge になるが、`related: [../topics/foo.md]` のような raw path 値は edge にならない。実 vault では raw path で書く運用が普通にある。
- `--where` は `=` `!=` `~` `>` `<` `>=` `<=` EXISTS / NOT EXISTS に対応済み。絶対日付比較（`updated<2026-03-01`）は動くが、相対日付は書けない。
- `search --fields type` のような meta key 指定は `unknown search field` になる。

## 設計方針（最重要）

- **`llm-wiki/` 専用機能を作らない。** mdhop は Markdown vault のリンク・メタデータ・到達性を扱う汎用ツールとして保つ。LLM Wiki としての意味づけは wiki-lint skill や運用側で行う。
- mdhop 本体に入れる機能は、特定フォルダ・特定運用名を知らない汎用機能にする。
- `--path` / `--exclude` の path filter で対象範囲を絞れるようにする（フラグ名は 2026-06-10 決定。タスク 2 参照）。
- mdhop は「グラフ・frontmatter・リンク解決・到達性・変更ファイル列挙」までを提供する。結果の解釈（log に書くべきか、source summary を作るべきか等）は Skill / 運用 / ユーザー判断に残す。

LLM Wiki 側の要求 → mdhop の汎用機能への翻訳:

| LLM Wiki として欲しいこと | mdhop に入れるなら |
|---|---|
| `llm-wiki/` 内の phantom だけ見たい | `diagnose --path "llm-wiki/**"` |
| index から葉ページへ辿れるか見たい | `reachable --from <entry> --path <glob>` |
| `sources:` の参照先が実在するか見たい | 任意 frontmatter key の path-like value 検査 |
| top index を全ページ索引にしないまま孤立を検出したい | 到達性チェック + path filter |
| frontmatter が schema 通りか見たい（必須 key・enum・空値） | `meta-validate`（mdhop.yaml の meta.types を検査側でも使う） |
| 巨大すぎるページ・リンク過疎ページを見たい | search の computed fields（行数・リンク数）+ `--sort` |
| `related:` / `sources:` をナビゲーションとして辿りたい | frontmatter key の path 値を graph edge にする設定 |
| stale 候補（更新が古いページ）を見たい | `--where` の相対日付比較 |
| 似たページの重複・同じ概念の分裂を見たい | mdhop は subgraph export まで。重複判定は skill 側 |
| `[[note#見出し]]` の anchor 切れを見たい | diagnose の anchor 検査（実装コスト次第） |
| `log.md` に書いたか確認したい | mdhop ではなく Skill / 運用側。mdhop は変更ファイル列挙まで |
| source summary を作るべきか判断したい | mdhop ではなく wiki-lint / ingest 側 |

この分離により、mdhop は `llm-wiki/` だけでなく任意の project docs、Obsidian MOC、README 起点のドキュメント群にも使える。

## v0.10.0 タスク — path filter とグラフ到達性

「範囲を絞って診て、辿れる」を 1 つの capability として出す。実装順は記載順: タスク 8（link_keys）をタスク 3（reachable）より先に入れる。`related:` の raw path が edge にならないまま reachable を出すと false positive だらけになるため。

### 1. diagnose に path filter を追加する

目的: 特定領域だけの basename conflict、phantom、壊れたリンクを見たい。

想定コマンド:

```bash
mdhop diagnose --path "docs/**" --format json
mdhop diagnose --path "project/foo/**" --format json
```

決定（2026-06-10）:

- **`--path` は source note を絞る。**「指定 path 配下の note から出ているリンクの問題」を見る。範囲外への参照や phantom も拾う（phantom は実在 path を持たないため source 側で絞るのが唯一成立する定義でもある）。

決定（v0.10.0 実装時）:

- フィルタ指定時の basename conflict は「対象 note からの basename 形式リンク（wikilink / markdown でパス区切りなし）が衝突グループのメンバーを指すグループのみ」を返す。パスリンクは解決リスクがないため対象外。
- `--path` / `--exclude` は CLI 引数のみで動作し、`mdhop.yaml` の `exclude` 設定は diagnose に適用しない（フィルタ未指定時の互換維持を優先）。

受け入れ条件:

- vault 全体の既存 phantom に埋もれず、指定 path 配下の note から出ている phantom を確認できる。
- 既存の `mdhop diagnose` の出力互換性を壊さない。
- `--path` なしの挙動は現状維持。

### 2. path filter の表現をコマンド間で揃える

目的: `search`, `query`, `diagnose`, 将来の検査コマンドで、同じ感覚で対象範囲を絞れるようにする。

決定（2026-06-10）:

- **フラグ名は search の既存名 `--path`（include）/ `--exclude` を正として全コマンドに展開する。** `--include-path` / `--exclude-path` への改名はしない（リリース済みの search と互換を保ち、名前も短い）。

```bash
mdhop search --path "docs/**" --format json
mdhop query --file docs/index.md --path "docs/**" --fields outgoing --format json
mdhop diagnose --path "docs/**" --exclude "docs/archive/**" --format json
```

決定（v0.10.0 実装時）:

- query の `--path` は結果ノード（backlinks / outgoing / twohop targets / snippet）に適用し、twohop の via には適用しない（範囲外 via 経由の到達を保つ）。phantom / tag のような path なし node は `--exclude` と同じ NULL 保護で除外しない。

検討点:

- 除外指定は `docs/archive/**` のような意図的に診断対象から外したい領域に使える。
- path glob の仕様を Go 側と SQL 側で揃える。特に `[` を含むパス、大小文字、ディレクトリ末尾 `/` の扱いに注意（v0.9.0 で同値性テストを先行追加済み: `TestGlobMatchSQLiteEquivalence`）。

受け入れ条件:

- include / exclude の優先順位が明文化されている。
- phantom のような path を持たない node を、source note の path で絞れる。
- 既存コマンドに同じフィルタを追加しても意味が破綻しない。

### 8. frontmatter key の path 値を graph edge にする設定

目的: `related:` / `sources:` のような frontmatter key に書かれた **raw path 値**を graph edge として扱いたい。

現状: frontmatter 値内の `[[...]]` は `frontmatter_wikilink` として既に edge になる（v0.7.0）。しかし `related: [../topics/foo.md]` のような raw path 文字列は edge にならず、グラフから見えない。実vault（LLM Wiki 等）では related / sources を raw path で書く運用が普通にあり、ここが edge にならないと reachable（タスク 3）の false positive、孤立検出の見落としが起きる。

想定設定（mdhop.yaml）:

```yaml
meta:
  link_keys:
    - related
    - sources
```

決定（2026-06-10）:

- **URL 値（`https://...` 等）は edge にしない。** link_keys は vault 内ナビゲーション・到達性に絞り、URL はスキップする。
- **link_type は新設する**（例: `frontmatter_path`）。既存の `frontmatter_wikilink` と区別し、raw path 由来の edge をフィルタ・診断で識別できるようにする。query / graph の JSON 出力に出るため外部仕様。

決定（v0.10.0 実装時、詳細は ADR 0014 / rules/overview.md）:

- 解決規則は markdown link と同一（`./` `../` は note 起点、`/` 含みは vault 相対、なしは basename 解決）。タスク 4（meta-check）もこの規則を共有する。
- raw path 値は厳密モードの検証対象（vault escape・曖昧 basename は build / update / add / move でエラー）。
- raw path 値はリンク構文ではないため rewrite 系（move / disambiguate / simplify / repair / convert）の対象外。raw path の解決先が変わる add / move は操作前にエラーで止める（rebuild との不整合を残さない）。

検討点:

- query backlinks / twohop / `--no-incoming` / reachable のすべてに効く。edge 種別でのフィルタ（本文リンクのみ等）が必要になるかも検討する。

受け入れ条件:

- `link_keys` 未設定なら現状の挙動を完全維持。
- 設定した key の path 値が backlinks / reachable / 孤立検出に反映される。
- 解決できない path 値は phantom 相当として diagnose で見える。

### 3. 到達性チェックを追加する

目的: 入口ページから、指定 path 配下のページへリンクで辿れるか確認したい。

想定コマンド:

```bash
mdhop reachable --from docs/index.md --path "docs/**" --format json
mdhop reachable --from project/foo/index.md --path "project/foo/**" --format json
```

用途:

- docs index から葉ページが孤立していないか確認する。
- トップ index を全ページ索引にせず、sub-index 経由で辿れるかを確認する。
- 任意プロジェクトの README / index / MOC から関連 note に到達できるかを見る。

検討点:

- frontmatter `related:` を graph edge として扱うか（タスク 8 で対応。本タスクより先に入れる）。
- 本文 wikilink のみか、markdown link も対象にするか。
- 最大深さを指定できるようにするか。

受け入れ条件:

- 到達できない note の一覧を出せる。
- 到達経路をオプションで出せる。
- index から直リンクされていないが sub-index から辿れるページを false positive にしない。

### 9. subgraph export

目的: 指定範囲の node / edge をそのまま機械可読形式で出力し、mdhop に入れない分析（類似ページ検出、クラスタ分析、中心性など）を呼び出し側でできるようにする。

想定コマンド:

```bash
mdhop graph --path "docs/**" --format json
mdhop graph --path "docs/**" --format dot
```

用途:

- 「似たページの重複」「同じ概念の分裂」のような意味判断を伴う lint を、skill 側が outgoing-link 重複度などから機械的に絞り込む材料にする。
- 設計方針「mdhop はグラフ提供まで、解釈は呼び出し側」の escape hatch。個別の分析コマンドを mdhop に増やさずに済む。

検討点:

- edge に link_type を含める。
- phantom node を含めるか（オプション化が自然）。
- `--format dot` は可視化用の おまけ。json を先に決める。

受け入れ条件:

- node / edge の列挙のみで、解釈ロジック（類似度計算等）を mdhop に入れない。
- path filter（include / exclude）が使える。
- JSON schema が明文化されている。

### 12. examples skill と README を v0.10.0 追加分に同期する

目的: README は `examples/skills/mdhop` を up-to-date な skill として案内している。リリース時点で v0.10.0 の新コマンド・新オプションが反映されている状態にする。

対象:

- `examples/skills/mdhop/SKILL.md` と `references/`（path filter の使い分け、`reachable`、`graph`、`meta.link_keys` の使用例）
- `README.md` / `README.ja.md` の Commands 表と使用例

受け入れ条件:

- v0.10.0 で追加した全コマンド・オプションが skill の references から引ける。
- README の Commands 表が実装と一致する。

## v0.11.0 タスク — frontmatter 検査と search 強化

frontmatter の品質検査と search の強化。設計前に v0.10.0 の実運用結果を見る: meta-check / meta-validate の統合可否（タスク 4・6）、changed の要否（タスク 5）、anchor 検査のコスト判断（タスク 11）はここで決める。うち **10（`--where` 相対日付）は実施決定**、**11（anchor 検査）は実装コストが軽い場合のみ**実施する。

### 4. frontmatter の任意 key を path-like value として検査する

目的: `sources:`, `related:`, `attachments:`, `refs:` など frontmatter 内の任意 key の値が、vault 内 path / URL / directory として妥当か確認したい。

想定コマンド案:

```bash
mdhop meta-check --path "docs/**" --key sources --kind path-list --format json
mdhop meta-check --path "project/**" --key related --kind wikilink-list --format json
```

`sources:` 専用ではなく**任意 key** を対象にする。

検討点:

- URL は `http://` / `https://` を許可する。
- vault-relative path を実在確認する。
- directory path を許可するかをオプション化する。
- wikilink と raw path は扱いを分ける。
- frontmatter の scalar / list / quoted wikilink を安定して読む。

受け入れ条件:

- key 名を mdhop 本体が固定しない。
- 存在しない path、曖昧な wikilink、許可外 URL scheme を区別して報告できる。
- `--format json` で agent が扱いやすい構造を返す。

### 6. frontmatter schema validation（meta-validate）

目的: frontmatter が宣言済み schema に準拠しているか検査したい。mdhop.yaml の `meta.types` には ordered enum / date 等の型定義が既にあるが、現在は `--where` の比較・ソートにしか使われておらず、**準拠を検査する側が存在しない**。

想定コマンド:

```bash
mdhop meta-validate --path "docs/**" --require type,status,updated --format json
mdhop meta-validate --path "books/**" --require isbn --format json
```

検出したいもの:

- 必須 key の欠落（`--require` で指定）
- enum（ordered）外の値（`status: actve` のような typo）
- date 型なのに parse できない値
- key はあるが値が空（空フィールド検出）

検討点:

- 必須 key セットを CLI 引数で渡すか、mdhop.yaml に path-scoped で宣言できるようにするか。まずは CLI 引数のみで十分。
- タスク 4 の meta-check との関係を明確にする。meta-check は「値の参照先が実在するか」、meta-validate は「key の存在と型・enum 妥当性」。コマンドを分けるか統合するかは設計時に判断する。
- `meta.types` に未宣言の key をどう扱うか（無視が基本）。

受け入れ条件:

- key 名・必須セット・enum 値を mdhop 本体がハードコードしない。すべて mdhop.yaml / CLI 引数由来。
- 欠落・enum 外・型不正・空値を区別して報告できる。
- `--format json` で agent が扱いやすい構造を返す。

### 7. search の computed fields（行数・リンク数）と meta key 出力

目的: 「巨大すぎるページ」「リンクがほぼない葉ページ」「backlinks 過密な hub」を機械的に検出したい。

想定コマンド:

```bash
mdhop search --path "docs/**" --sort -lines --limit 10 --format json
mdhop search --path "docs/**" --fields path,lines,outgoing_count,incoming_count --format json
mdhop search --path "docs/**" --fields path,meta.type,meta.status --format json
```

検討点:

- どの計算値を持つか: lines / bytes / outgoing_count / incoming_count あたりから。heading 数は anchor 検査（タスク 11）と要否を合わせて判断。
- index に永続化するか実行時計算か。`--sort` で使うなら index 側が自然。
- 現在 `search --fields type,status` が `unknown search field` で落ちる。meta key の出力対応（`meta.<key>` 等の記法）も合わせて入れると agent から使いやすくなる。

受け入れ条件:

- computed fields を `--fields` と `--sort` の両方で使える。
- 既存の `--fields` 指定の出力互換性を壊さない。
- 計算値の定義（lines に frontmatter を含むか等）が明文化されている。

### 10. `--where` の相対日付比較【実施決定】

目的: 「更新が古いページ」（stale 候補）を、日付を外部で計算せずに検出したい。

想定コマンド:

```bash
mdhop search --where "updated<today-90d" --format json
mdhop search --path "docs/**" --where "reviewed<today-1y" --format json
```

検討点:

- 構文を決める: `today`, `today-90d`, `today-3m`, `today-1y` あたり。`now` との別名は不要。
- `meta.types` で `date` 宣言された key のみ対象にするか、値が date 形式なら許すか。
- タイムゾーンはローカル日付でよい。
- 既存の絶対日付比較（`updated<2026-03-01`）と同じ比較経路に乗せる。

受け入れ条件:

- 絶対日付比較の既存挙動を壊さない。
- 相対日付の評価基準日が明文化されている（実行時のローカル日付）。

### 5. 変更ファイル列挙を汎用コマンドとして提供する（要検討）→ 見送り（2026-06-11）

判断: **実装しない。** `git diff --name-status -M <since> -- <pathspec>` で status・old/new path（rename 含む）・path filter が取れ、untracked は `git ls-files --others --exclude-standard`、JSON 整形は jq でできる。この機能は mdhop の SQLite インデックス（nodes/edges/meta）を一切使わず、git を shell out するだけのラッパーになる。mdhop の意味境界（事前解析したリンクグラフ・解決・到達性を引く）に属さないため見送る。変更ファイルとその backlinks / reachability を結合して出したい要求が将来出たら、その時点で mdhop インデックス側に意味が寄るので再検討する（YAGNI）。

目的: 特定 path 配下で、git の基準点以降に変わった Markdown note を機械的に列挙したい。

想定コマンド:

```bash
mdhop changed --since HEAD --path "docs/**" --format json
mdhop changed --since main --exclude "docs/archive/**" --format json
```

log 整合チェックではない。mdhop は「何が変わったか」を出すだけ。

検討点:

- すでに git コマンドで簡単に取れるなら、mdhop に入れず Skill / script でよい可能性がある。**git コマンドとの差分を見てから判断する。**
- untracked file を含めるか。
- deleted / renamed / moved をどう表現するか。

受け入れ条件:

- `llm-wiki` 専用、`log.md` 専用、daily report 専用の意味づけを持たない。
- path include / exclude が使える。
- `--format json` で changed file、status、old path / new path を返せる。

### 11. anchor 切れ検出【実装が軽い場合のみ】

目的: `[[note#見出し]]` / `[text](note.md#fragment)` の fragment が、対象 note の見出しとして実在するか検査したい。

想定コマンド:

```bash
mdhop diagnose --path "docs/**" --fields anchors --format json
```

検討点:

- 見出し→fragment の正規化規則を Obsidian 互換にする必要がある（スペース、記号、大小文字、重複見出しの連番）。ここが重いなら見送る。
- index に各 note の headings を持つ必要がある。タスク 7 の computed fields と index 拡張を合わせて設計すると二度手間にならない。
- block reference（`#^block-id`）は対象外でよい。

受け入れ条件:

- 対象 note は存在するが fragment が見つからないケースを、note ごと存在しない（phantom）ケースと区別して報告する。
- `--path` で対象範囲を絞れる。

### 13. examples skill と README を v0.11.0 追加分に同期する

目的: タスク 12 と同じ。v0.11.0 の新コマンド・新オプションをリリース時点で反映する。

対象:

- `examples/skills/mdhop/SKILL.md` と `references/`（`meta-check` / `meta-validate`、computed fields、相対日付比較の使用例。`changed` / anchor 検査は実施した場合のみ）
- `README.md` / `README.ja.md` の Commands 表と使用例

受け入れ条件:

- v0.11.0 で追加した全コマンド・オプションが skill の references から引ける。
- README の Commands 表が実装と一致する。

## v0.12.0 タスク — 2026-06-11 LLM Wiki lint 所見

2026-06-11 に Knowledge vault の `/wiki-lint`（v0.11.0 適用後の初回フル lint）で踏んだ不足。いずれも特定運用に依存しない汎用機能として整理する。

### 14. index 登録パスの Unicode 正規化（NFC 統一）

目的: NFD ファイル名と NFC 参照の不一致による二重ノード登録・偽 not_found を防ぐ。

経緯（2026-06-11）:

- `03-Notes/tech/ai/coding-agent/AIエージェント開発の情報管理-原則.md` がディスク上 NFD 名で保存されており、frontmatter の NFC 参照が `meta-check` で not_found になった。
- NFC パスで `mdhop add` すると、既存の NFD ノードと**同一ファイルが 2 ノードとして二重登録**された。
- NFD 側を `mdhop delete` しようとすると「ファイルがまだ存在する」と拒否された（APFS は正規化非区別のため NFD パスの stat が NFC 実体に当たる）。
- 復旧は `mdhop build` の全再構築でしかできなかった。

対応案:

- index 登録時（build / add / update / move）にパスを NFC へ正規化して保存する。
- パス比較系（meta-check / resolve / delete / move の重複検査）も比較前に NFC 正規化する。
- 既存 index の NFD エントリは build で移行される（マイグレーション不要）。

受け入れ条件:

- NFD 名のファイルを add しても 1 ノード（NFC）として登録される。
- NFC / NFD どちらの表記の参照でも resolve / meta-check が当たる。
- 既存の NFC-only vault で挙動が変わらない。

### 15. meta-check のディレクトリ参照対応

目的: frontmatter の path 値がディレクトリを指す場合の偽 not_found を解消する。

経緯（2026-06-11）: `sources:` にディレクトリ参照（例: `03-Notes/hobbies/スプラトゥーン/`）を書く運用があり、実在するのに meta-check が not_found を返す。lint のたびに 8 件の誤検出を目視で除外する必要があった。ディレクトリ参照は LLM Wiki 固有ではなく、一般の docs / MOC でも普通にあり得る。

対応案:

- 値が `/` で終わる場合はディレクトリとして存在チェックする。
- 実在すれば issue にしない。実在しなければ従来どおり not_found。
- ディレクトリをどう扱ったか分かるよう、`--format json` に種別（note / directory）を出してもよい。

受け入れ条件:

- 実在ディレクトリへの参照が issue にならない。
- 存在しないディレクトリ参照は not_found のまま検出される。
- 末尾 `/` なしの既存挙動は変わらない。

### 16. repair に path filter を追加する

目的: 特定領域だけリンク修復したい。

経緯（2026-06-11）: `llm-wiki/` の lint で `repair` を使うと、対象が vault 全体固定のため scope 外（`03-Notes/`, `Dashboard.md`）の書き換えが混ざり、`git checkout` で戻す運用になった。`meta-check` / `reachable` / `diagnose` には `--path` があり、repair だけ非対称。

対応案:

- タスク 2 で決めた `--path` / `--exclude` の表現（source note を絞る）を repair にも適用する。
- `--dry-run` との組み合わせで「範囲内の修復案だけ」を確認できるようにする。

受け入れ条件:

- `repair --path "llm-wiki/**"` で指定範囲の note のみ書き換わる。
- `--path` なしの挙動は現状維持。

## Non-goals

mdhop 本体ではやらないこと。

- `llm-wiki/` を特別扱いする。
- `llm-wiki/log.md` に記録されたかを判定する。
- source summary を作るべきか判断する。
- 特定フォルダの運用ルールを mdhop に埋め込む。
- Markdown style lint を mdhop に取り込む（形式検査は `markdownlint-cli2` に任せる）。
- 類似ページ・同一概念の分裂の判定を mdhop に入れる。意味の重複判定は LLM / skill 側の仕事。mdhop は subgraph export（タスク 9）で材料を出すまで。

## 実装時の注意

- path filter は source note と target node のどちらに効いているかを明示する。
- phantom node は path を持たないため、target path filter だけでは扱えない。
- 既存 `diagnose` の出力を壊さない。
- `--format json` の schema を先に決める。
- tests は CLI レベルと core レベルの両方に置く。
- `llm-wiki/**` はテスト fixture の一例にしてよいが、仕様名や関数名に `llmWiki` を入れない。
