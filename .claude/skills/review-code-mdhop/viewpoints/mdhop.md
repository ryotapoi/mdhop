# mdhop プロジェクト固有の設計制約（実装レビュー）

mdhop プロジェクトに固有の設計制約・命名規則・既存実装パターンと、実装差分を照合して違反がないか検証する観点。

## 検証手順

1. 変更されたファイルを Read で読み、変更の全体像を把握する
2. 必要に応じて `rules/architecture.md`, `rules/01-concept.md`, `rules/02-requirements.md`, `rules/03-data-model.md`, `rules/overview.md` を Read で読む
3. 関連する `decisions/` の ADR（特に 0004 ルート優先ルール、0008 move collateral rewrite、0011 asset node 等）を必要に応じて Read で読む
4. 以下の設計制約リストと実装を照合する
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
10. **SQL 集約関数の意味的正しさ**: MAX/MIN 等の集約関数が字句順ではなく意味的に正しい代表値を返すか。特に文字列カラムの MAX は辞書順最大値であり、「最新」や「代表」とは限らない

### Vault escape・パス安全性
11. **vault escape チェックは filepath.IsAbs + pathEscapesVault の両方必要**: `..` チェックのみでは絶対パスを弾けない。`filepath.Join(vaultPath, "/sub")` は vaultPath を無視して `/sub` になる
12. **vault 内判定で strings.HasPrefix は不安全**: `/vault` と `/vault2` を誤許可する。`filepath.Rel(vaultAbs, targetAbs)` で `..` 始まりでないことを検証すること

### Disk 操作の制約
13. **D5: disk-based operation は非 .md のみ**: delete --rm や move のディスク walk で `.md` ファイルを誤って削除・移動しないこと。WalkDir に `.md` 拡張子チェックを入れること
14. **破壊的操作の順序**: DB 操作 → ディスク操作の順。ディスク操作を先にすると未登録ファイルでも削除してしまう。`committed = true` は DB commit 成功後に設定すること

### テスト検証の網羅性
15. **テストでは DB edge の raw_link も検証する**: ファイル内容の書き換え検証だけでなく、DB 側の edge 更新（raw_link 値）が正しいか確認すること
16. **仕様変更時は既存テストの期待値変更を grep で網羅的に洗い出す**: 見落とすと旧仕様の期待値でテストが通り続けるリスクがある
17. **build のテスト観点が add/update/delete にも適用されているか**: エラーメッセージ改善・出力フォーマット変更等で build_test のみ検証し add_test/update_test が漏れるパターンが繰り返し発生。変更が複数コマンドに影響する場合は全コマンドのテストを確認すること
18. **テストが正しい仕様を検証しているか**: テストが「現在の実装の動作」を追認しているだけでなく、「意図した仕様」を検証しているか。実装のバグをテストが正として固定化していないか

### 新規ノードタイプ・機能追加時
19. **既存ループの早期 return / continue を全件チェックする**: 新しいノードタイプ（asset 等）を追加した場合、既存の for ループが `continue` で新タイプをスキップしていないか確認すること
20. **新しい map / キャッシュを追加したら、既存の調整ループでも同様に調整する**: rootBasenameToPath のような map を追加した場合、ディスク欠損調整等の既存ループにも反映が必要

### stdout の安定インターフェース
21. **stdout JSON に新フィールドを追加していないか**: `overview.md` で定義された JSON フィールド以外を stdout に追加するのは agent 向け安定 IF の破壊。warnings 等の付加情報は stderr に出力すること（Build と同じパターン）

### Usage / config
22. **Usage / ヘルプ文字列の網羅性**: 新しいフラグやモードが usage 定数・ヘルプ文字列に反映されているか。既存の usage に元々欠落があった場合でも、今回追加したフラグは含めること
23. **config 読み込みが条件付きになっているか**: MetaConfig 等の設定が必要なフラグ（--where 等）が使われていない場合、壊れた yaml で新たにエラーを出さないよう条件付きロードになっていること

### 実装の正確性
24. **ループ内の時刻取得**: 複数ファイル・レコードを処理するループで、タイムスタンプをループ外で1回だけ取得していないか。処理時間が長い場合、各反復で取得すべき

### モジュール配置・構造
25. **モジュール配置と依存方向の遵守**: 新しい import が `cmd/mdhop → internal/core` の方向に従っているか。`internal/core` が `cmd/mdhop` に依存していないか（`rules/architecture.md` 参照）
26. **共通化の妥当性**: `cmd/mdhop` と `internal/core` 間で共有するコードが `internal/core` に正しく配置されているか。`cmd/mdhop` のローカルなヘルパーが本来 `internal/core` に属する概念を扱っていないか
27. **リファクタリングと機能実装のコミット分離**: diff にリファクタリング（rename、ファイル移動、構造変更）と機能実装（新しいビジネスロジック）が混在していないか

上記に該当しないが mdhop 固有の設計判断に関わる問題も自由に指摘してよい。
