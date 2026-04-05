---
name: review-code-mdhop
description: mdhop 固有の設計制約に基づく実装レビュー。通常はチェーンスキルから呼ばれる。
argument-hint: <plan-file-path>
allowed-tools: Read, Glob, Grep, Bash(git diff *), Bash(git log *), Bash(git status *), Task
context: fork
---

# Self Implementation Review — mdhop Project

グローバルの `/review-code` + `/review-code-go` の後に追加実行する mdhop プロジェクト固有の実装レビュー。
1つの Plan サブエージェントで実行する。

**重要な制約:**
- 使用できるツール: Read, Glob, Grep, Bash(git diff/log/status), Task **のみ**
- レビューは Task ツール（subagent_type: Plan）で実行する。自分で直接レビューしない
- **結果はファイルに書き出さない。テキストとして返すだけにすること。/tmp やプロジェクト配下へのファイル作成は行わない**

## 手順

### 1. レビュー対象の差分を取得する

- `git diff` と `git diff --cached` で未コミットの変更差分を取得する
- `git status` で変更ファイル一覧を取得する
- 差分がなければ「レビュー対象の変更がありません」と返して終了する
- 変更ファイルに `.go` ファイルが含まれていなければ「Go ファイルの変更がないためスキップします」と返して終了する

### 2. Plan サブエージェントを起動する

Task ツールで `subagent_type: Plan, model: "sonnet"` を使う。

エージェントのプロンプトには、手順1で取得済みの以下の値を埋め込む:
- `{GIT_DIFF}`: 手順1で取得した変更差分（git diff + git diff --cached の結合出力）
- `{FILE_LIST}`: 手順1で取得した変更ファイル一覧（git status の出力）

加えて、「変更されたファイルの全文は自分で Read/Grep/Glob して確認すること」という指示を含める。

#### Agent 1: mdhop 固有の設計制約チェック

プロンプト:

```
あなたはコードレビュアーです。以下の実装変更を「mdhop プロジェクト固有の設計制約」と照合し、違反がないか検証してください。

## 変更差分
{GIT_DIFF}

## 変更ファイル一覧
{FILE_LIST}

## 検証手順
1. 変更されたファイルを Read で読み、変更の全体像を把握する
2. 以下の設計制約リストと実装を照合する
3. 違反があれば指摘する

## mdhop 設計制約

以下はこのプロジェクトで繰り返し発見された設計上の落とし穴です。実装がこれらに抵触していないか検証してください。

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

### テスト検証の網羅性
14. **テストでは DB edge の raw_link も検証する**: ファイル内容の書き換え検証だけでなく、DB 側の edge 更新（raw_link 値）が正しいか確認すること
15. **仕様変更時は既存テストの期待値変更を grep で網羅的に洗い出す**: 見落とすと旧仕様の期待値でテストが通り続けるリスクがある

### 新規ノードタイプ・機能追加時
16. **既存ループの早期 return / continue を全件チェックする**: 新しいノードタイプ（asset 等）を追加した場合、既存の for ループが `continue` で新タイプをスキップしていないか確認すること
17. **新しい map / キャッシュを追加したら、既存の調整ループでも同様に調整する**: rootBasenameToPath のような map を追加した場合、ディスク欠損調整等の既存ループにも反映が必要

### stdout の安定インターフェース
18. **stdout JSON に新フィールドを追加していないか**: overview.md で定義された JSON フィールド以外を stdout に追加するのは agent 向け安定 IF の破壊。warnings 等の付加情報は stderr に出力すること（Build と同じパターン）

### テスト検証の網羅性（ミューテーション系コマンド）
19. **build のテスト観点が add/update/delete にも適用されているか**: エラーメッセージ改善・出力フォーマット変更等で build_test のみ検証し add_test/update_test が漏れるパターンが繰り返し発生。変更が複数コマンドに影響する場合は全コマンドのテストを確認すること

### config 読み込み
20. **config 読み込みが条件付きになっているか**: MetaConfig 等の設定が必要なフラグ（--where 等）が使われていない場合、壊れた yaml で新たにエラーを出さないよう条件付きロードになっていること

### モジュール配置・構造
21. **モジュール配置と依存方向の遵守**: 新しい import が `cmd/mdhop → internal/core` の方向に従っているか。`internal/core` が `cmd/mdhop` に依存していないか（rules/architecture.md 参照）
22. **共通化の妥当性**: `cmd/mdhop` と `internal/core` 間で共有するコードが `internal/core` に正しく配置されているか。`cmd/mdhop` のローカルなヘルパーが本来 `internal/core` に属する概念を扱っていないか
23. **リファクタリングと機能実装のコミット分離**: diff にリファクタリング（rename、ファイル移動、構造変更）と機能実装（新しいビジネスロジック）が混在していないか

上記に該当しないが mdhop 固有の設計判断に関わる問題も自由に指摘してよい。

## 出力形式
- 日本語で出力
- 指摘事項は箇条書きで、該当するコードの箇所を引用する
- 指摘ごとに重要度を付ける: 🔴 MUST / 🟡 SHOULD / 🔵 NIT
- 問題がなければ「mdhop 固有の指摘なし」と記載する
```

### 3. 結果を出力する

エージェントの結果を以下の形式でユーザーに表示する:

```
## 自己レビュー結果（mdhop 固有）

### mdhop 設計制約チェック
{Agent 1 の結果}
```

スキル側ではコードの修正は行わない（呼び出し元に判断を委ねる）。

### 差分チェック（2回目以降の実行時）

このスキルがループ内で繰り返し呼ばれる場合、エージェントに以下の追加指示を含めること:

```
## 差分チェック指示
実装に「対処済み」「意図的な判断」と読み取れるコード・コメントがある場合、その論点を再度指摘しないこと。
報告するのは **新規の指摘のみ**。既出の論点の言い換え・補足・「もっと明示的に書け」は NIT としても報告しない。
```
