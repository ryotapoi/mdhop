---
name: wiki-lint
description: llm-wiki/ が作業地図として機能しているか点検する。孤立ページ・リンク切れ・sources パス切れを検出し（grep/目視が基本、mdhop があれば高速路）、各ページが「速い / docs レベルでない / 嘘がない / 拾える」の不変条件を満たすか照合する。節目や llm-wiki 編集後に使う。
---

# wiki-lint

`llm-wiki/` が Coding Agent の作業地図として機能している状態を維持するための点検 skill。

## Intent

llm-wiki/ の各ページが、`docs/rules/information-management.md` で定める不変条件を満たしているかを点検し、機械的に安全な修復だけ行う。崩れているページは「直すか捨てる」判断材料を main に返す。

## 不変条件（判定基準）

正本は `docs/rules/information-management.md` の「正本ではない情報 → llm-wiki/」節。ここでは再掲せず要点だけ:

- **速い**: ソース全追いより速く「どこを読み・何を直すか」の当たりがつく。経路・読む場所・注意点があり、単なる目次の羅列ではない。
- **docs レベルではない**: 正本（rules/specs/decisions）・ソースを再掲しない。規範・仕様・設計理由はポインタ（パス・行・ADR 番号）で送る。
- **嘘がない**: 行番号・関数名・パス参照が現在のソースと一致している。
- **拾える**: index.md から全ページに到達でき、ページ間リンクが GitHub で繋がる `[テキスト](相対パス.md)` 形式（`[[wikilink]]` は使わない）。置き場所ルール（特定ソースの罠はコメント / 横断的挙動は地図 / 単一の集約ファイルは作らない）に従う。

## Constraints

- 規範・不変条件の定義は `information-management.md` が正本。この skill に再掲・上書きしない。
- `regen: full` / `compiled` ページの**内容のズレ**（行番号・関数名・正本再掲）は手で取り繕わない。再編纂が必要な旨をレポートし、修正は main / ユーザーに委ねる（直すなら正本・ソースを直して再生成）。
- 修正してよいのは機械的に安全なものだけ（後述）。それ以外は検出してレポートする。
- mdhop は任意の補助。なくても grep/目視で点検できる書き方を崩さない。
- `mdhop meta-check --key sources` は使わない。sources はリポジトリ全域（cmd/ internal/ docs/）を指すため、`meta-check` の値検証より、リポジトリルートからの素朴な存在確認（grep + パス実在）の方が確実。

## Acceptance

- 4 不変条件それぞれについて、違反ページ一覧または「違反なし」が出ている。
- 孤立・リンク切れ、および外向きポインタ（sources・本文の docs/・ソース参照）の実在の検出結果が出ている。
- 機械的に修復した箇所と、再編纂・判断が必要な箇所が分離されている。

## Relevant

- `docs/rules/information-management.md`（不変条件・置き場所ルールの正本）
- `llm-wiki/index.md`（ページ一覧・regen 区分・sources）
- `llm-wiki/*.md`（点検対象）
- 各ページ frontmatter の `sources:`（嘘チェックの照合先）

## 手順

### 1. 孤立・リンク切れの検出（grep / 目視が基本、mdhop は任意）

llm-wiki/ は GitHub でリンクされる素の Markdown（ページ間リンクは `[テキスト](相対パス.md)`）。ページ数は少なくフラットなので、基本は目視 + grep で足りる。

- **孤立ページ**: `ls llm-wiki/*.md` と `index.md` の一覧表を突き合わせ、表に載っていない `.md` がないか確認する。
- **リンク切れ**: 各ページの `[...](相対パス)` のリンク先ファイルが実在するか確認する（grep でリンクを抽出し、パス実在を見る）。

**mdhop があれば**高速路として使える（任意。必須ではない）。このリポジトリはルートに `mdhop.yaml`（testdata/ examples/ を除外）があるので、ルートで build すれば docs/ も含めた整合性を一度に見られる。`.mdhop/` は `.gitignore` 済み。

```bash
mdhop build                                                                   # ルート build（mdhop.yaml で testdata/examples 除外）
mdhop reachable --from llm-wiki/index.md --path 'llm-wiki/**' --format json   # unreachable = index から拾えない孤立ページ
mdhop diagnose --path 'llm-wiki/**' --format json                             # llm-wiki 内のリンク切れ・anchor 切れ・phantom
```

- `diagnose` は必ず `--path 'llm-wiki/**'` で絞る。絞らないと cmd/ と internal/ の同名 `.go`、`.agents/` と `.claude/` の同名 workflow など llm-wiki と無関係の basename 衝突が大量に出て、結果を読み違える。docs/ も一緒に見るなら `--path 'docs/**' --path 'llm-wiki/**'` と対象を明示する。
- `reachable` の `unreachable` が空でなければ「拾える」違反（index.md にリンクを足すか、ページ自体が不要かを判断）。
- `diagnose` の broken / phantom はリンク切れ。`repair --path 'llm-wiki/**' --dry-run --format json` で書き換え案を確認できる（実修正は機械的に安全な範囲のみ、後述）。

