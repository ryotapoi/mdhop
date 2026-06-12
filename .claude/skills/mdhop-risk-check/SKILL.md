---
name: mdhop-risk-check
description: mdhop 固有の plan / 実装チェック。CLI 仕様、SQLite/SQL、リンク解決、ルート優先ルール、vault パス、破壊的処理（delete --rm / move / rewrite 系）、stdout JSON、モジュール境界に触れる変更で使う。汎用レビューではなく mdhop 固有の実害に絞って確認する。
---

# mdhop Risk Check

## Intent

mdhop 固有のプロダクト制約・アーキテクチャ制約・既知の落とし穴に照らして、計画または実装のリスクを確認する。

## Execution

main で直接チェックを回さず、fork 構造で実行する。main の context を汚さず、最終判断（修正・同期）は main に残すための分離。

1. **監督起動**: main は `Agent` ツールで risk-check 監督を 1 体起動する（`model: opus`）。Opus を使うのは観点クラスタへの振り分けと結果の dedup・統合に判断が要るため。prompt には次を渡す:
   - 対象: plan ファイルのパス / 未コミット差分 / commit range（`base..HEAD`）のいずれか。
   - 参照すべきパス: 関連 `rules/`、`specs/`（あれば）、`references/knowledge.md`、関連 ADR（特に 0004 / 0008 / 0011）。
2. **観点クラスタへの並列振り分け**: 監督は下の Checkpoints を観点クラスタに分け、subagent 2〜5 体（`model: sonnet` を明示）に振り分けて並列起動する。クラスタ例:
   - (a) Mission / Scope（Intent・対象範囲・仕様/CLI 挙動判断の要否）
   - (b) Architecture / 依存方向（Checkpoints 25–27、`rules/architecture.md`）
   - (c) ドメイン semantics（リンクパース・ルート優先・DB/SQL・vault escape・disk 操作・stdout IF: Checkpoints 1–15）
   - (d) 既知の落とし穴 + `knowledge.md` 照合（Checkpoints 16–24, 28–29 と過去知見）
   - 対象が小さい場合は観点をまとめて体数を減らしてよい（最小 2 体）。
3. **各 subagent への指示（必須）**: 「ファイルパス・行番号つきの事実と該当 Checkpoint 番号のみ返す。推測・提案・『推奨事項』セクションは含めない」を必ず渡す。判断は監督と main 側で行う。
4. **統合**: 監督は各 subagent の結果を dedup し、🔴（実害確定・要対応）/ 🟡（要確認）/ 🔵（軽微・任意）を付けて一覧に統合して返す。監督は修正を一切行わない。固有の指摘がなければ「固有の指摘なし（LGTM）」を返す。
5. **main 側の責務**: 修正と、`specs/` / `backlog/backlog.md` / `decisions/` / `references/knowledge.md` への同期判断は main 側で行う。

## Constraints

- 汎用レビューではなく、mdhop 固有の実害に絞る。一般的なコード品質・構造劣化は汎用レビュー側で見る（Claude では `/code-review` / `thermo-nuclear-code-quality-review`）。
- 仕様・CLI 挙動の判断が必要なら、実装判断として決めずユーザー確認に回す。
- 具体的な過去知見は `references/knowledge.md` を参照し、skill 本体には増やしすぎない。
- plan / 実装どちらのレビューでも使える。対象は plan ファイル、または未コミット差分 / commit range。
- Checkpoints と対象を照合する際、必要に応じて `rules/` と関連 ADR（特に 0004 ルート優先、0008 move collateral rewrite、0011 asset node）を Read で読む。

## Acceptance

- `LGTM` またはリスク一覧がある。
- リスクには影響、根拠、推奨対応がある。
- 必要な場合、更新すべき `rules/`, `backlog/backlog.md`, `decisions/`, `references/knowledge.md`, `specs/`（あれば）が明確。

## Relevant

- ユーザー依頼、plan、または変更差分（未コミット / commit range）
- `rules/01-concept.md`, `rules/02-requirements.md`, `rules/03-data-model.md`
- `rules/overview.md`
- `rules/architecture.md`
- `decisions/`
- `references/knowledge.md`

## Checkpoints

監督はこの観点リストを Execution のクラスタに振り分けて subagent に渡す。各 subagent は担当 Checkpoint の番号で指摘を返す。

### リンクパース・rawLink の仕様

1. **`!` プレフィックスは rawLink に含まれない**: `![[image.png]]` の rawLink は `[[image.png]]`。embed の `!` は rawLink の外。CLI 入力で `![[...]]` を受け取っても DB の rawLink とは一致しない。
2. **filepath.Ext の罠**: `filepath.Ext("Note.v1")` は `.v1` を返す。basename 生成・拡張子判定では `.md` のみを明示除去し、`filepath.Ext` で汎用除去しない。
3. **wikilink は常に `.md` なし、markdown link は元リンクの拡張子有無を保持**: rewrite 時にこの規則を破らない。

### ルート優先ルール（ADR 0004）

4. **影響を全コマンドに波及させる**: basename 重複時、ルート直下にファイルがあれば `[[basename]]` はルートに解決する。このルールに影響する変更は resolve / query / add / move / update / delete / disambiguate の全コマンドで整合性を確認する。
5. **move でルート優先の状態変化を検出する**: ファイルがルートに出入りすると `[[basename]]` の解決先が変わる。`isAmbiguousBasenameLink` だけでは検出できず、pre-move vs post-move のターゲットパス比較が必要。

