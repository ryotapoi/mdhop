# mdhop プロジェクト固有の設計制約（プランレビュー）

mdhop プロジェクトに固有の設計制約・命名規則・既存実装パターンと、プランを照合して違反がないか検証する観点。

## 検証手順

1. プラン内で参照されている対象コードを Read で読む
2. 必要に応じて `rules/architecture.md`, `rules/01-concept.md`, `rules/02-requirements.md`, `rules/03-data-model.md`, `rules/overview.md` を Read で読む
3. 関連する `decisions/` の ADR（特に 0004 ルート優先ルール、0008 move collateral rewrite、0011 asset node 等）を必要に応じて Read で読む
4. 以下の設計制約リストとプランを照合する
5. 違反があれば指摘する

## mdhop 設計制約

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
16. **stdout JSON に新フィールドを追加する場合は仕様変更として扱う**: overview.md で定義された JSON フィールド以外を stdout に追加するのは agent 向け安定 IF の破壊。warnings 等の付加情報は stderr に出力すること（Build と同じパターン）
17. **build でテストする観点は add/update/delete でも同様にテストする**: エラーメッセージ改善・出力フォーマット変更等で build_test のみテスト計画を書き、add_test/update_test を漏らすパターンが繰り返し発生。変更が build 以外のミューテーション系コマンドにも影響する場合は全コマンドのテスト計画を含めること
18. **config 読み込みは必要な場合のみに限定する**: MetaConfig 等の設定が必要なフラグ（--where 等）が使われていない場合、壊れた yaml で新たにエラーを出さないよう条件付きロードにすること

### モジュール配置・構造
19. **モジュール配置は依存方向 `cmd/mdhop → internal/core` に従う**: 新しいコードの配置先が `rules/architecture.md` の責務定義と合っているか。定義された方向に違反する依存がないか
20. **共通化は依存方向に沿って配置する**: `cmd/mdhop` と `internal/core` 間で共有するコードは `internal/core` に置く。片方だけ変更したくなったとき分離できるか検討されているか
21. **リファクタリングと機能実装を同一ステップに混ぜない**: 既存コードの構造改善が必要なら、機能実装の前ステップとして分離されているか

### 派生ドキュメント・互換性
22. **派生ドキュメントの更新**: CLI 表面仕様の変更に伴い、`README.md`, `README.ja.md`, `examples/` 配下の SKILL.md 等の派生ドキュメントの更新がプランに含まれているか
23. **互換性への影響の明示**: 出力フォーマット変更が既存の利用パターン（スクリプト連携、TSV パース等）に影響する場合、破壊的変更として明示されているか

上記に該当しないが mdhop 固有の設計判断に関わる問題も自由に指摘してよい。
