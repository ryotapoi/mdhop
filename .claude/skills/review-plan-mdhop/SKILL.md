---
name: review-plan-mdhop
description: mdhop 固有の設計制約に基づくプランレビュー。通常はチェーンスキルから呼ばれる。
argument-hint: <plan-file-path>
allowed-tools: Read, Glob, Grep, Task
context: fork
agent: no-mcp-worker
---

# Self Plan Review — mdhop Project

グローバルの `/review-plan` の後に追加実行する mdhop プロジェクト固有のプランレビュー。
1つの Plan サブエージェントで実行する。

**重要な制約:**
- 使用できるツール: Read, Glob, Grep, Task **のみ**
- **mcp__codex__codex や mcp__codex__codex-reply は絶対に呼び出さないこと**
- レビューは Task ツール（subagent_type: Plan）で実行する。自分で直接レビューしたり、MCP ツールで外部に聞いたりしない
- **結果はファイルに書き出さない。テキストとして返すだけにすること。/tmp やプロジェクト配下へのファイル作成は行わない**

## 手順

### 1. プランファイルのパスを決定する

- `$ARGUMENTS` が空でなければ、その値をプランファイルパス `PLAN_PATH` とする
- `$ARGUMENTS` が空なら:
  1. Glob で `tmp/plans/*.md` を検索する
  2. 最も新しいファイル（Glob 結果の先頭）を `PLAN_PATH` とする
  3. ファイルが見つからなければ「tmp/plans/ にプランファイルが見つかりません」と返して終了する

### 2. プランファイルを読む

- Read で `PLAN_PATH` を読み込む
- プラン内で参照されているファイル（仕様書・対象コード）のパスを抽出する

### 3. Plan サブエージェントを起動する

Task ツールで `subagent_type: Plan, model: "sonnet"` を使う。

エージェントには以下を渡す:
- プランの全文
- 参照ファイルのパス一覧
- 「コードや仕様書は自分で Read/Grep/Glob して確認すること」という指示

#### Agent 1: mdhop 固有の設計制約チェック

プロンプト:

```
あなたはコードレビュアーです。以下の実装計画を「mdhop プロジェクト固有の設計制約」と照合し、違反がないか検証してください。

## 実装計画
{PLAN_CONTENT}

## 検証手順
1. プラン内で参照されている対象コードを Read で読む
2. 以下の設計制約リストとプランを照合する
3. 違反があれば指摘する

## mdhop 設計制約

以下はこのプロジェクトで繰り返し発見された設計上の落とし穴です。プランがこれらに抵触していないか検証してください。

### リンクパース・rawLink の仕様
1. **`!` プレフィックスは rawLink に含まれない**: `![[image.png]]` の rawLink は `[[image.png]]`。embed の `!` は rawLink の外。CLI 入力で `![[...]]` を受け取っても DB の rawLink とは一致しない
2. **filepath.Ext の罠 — `.md` のみ除去すべき箇所で全拡張子除去しない**: `filepath.Ext("Note.v1")` は `.v1` を返す。basename 生成・拡張子判定では `.md` のみを明示除去し、`filepath.Ext` で汎用除去しないこと
3. **wikilink は常に .md なし、markdown link は元リンクの拡張子有無を保持**: rewrite 時にこの規則を破らないこと

### ルート優先ルール（ADR 0004）
4. **ルート優先ルールの影響を全コマンドに波及させる**: basename 重複時、ルート直下にファイルがあれば `[[basename]]` はルートに解決する。新しい機能や仕様変更がこのルールに影響する場合、resolve / query / add / move / update / delete / disambiguate の全コマンドで整合性を確認すること
5. **move でルート優先の状態変化を検出する**: ファイルがルートに出入りすると `[[basename]]` の解決先が変わる。`isAmbiguousBasenameLink` だけでは検出できず、pre-move vs post-move のターゲットパス比較が必要

### DB・SQL の安全性
6. **upsertTag の name は原文保持、node_key は小文字正規化**: `LOWER()` でクエリする必要がある箇所で name を直接比較しないこと
7. **SQL の NULL 三値論理**: phantom / tag の path は NULL。`NOT (path GLOB ?)` は NULL 行を除外する。`(path IS NULL OR NOT (path GLOB ?))` にすること
8. **exists_flag フィルタ**: `type='note'` だけでは phantom（exists_flag=0）も含まれうる。実在ノードのみ必要な場合は `AND exists_flag=1` を追加すること
9. **modernc.org/sqlite の LastInsertId の罠**: `ON CONFLICT DO NOTHING` 時、`LastInsertId()` は前回挿入の rowid を返す。`RowsAffected()` を先にチェックすること

### Vault escape・パス安全性
10. **vault escape チェックは filepath.IsAbs + pathEscapesVault の両方必要**: `..` チェックのみでは絶対パスを弾けない。`filepath.Join(vaultPath, "/sub")` は vaultPath を無視して `/sub` になる
11. **vault 内判定で strings.HasPrefix は不安全**: `/vault` と `/vault2` を誤許可する。`filepath.Rel(vaultAbs, targetAbs)` で `..` 始まりでないことを検証すること

### Disk 操作の制約
12. **D5: disk-based operation は非 .md のみ**: delete --rm や move のディスク walk で `.md` ファイルを誤って削除・移動しないこと。WalkDir に `.md` 拡張子チェックを入れること
13. **破壊的操作の順序**: DB 操作 → ディスク操作の順。ディスク操作を先にすると未登録ファイルでも削除してしまう。`committed = true` は DB commit 成功後に設定すること

### 仕様変更時の影響分析
14. **既存テストへの影響を網羅的に洗い出す**: 仕様変更時は grep で既存テストの期待値変更を分析すること。ルート優先ルール導入時に9件の既存テスト更新漏れが発覚した前例あり
15. **overview.md との整合を確認する**: 新機能や仕様変更時は overview.md の記述と照合し、矛盾があればドキュメント更新をプランに含めること

上記に該当しないが mdhop 固有の設計判断に関わる問題も自由に指摘してよい。

## 出力形式
- 日本語で出力
- 指摘事項は箇条書きで、該当するコード・計画の箇所を引用する
- 指摘ごとに重要度を付ける: 🔴 MUST / 🟡 SHOULD / 🔵 NIT
- 問題がなければ「mdhop 固有の指摘なし」と記載する
```

### 4. 結果を出力する

エージェントの結果を以下の形式でユーザーに表示する:

```
## 自己レビュー結果（mdhop 固有）

### mdhop 設計制約チェック
{Agent 1 の結果}
```

スキル側ではプランの修正は行わない（呼び出し元に判断を委ねる）。

### 差分チェック（2回目以降の実行時）

このスキルがループ内で繰り返し呼ばれる場合、エージェントに以下の追加指示を含めること:

```
## 差分チェック指示
プランの記述に「対処済み」「明記済み」「トレードオフとして認識」等と読み取れる内容がある場合、その論点を再度指摘しないこと。
報告するのは **新規の指摘のみ**。既出の論点の言い換え・補足・「もっと明示的に書け」は NIT としても報告しない。
```
