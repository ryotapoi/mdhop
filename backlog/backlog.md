# Backlog

## 運用ルール

- Backlog には、今後着手でき、完了を判定できる作業を `- [ ]` 形式のタスクとして置く。
- 対応要否を決める調査もタスクにできる。観察事実だけを残さず、判断したい問いと、判断をもって完了とすることを明示する。
- 粗いメモで起票してよいが、着手前に目的と受け入れ条件を読める粒度へ詰める。
- バージョン番号を付ける場合は SemVer に従う。
- タスクは `###` 見出しでグループに分ける。バージョンが確定したグループだけ、`### v0.11.0 名前` のように見出しへ番号を付ける。
- グループが大きい場合は、`####` 見出しでさらに分割してよい。

## タスク

### v0.19.1 バグ修正・保守整備

#### 監査後の simplify-code 実行

- [ ] まず CODEX:HY-1、次に GROK:B20/B21、続いて CODEX:HY-3 + CODEX:A6-1 と GROK:N1 の文書整合を完了・検証してから、既存コード・test を対象に `$simplify-code` を実行する。
  - CODEX:HY-2、CODEX:A3-2/A3-3/A2-1、GROK:B9/N3/N4/B7 は検討入力であり、無条件の実装対象ではない。backlog 全件や GROK:D6 の完了は前提にしない。
  - 関連する test の観測強化は必要に応じて行い、全項目の先行完了を求めない。採用した簡素化は skill と project の方針に従って実装・検証し、残りの候補を完了・不要・継続必要に再評価して扱いを明記する。

#### 監査確認済みのバグ・保守負担

- [x] 書込み後に error となった現在ファイルも既存の backup/restore 対象に含め、復元不能 path を既存形式で報告する。原本・権限など既存の復元契約と DB 整合を確認し、mtime は Set の既存契約だけを保ち rewrite/Move に完全復元契約を新設しない。`internal/core/rewrite.go`、`move_apply.go`、`set.go` の既存 rollback 経路と、再現済み bug の代表的な最小 regression に限定し、新しい保存方式、全面的な atomic write、DI 層、test 専用 hook は導入しない。出典: CODEX:HY-1。完了: 部分書込み error 後の file 復元と既存形式の失敗報告を確認する。

- [x] move の source-relative 保持例外を上位要件へ明記し、現行実装・test と整合させる。`docs/rules/02-requirements.md`、`docs/specs/overview.md`、`internal/core/move_link.go` を対象とする docs-only の変更とし、挙動変更や既存 ADR 本文の書換えはしない。出典: CODEX:HY-3、CODEX:A6-1。完了: Move の既存相対リンク保持例外が両正本と既存 test に一致する。

- [ ] directory delete の入力展開、登録 file と DB 更新、未登録 asset cleanup を core の削除操作へ集約する。`cmd/mdhop/delete.go` から展開・Walk・Remove・cleanup を除き CLI は引数と出力だけを担い、`internal/core/delete.go`、`fs_cleanup.go`、`docs/specs/overview.md` を整合させる。単一 file 契約、未登録 directory error、hidden directory・未登録 Markdown の保持、post-update cleanup 失敗時に成功出力しない契約を維持し、既存 directory delete test の移設・再利用を主とする。汎用 FS service/new package は作らず、旧 ADR は履歴として扱う。出典: CODEX:HY-2。完了: directory delete の完了境界が core 一箇所となり、既存契約を確認する。

- [ ] move 情報と候補本文を単一 record に対応づけ、parallel slices の index 規約を除く。`internal/core/move_rewrite.go`、`move_apply.go`、`move_dir.go` を対象に、Move/MoveDir/MoveTemplate/asset/rollback の結果を不変に保ち既存 test を再利用する。新しい防御 check や framework は作らない。出典: CODEX:A3-3。完了: file と書換え本文の対応が型上の単一 record で表される。

- [ ] Simplify の除外設定について、先行 callback が外側変数へ書く依存を除き、scan で確定した設定を Prepare に明示して note と asset が同じ設定を参照するようにする。`internal/core/scan_rewrite.go`、`simplify.go` と callers を対象に、scan 系4 command の既存 exclude/dry-run 契約を維持する。新たな汎用 framework は作らない。出典: CODEX:A3-2。完了: callback 実行順序に依存せず既存設定で note/asset の除外結果が一致する。

- [ ] node/link type の実行時 SQL predicate を既存 Go 定数の引数と `linkTypeSQLIn` へ置換し、手動同期を減らす。`internal/core/stats.go`、`reachable.go`、`graph.go`、`frontmatter_path_guard.go` などの原指摘箇所と `db.go` を対象に、feature 別集合・意味を保ち既存 query test を再利用する。schema、新分類表、新 helper の一律導入はしない。出典: CODEX:A2-1、GROK:B9。完了: 各 predicate が既存定数由来の値を使い、既存 query の意味が不変である。

- [x] 曖昧 resolve、`--note`、overview 冒頭の command 一覧を現行詳細仕様と CLI に整合させる。`docs/rules/01-concept.md`、`02-requirements.md`、`docs/specs/overview.md`、`cmd/mdhop/query.go`、`main.go` を照合対象とし、候補一覧機能・flag は追加しない。出典: GROK:N1。完了: 記載された resolve/flag/command 一覧が現行 CLI と詳細仕様に一致する。

- [ ] `go.mod` の `golang.org/x/text` を direct dependency として整合させる。version は変更せず、既存の norm 依存を正しく宣言する。出典: GROK:N2。完了: `go mod tidy -diff` が差分なしとなる。