### 2. 外向きポインタの実在（リポジトリルートから）

llm-wiki は正本を再掲せず、docs/・ソースへ**ポインタ**で送る設計。そのポインタが切れていたら「嘘がある」状態なので、リポジトリルートで実在確認する。対象は 2 種類:

- **frontmatter の `sources:`** — 各ページの導出元（cmd/ internal/ docs/ など）。
- **本文中のパス参照** — `docs/decisions/0004-….md`、`internal/core/build.go` のような、正本・ソースを指す裸のパス。これらは markdown link ではなく文字列なので mdhop の `diagnose` では拾えない。**grep + 実在確認が唯一の検出手段**（mdhop は無力）。

```bash
# 例: llm-wiki 本文 + frontmatter から docs/・cmd/・internal/ 等のパスを grep で抽出し、
#     リポジトリルートで存在しないものを洗い出す（grep -ohE で抽出 → test -e でループ確認）。
#     `file.go:NN` の :NN 部分はパス実在の確認では落として、ファイルの有無だけ見る（行ズレは 3 で照合）。
```

**glob / brace の短縮記法に注意**: 地図は `cmd/mdhop/format*.go` や `internal/core/{add,update,delete,move,...}.go` のような短縮記法を使う（同型ファイルをまとめて速く示すための表現で、これは許す）。素朴な `test -e` はこれらを必ず「無い」と誤検出するので、`*` / `{...}` / `...` を含むパスは:

- `...`（「ほか」を表す省略）を除いてから shell の glob として展開し、**1 つでもマッチすれば実在**とみなす（例: `ls cmd/mdhop/format*.go` がヒットすれば OK）。
- 1 つもマッチしないときだけ「切れ」として報告する。

存在しないパス（短縮記法は展開しても 0 マッチ）は「嘘がない」違反（正本・ソースがリネーム・削除された）。該当ページの行参照・本文も古い可能性が高いので 3 で重点的に照合する。

### 3. 不変条件の照合（ページを読んで判断）

機械検証で当たりをつけたページ、および sources が動いたページを優先して読み、4 軸を照合する。全ページ毎回フル照合する必要はなく、対象は「今回 lint 対象に指定された範囲」「sources が動いたページ」「機械検証で引っかかったページ」に絞ってよい。

- **速い**: 経路・読む場所・注意点があるか。「§変更時の読むべき場所」相当の導線があるか。ポインタだけで中身が空でないか。
- **docs レベルではない**: 正本・ソースの本文を再掲していないか。`regen: full`/`compiled` がポインタに徹しているか。
- **嘘がない**: 手順 2 で実在を確認したパスについて、行番号（`file.go:NN`）・関数名・参照先の節が現ソース / 正本と一致するか（疑わしい参照だけ実ファイルを開いて確認。同型の多数参照は代表例で確認しサンプリングでよいが、サンプリングしたことをレポートに明記する）。手順 2 でパスが消えていたものは確実にズレているので必ず照合する。
- **拾える**: index.md から全ページに到達でき、ページ間リンクが GitHub でレンダリングされる `[テキスト](相対パス.md)` 形式か（`[[wikilink]]` は GitHub で繋がらないので使わない）。単一の集約ファイルが復活していないか。`regen: none` ページが正本級情報を抱え込んでいないか（昇格漏れ）。

### 4. 修正とレポート

**機械的に修復してよいもの**（実施する）:

- 壊れた `[](相対パス)` リンク（リンク先の正しいパスが一意に確定する場合）。mdhop があれば `repair --path 'llm-wiki/**' --dry-run` で書き換え案を確認してから直す（`--path` で絞らないと llm-wiki 外まで対象になる）。
- frontmatter `sources:` の明白なパス追従（ソースがリネームされ新パスが一意に確定する場合）。

**修復せずレポートするもの**:

- `regen: full`/`compiled` ページの行番号ズレ・正本再掲・「速い」違反 → 再編纂が必要な旨を報告（main がソース・正本を直して再生成）。
- 単一集約ファイルの復活・昇格漏れ → 配置の判断が要るので報告。

mdhop を使った場合、修復後は `mdhop build`（または `mdhop update --file <path>`）で index に反映して再検証する。

## 出力

```md
# llm-wiki lint

## 孤立・リンク切れ・外向きポインタ
- 孤立ページ（index に載っていない / reachable unreachable）:
- ページ間リンク切れ（[](相対パス) の切れ / diagnose broken/phantom）:
- 外向きポインタ切れ（frontmatter sources / 本文の docs/・ソース参照のパス消失）:

## 不変条件
- 速い 違反:
- docs レベル違反（正本再掲）:
- 嘘（行・関数・パスのズレ）違反:    ※サンプリング照合した場合はその旨
- 拾える 違反（集約ファイル復活・昇格漏れ）:

## 修復したこと（機械的に安全な範囲）

## 再編纂・判断が必要（main / ユーザー）
```
