---
regen: full
sources:
  - llm-wiki/01-command-index.md
  - llm-wiki/02-write-map.md
  - llm-wiki/03-linktype-matrix.md
  - llm-wiki/04-shared-helpers.md
  - llm-wiki/05-output-contract.md
  - llm-wiki/06-resolve-rewrite.md
  - docs/rules/information-management.md
---

# llm-wiki

AI が編纂・保守する mdhop の作業の入口。**正本ではない** — docs/（rules / specs / decisions）・ソース・テストと矛盾したら、そちらが勝つ。「変更時にどこを読み・何を直すか」をソース全追いより速く掴むための地図と外部知見を置く。

各ページは次の不変条件を満たす（崩れたら直すか捨てる）: **速い**（ソース全追いより速く当たりがつく）/ **docs レベルではない**（正本・ソースを再掲せずポインタで送る）/ **嘘がない**（行・関数・パス参照が現ソースと一致）/ **拾える**（index から到達でき置き場所ルールに従う）。定義の正本は `docs/rules/information-management.md`。点検は `wiki-lint` skill。

権威による配置（docs/ = 正本 / llm-wiki/ = 正本に負ける作業入口）の正本は `docs/rules/information-management.md`。

## 地図と知見

各ファイルは先頭の `regen` フロントマターで再生成可否を宣言する（`full` = 機械的に再抽出できる索引 / `compiled` = ソースを読んで AI が再編纂するガイド / `none` = ソースから復元できない外部知見）。`full` / `compiled` は正本を再掲せず、パス・行・ADR 番号のポインタだけを持つ。腐ったら捨てて作り直す。

| ファイル | regen | 内容 | 主なソース |
|---|---|---|---|
| [01-command-index.md](01-command-index.md) | full | 全 CLI サブコマンド → cmd/core 実装位置の索引 | `cmd/mdhop/main.go` ほか |
| [02-write-map.md](02-write-map.md) | compiled | 書き込み系コマンドの破壊性・波及マップ | `internal/core/{add,update,delete,move,...}.go` |
| [03-linktype-matrix.md](03-linktype-matrix.md) | full | LinkType 全種別 × パース/解決/リライトの対応表 | `internal/core/{parse,resolve,rewrite}.go` |
| [04-shared-helpers.md](04-shared-helpers.md) | full | 共有ヘルパーの定義位置と呼び出しサイト対応表 | `internal/core/{rewrite,util,resolve_maps}.go` |
| [05-output-contract.md](05-output-contract.md) | compiled | stdout JSON / stderr の出力契約ガイド | `cmd/mdhop/format*.go` |
| [06-resolve-rewrite.md](06-resolve-rewrite.md) | compiled | リンク解決〜リライトの編纂ガイド（変更時の読む場所） | `internal/core/{parse,resolve,rewrite}.go` |

外部知見（`regen: none`）の常設ファイルは現状ない。単一の `knowledge.md` には集約しない方針: 特定の関数/ファイルだけに効く罠はそのソースのコメントに置き、複数箇所にまたがる挙動・設計理解は上の地図へ統合する。どちらにも収まらない外部由来の知見（ライブラリ仕様の罠・実測ログ等）が出たら、その時はテーマ別の `regen: none` ページを個別に立てる。

## 使い方

- **作業前に読む**: 変更対象に対応する地図を 1〜2 本だけ読み、どのファイルを読むべきか当たりをつける
- **`full` / `compiled` は手で恒久編集しない**: 直したくなったら正本（ソース・docs/）を直して再編纂する。地図のズレ（行番号・関数名）に気づいたらここを直すのではなく、再生成し直す
- **新しい知見の置き場所**: 特定ソースに紐づく罠はそのコードのコメントへ、横断的な挙動は該当する地図へ。単一の集約ファイルは作らない。ソースから復元できない外部知見が溜まってきたらテーマ別の `regen: none` ページを立て、それが設計判断や仕様を拘束し始めたら docs/decisions か docs/specs へ昇格させる
- **正本の矛盾・抜けに気づいたら**: ここにメモを溜めず、`backlog/` か `docs/decisions/` へ還流させ、正本を直してから再編纂する