### DB・SQL の安全性

6. **upsertTag の name は原文保持、node_key は小文字正規化**: `LOWER()` でクエリすべき箇所で name を直接比較しない。
7. **SQL の NULL 三値論理**: phantom / tag の path は NULL。`NOT (path GLOB ?)` は NULL 行を除外する。`(path IS NULL OR NOT (path GLOB ?))` にする。
8. **exists_flag フィルタ**: `type='note'` だけでは phantom（exists_flag=0）も含まれうる。実在ノードのみ必要なら `AND exists_flag=1` を追加する。
9. **modernc.org/sqlite の LastInsertId の罠**: `ON CONFLICT DO NOTHING` 時、`LastInsertId()` は前回挿入の rowid を返す。`RowsAffected()` を先にチェックする。
10. **SQL 集約関数の意味的正しさ**: 文字列カラムの MAX は辞書順最大値であり、「最新」や「代表」とは限らない。

### Vault escape・パス安全性

11. **vault escape チェックは filepath.IsAbs + pathEscapesVault の両方必要**: `..` チェックのみでは絶対パスを弾けない。`filepath.Join(vaultPath, "/sub")` は vaultPath を無視して `/sub` になる。
12. **vault 内判定で strings.HasPrefix は不安全**: `/vault` と `/vault2` を誤許可する。`filepath.Rel(vaultAbs, targetAbs)` で `..` 始まりでないことを検証する。

### Disk 操作の制約

13. **D5: disk-based operation は非 .md のみ**: `delete --rm` や `move` のディスク walk で `.md` ファイルを誤って削除・移動しない。WalkDir に `.md` 拡張子チェックを入れる。
14. **破壊的操作の順序**: DB 操作 → ディスク操作の順。ディスク操作を先にすると未登録ファイルでも削除してしまう。`committed = true` は DB commit 成功後に設定する。

### stdout の安定インターフェース

15. **stdout JSON に未定義フィールドを追加しない**: `rules/overview.md` で定義された JSON フィールド以外を stdout に追加するのは agent 向け安定 IF の破壊（仕様変更として扱う）。warnings 等の付加情報は stderr に出力する（Build と同じパターン）。

### Usage / config

16. **Usage / ヘルプ文字列の網羅性**: 新しいフラグやモードが usage 定数・ヘルプ文字列に反映されているか。既存の usage に欠落があっても、今回追加したフラグは含める。
17. **config 読み込みは条件付きにする**: MetaConfig 等の設定が必要なフラグ（`--where` 等）が使われていない場合、壊れた yaml で新たにエラーを出さないよう条件付きロードにする。

### 新規ノードタイプ・機能追加時

18. **既存ループの早期 return / continue を全件チェックする**: 新しいノードタイプ（asset 等）を追加した場合、既存の for ループが `continue` で新タイプをスキップしていないか確認する。
19. **新しい map / キャッシュは既存の調整ループにも反映する**: rootBasenameToPath のような map を追加した場合、ディスク欠損調整等の既存ループにも反映が必要。
20. **ループ内の時刻取得**: 複数ファイル・レコードを処理するループで、タイムスタンプをループ外で 1 回だけ取得していないか。処理時間が長い場合、各反復で取得すべき。

### テスト検証の網羅性

21. **DB edge の raw_link も検証する**: ファイル内容の書き換え検証だけでなく、DB 側の edge 更新（raw_link 値）が正しいか確認する。
22. **仕様変更時は既存テストの期待値変更を grep で網羅的に洗い出す**: 見落とすと旧仕様の期待値でテストが通り続ける。ルート優先ルール導入時に 9 件の更新漏れが発覚した前例あり。
23. **build の観点を add/update/delete にも適用する**: エラーメッセージ改善・出力フォーマット変更等で build_test のみ検証し add_test / update_test が漏れるパターンが繰り返し発生。変更が複数コマンドに影響する場合は全コマンドのテスト（計画）を確認する。
24. **テストが意図した仕様を検証しているか**: 「現在の実装の動作」の追認になっていないか。実装のバグをテストが正として固定化していないか。

### モジュール配置・構造

25. **依存方向 `cmd/mdhop → internal/core` の遵守**: 新しい import がこの方向に従っているか。`internal/core` が `cmd/mdhop` に依存していないか（`rules/architecture.md` 参照）。
26. **共通化は依存方向に沿って配置する**: `cmd/mdhop` と `internal/core` 間で共有するコードは `internal/core` に置く。`cmd/mdhop` のローカルなヘルパーが本来 `internal/core` に属する概念を扱っていないか。
27. **リファクタリングと機能実装を混ぜない**: diff / plan のステップに構造変更と新しいビジネスロジックが混在していないか。必要なら先行リファクタとして分離する。

### 派生ドキュメント・互換性

28. **派生ドキュメントの更新**: CLI 表面仕様の変更に伴い、`rules/overview.md`, `README.md`, `README.ja.md`, `examples/` 配下の SKILL.md 等の更新が含まれているか。
29. **互換性への影響の明示**: 出力フォーマット変更が既存の利用パターン（スクリプト連携、TSV パース等）に影響する場合、破壊的変更として明示されているか。

上記に該当しないが mdhop 固有の設計判断に関わる問題も自由に指摘してよい。