- [ ] 単一 link の escape/ambiguity 検証と既存 sentinel による原因判定を共有し、Build の inline 条件と文言 prefix 判定の重複を除く。`internal/core/build_prepare.go`、`build.go`、`link_ambiguity.go` を対象に、Build の複数 error 上限・順序・candidates/hint と mutation の最初の error・既存出力を保つ。単一 link validator の共有に限定し、mode 付き汎用 validator/new error 体系は作らず、既存 build/mutation/format test を再利用する。出典: GROK:N3、GROK:N4。完了: 同じ入力で共有検証を使いつつ各入口の既存出力契約を満たす。

- [x] Add の `ErrAddingMakesAmbiguous` に特定可能な衝突 basename を既存の `fmt.Errorf`/`%w` で付与する。`internal/core/add.go` と既存曖昧追加 test を対象に、機械分類と拒否時の無変更を保つ。汎用診断体系は作らない。出典: GROK:B20。完了: error から衝突 basename を特定でき、既存 sentinel 判定と state 不変を確認する。

- [x] `checkStale` で不存在だけを `ErrFileNotFound` にし、permission など他の `os.Stat` error は元 error を保持して返す。`internal/core/query_content.go` を対象に、再現済み権限誤診断の regression と既存 missing/stale test を最小層で確認する。新しい FS wrapper/hook は追加しない。出典: GROK:B21。完了: missing と権限 error が区別され、既存 stale 契約を満たす。

#### テストの観測強化

- [ ] `query --where --no-exclude` の既存 CLI test で JSON の選択 backlinks を断言する。`cmd/mdhop/cli_test.go` を対象に、通常 where の既存 E2E と重複させず弱い case を置換または統合し、production は変更しない。出典: GROK:N6。完了: `--no-exclude` 分岐の JSON backlinks が直接観測される。

- [ ] `MoveDir_PhantomPromotion` fixture を実際に promotion が起きるものへ修正し node/edge 結果を確認する。`internal/core/move_dir_test.go` の HiddenFiles では hidden `.DS_Store` が移動先に存在せず移動元に残る現行除外契約を確認し、既に検証済み NonMD を重複追加しない。既存 case の修正だけとし production は変更しない。出典: GROK:N7。完了: promotion と hidden file 除外が fixture と assertion で直接確認される。

- [ ] `AllowsSafeRewriteCandidate` の既存 case で計画の予定先、移動後 file/DB、quoted link 結果を必要最小限確認する。`internal/core/move_template_test.go` の既存 fixture を再利用し、別 E2E は追加せず production は変更しない。出典: GROK:N8。完了: 成功時の計画・disk・index・link 更新を観測し no-op/部分適用を検知する。

- [ ] repair の frontmatter 保持既存 test の bare `[[...]]` を、実際に `frontmatter_wikilink` と判定される quoted fixture へ修正し、body 修復と frontmatter 不変を観測する。`internal/core/repair_test.go` を変更対象、`rewrite_test.go` と `repair.go` を参照対象とし、分類表列追加や同義 helper test は作らず production は変更しない。出典: GROK:B10。完了: quoted frontmatter を残して body のみ修復することを確認する。

- [ ] Add self-link の既存 test fixture を root-priority の早期 skip に隠れないものへ修正し、self-link の意味と追加 node/edge 状態を直接確認する。`internal/core/add_test.go` の既存 case 置換だけとし production は変更しない。出典: GROK:B25。完了: self-link 追加後の node/edge 状態が期待どおりである。

- [ ] Update batch の既存 test で Deleted/Phantomed のどちらでもよい断言を正しい完全削除へ固定し、参照先 path を確認する。`internal/core/update_test.go` の既存 case 修正のみとし production は変更しない。出典: GROK:B26。完了: 対象 node が完全削除され、参照先 path が期待どおりである。

- [ ] 公開 `PlanMoveTemplate` に空 Template、絶対 From、`../` による vault 外 From を渡す代表 table test で拒否を固定する。`internal/core/move_template_test.go` で既存 `newMoveVault` と `filepath` を再利用し、error 全文一致でなく失敗理由を識別して確認する。新 fixture 基盤/hook/DI/private 関数直 test/MoveTemplate との重複検証は不要。出典: GROK:N9。完了: 3種類の誤入力が公開入口で適切に拒否される。

- [ ] 警告付き成功の `runAdd`（または `runUpdate`）を `--format json` で実行し、stdout 単独の JSON parse、stderr のみの warning、操作成功を代表1 case で確認する。`cmd/mdhop/cli_test.go`、`format.go`、`internal/core/add_test.go` で既存 capture と `date: not-a-date` fixture を再利用し、subprocess/E2E 基盤・production 変更・全 command への複製はしない。出典: GROK:B28。完了: warning 付き JSON 成功時の stream 分離と成功結果を確認する。

#### リファクタリング候補

- [ ] repair/directory meta-check の vault escape 判定について、用途差と既存低水準 helper 共有を踏まえ、共有で変更箇所・読み解く条件が実際に減るか比較する。`internal/core/repair.go`、`meta_check.go`、`link_resolver.go` を対象に、現行の誤判定は未確認とし、特例・抽象化が増えるなら見送る。出典: GROK:B7。完了: 採用/見送りの理由と、採用時の最小範囲を決める。

### 新機能：関連検索のハブ除外

- [ ] 多くの note とつながる経由 note を除外して関連検索のノイズを抑える機能について、利用場面、除外対象・接続数の数え方、設定/既定値、既存 `max_via_per_target` との関係、受入例を検討する。`docs/rules/02-requirements.md`、`internal/core/query.go`、`query_fetch.go`、`cmd/mdhop/query.go` を対象に、`via_max_degree` の flag 名や具体アルゴリズムは先に固定せず、仕様確定前に実装へ進まない。出典: GROK:D6。完了: 仕様と実装 scope を確定する。
