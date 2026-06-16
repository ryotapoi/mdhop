---
name: wiki-lint
description: llm-wiki/ が作業地図として機能しているか点検する。孤立ページ・リンク切れ・sources パス切れを機械検証し、各ページが「速い / docs レベルでない / 嘘がない / 拾える」の不変条件を満たすか照合する。節目や llm-wiki 編集後に使う。
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
- **拾える**: index.md から全ページに到達でき、置き場所ルール（特定ソースの罠はコメント / 横断的挙動は地図 / 単一の集約ファイルは作らない）に従う。

## Constraints

- 規範・不変条件の定義は `information-management.md` が正本。この skill に再掲・上書きしない。
- `regen: full` / `compiled` ページの**内容のズレ**（行番号・関数名・正本再掲）は手で取り繕わない。再編纂が必要な旨をレポートし、修正は main / ユーザーに委ねる（直すなら正本・ソースを直して再生成）。
- 修正してよいのは機械的に安全なものだけ（後述）。それ以外は検出してレポートする。
- `mdhop meta-check --key sources` は llm-wiki を vault にすると sources が vault 外（cmd/ internal/ docs/）を指して全件 not_found になるため**使わない**。sources のパス実在はリポジトリルートからの存在確認で行う。

## Acceptance

- 4 不変条件それぞれについて、違反ページ一覧または「違反なし」が出ている。
- 機械検証（孤立・リンク切れ）の結果が出ている。
- 機械的に修復した箇所と、再編纂・判断が必要な箇所が分離されている。

## Relevant

- `docs/rules/information-management.md`（不変条件・置き場所ルールの正本）
- `llm-wiki/index.md`（ページ一覧・regen 区分・sources）
- `llm-wiki/*.md`（点検対象）
- 各ページ frontmatter の `sources:`（嘘チェックの照合先）

## 手順

### 1. 機械検証（mdhop で検出可能なもの）

llm-wiki/ をサブツリー vault として build し、index 起点の到達性と内部リンク切れを見る。`.mdhop/` は `.gitignore` 済み。

```bash
mdhop build --vault llm-wiki                                          # llm-wiki/.mdhop/index.sqlite を作る
mdhop reachable --vault llm-wiki --from index.md --format json        # unreachable = index から拾えない孤立ページ
mdhop diagnose --vault llm-wiki --format json                         # llm-wiki 内のリンク切れ・anchor 切れ
```

- `reachable` の `unreachable` が空でなければ「拾える」違反（index.md にリンクを足すか、ページ自体が不要かを判断）。
- `diagnose` の broken / phantom は llm-wiki 内のページ間リンク切れ。`repair --vault llm-wiki --dry-run --format json` で書き換え案を確認できる（実修正は機械的に安全な範囲のみ、後述）。

### 2. sources パスの実在（リポジトリルートから）

各ページ frontmatter の `sources:` が指すパスは vault 外なので、リポジトリルートで実在確認する。

```bash
# 例: 全ページの sources 値を集め、存在しないものを洗い出す（実装は grep + test -f で可）
```

存在しない sources は「嘘がない」違反の予兆（ソースがリネーム・削除された）。該当ページの本文参照も古い可能性が高いので 3 で重点的に照合する。

### 3. 不変条件の照合（ページを読んで判断）

機械検証で当たりをつけたページ、および sources が動いたページを優先して読み、4 軸を照合する。全ページ毎回フル照合する必要はなく、対象は「今回 lint 対象に指定された範囲」「sources が動いたページ」「機械検証で引っかかったページ」に絞ってよい。

- **速い**: 経路・読む場所・注意点があるか。「§変更時の読むべき場所」相当の導線があるか。ポインタだけで中身が空でないか。
- **docs レベルではない**: 正本・ソースの本文を再掲していないか。`regen: full`/`compiled` がポインタに徹しているか。
- **嘘がない**: 本文の `file.go:NN` 行参照・関数名が現ソースと一致するか（疑わしい参照だけ実ソースを開いて確認。同型の多数参照は代表例で確認しサンプリングでよいが、サンプリングしたことをレポートに明記する）。
- **拾える**: 単一の集約ファイル（knowledge.md 的なもの）が復活していないか。`regen: none` ページが正本級情報を抱え込んでいないか（昇格漏れ）。

### 4. 修正とレポート

**機械的に修復してよいもの**（実施する）:

- `repair --vault llm-wiki`（--dry-run を外す）で直せる llm-wiki 内の壊れた path リンク。
- frontmatter `sources:` の明白なパス追従（ソースがリネームされ新パスが一意に確定する場合）。

**修復せずレポートするもの**:

- `regen: full`/`compiled` ページの行番号ズレ・正本再掲・「速い」違反 → 再編纂が必要な旨を報告（main がソース・正本を直して再生成）。
- 単一集約ファイルの復活・昇格漏れ → 配置の判断が要るので報告。

修復後は `mdhop update --vault llm-wiki --file <path>` で index に反映して再検証する。

## 出力

```md
# llm-wiki lint

## 機械検証
- 孤立ページ（reachable unreachable）:
- リンク切れ（diagnose broken/phantom）:
- sources パス切れ:

## 不変条件
- 速い 違反:
- docs レベル違反（正本再掲）:
- 嘘（行・関数・パスのズレ）違反:    ※サンプリング照合した場合はその旨
- 拾える 違反（集約ファイル復活・昇格漏れ）:

## 修復したこと（機械的に安全な範囲）

## 再編纂・判断が必要（main / ユーザー）
```
