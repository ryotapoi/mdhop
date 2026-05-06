# Goal Decisions

ユーザー意図と違う可能性がある自動判断の記録。マスターが後でまとめて確認する。

## Format

各エントリは選択肢ベースで書く。マスターが「今回の選択が妥当か / 妥当でないか」を判断できる形にする。

```
### YYYY-MM-DD HH:mm - <判断タイトル>

- 対象タスク:
- 論点: 何を選ぶ必要があったか（短く）
- 選択肢:
  - 案A: 〜 — こういう設計判断 / タスク管理を優先するなら案A
  - 案B: 〜 — こういう設計判断 / タスク管理を優先するなら案B
- 選んだ案: 案X
- 理由: 〜
```

書き方のルール:

- 「選択肢」は実際に検討したものを書く（後付けで作らない）
- 各案の判断軸は **設計判断 / タスク管理レベル** で書く
  - 設計判断の例: 「YAGNI を優先 / 後の手戻り回避を優先」「整合性を優先 / 局所修正を優先」「型安全を優先 / シグネチャ変更回避を優先」
  - タスク管理の例: 「今のタスクで一緒にやる / 別タスクに切る」「v0.6.1 のスコープを守る / スコープを広げる」「即時解消 / 横断対応待ち」
  - コードレベル（「`fmt.Sprintf` を使う / 使わない」「変数名を X にする / Y にする」）には落とさない
- 「理由」は選んだ理由・選ばなかった理由のどちらか / 両方を必要なだけ書く（項目は分けない）
- 「ユーザー意図と違う可能性」「後で確認してほしい点」「派生アクション」のような曖昧な項目は書かない。選択肢と理由が読めればマスターが判断できる
- マスターが「妥当」と返せば履歴として残るだけ、「妥当でない」と返せばマスター指示で別タスク化や修正コミットが入る

派生タスクは `goal-decisions.md` には書かず、`backlog/backlog.md` に追記する（→ `.claude/goal/workflow.md`「Task SSoT」）。

### 2026-05-06 22:55 - NodeType 昇格時のテスト定数化スコープ

- 対象タスク: ノード型を `type NodeType string` の named type に昇格
- 論点: テストコードで `Node.Type != "note"` のような untyped string リテラル比較が多数（resolve_test.go / asset_test.go / query_test.go / search_test.go / query_exclude_test.go 計 50 箇所超）残存している。今回スコープに含めるか
- 選択肢:
  - 案A: 全テストの `Type != "note"` 系も `NodeTypeNote` 定数で書き換え — 「型昇格の意図をテストにも徹底」設計判断を優先
  - 案B: レビュー指摘で挙がった範囲（build_test.go の `targetType` 派生 + cli_test.go + query_test.go の `wantTypes`）に絞り、残る `Type != "note"` 系は据え置き — 「タスクスコープを守り、範囲爆発を避ける」タスク管理を優先
- 選んだ案: 案B
- 理由: untyped string constant と `NodeType` の比較はコンパイル・実行ともに正しく動き実害がない。今回のタスクは「Go 側引数・構造体・SQL バインドの両端を整合させる」が主眼で、テストコード内の比較リテラルすべてを定数化することは含まれない。範囲を広げると 50+ 箇所の機械的修正でレビュー対象が膨らむ。残存箇所は別タスクで一括処理する余地を残す

### 2026-05-06 - MoveDir ロールバックテストのレビュー深さを self-check に絞った

- 対象タスク: `internal/core/move_dir.go` の `MoveDir` ロールバックパスのテスト追加
- 論点: タスクは複数ファイル横断の `move_dir.go` に網を張るためレビュー的には非 Small。並列レビュー（review-code-all）に回すか、self-check で済ますか
- 選択肢:
  - 案A: `/review-code-all` に通して並列レビュー（design / facts / go / mdhop） — 「mdhop コア領域への変更は深いレビューを通す」設計判断を優先
  - 案B: self-check で済ます（既存テスト全 pass + テスト追加のみ + プロダクトコード変更なし） — 「review.md の『軽微なら self-check に落とせる』を適用、ループの実装速度を優先」タスク管理を優先
- 選んだ案: 案B
- 理由: 変更は `move_test.go` 末尾への 2 テスト追加のみで、`MoveDir` 本体・ヘルパーには触れていない。既存 388+ テストが全 pass し、新規 2 テストはロールバック後の disk 状態 / DB 状態 / 外部ファイル内容を最終状態のみで検証する形（順序非依存）。仕様変更も挙動変更もないためレビュー観点の MUST / SHOULD 指摘が出る余地が薄く、並列レビューのコストに見合わない

### 2026-05-06 - MoveDir ロールバックテストのカバー戦略を「失敗トリガ x 2 種類」に絞った

- 対象タスク: `internal/core/move_dir.go` の `MoveDir` ロールバックパスのテスト追加
- 論点: ロールバック発火点は `move_dir.go` 行 583-590（Phase 4.2 内 inline rollback）と行 601-620（defer 内 rollback：rename 巻き戻し + moved file restore + external restore）の 2 系統。テストでどう網を張るか
- 選択肢:
  - 案A: 583-590 と 611-617 をそれぞれ独立に発火させる 2 + α テストを書く（Phase 4.2 の write 失敗を直接起こすため、特定 moved file の `os.Chmod 0o400` 等で個別失敗を起こす） — 「行ごとの直接カバレッジ」設計判断を優先
  - 案B: 「outRewrites が積まれない rename 失敗」+「outRewrites が積まれた状態の rename 失敗」の 2 ケースで、ロールバック後の最終状態（disk / DB / 外部ファイル）が pre-move と一致することを検証 — 「行ごとカバレッジに固執せず、ロールバックの実害（state が壊れる）を捉える網にする」設計判断を優先
- 選んだ案: 案B
- 理由: 583-590 と 611-617 はロジック的にほぼ同じ（外部 backup 復元 + moved file content 復元 + rename 巻き戻し）。Phase 4.2 の早期 rollback と defer 内 rollback の違いは「completedRenames が空かどうか」「movedFileBackups の要素数」で、両方とも rollback 後の最終状態は「pre-move と等価」が要件。状態の等価性で検証すれば、ロジックのどちらが壊れても検出できる。Phase 4.2 内で write を確実に失敗させるには moved file 単位の chmod が必要だが、`m.from` の chmod 0o400 では `writeFilePreservePerm` の `os.WriteFile` (O_WRONLY|O_CREATE|O_TRUNC) が macOS で意図通り失敗するか不安定（実測で失敗しなかった）。代わりに「outRewrites が積まれた直後に rename を失敗させる」シナリオで、defer 内 moved file restore (611-617) が動く経路を網羅した

### 2026-05-06 23:36 - sentinel error 化のスコープを意味的識別ニーズのある範囲に絞った

- 対象タスク: `internal/core` の sentinel error 化
- 論点: `internal/core` には `fmt.Errorf` が 94 箇所ある。全部を機械的に `%w` ラップして sentinel 化するか、識別ニーズのある範囲に絞るか
- 選択肢:
  - 案A: 全 94 箇所を sentinel + `%w` 化 — 「将来の identification ニーズに先回り」設計判断を優先
  - 案B: 「呼び出し元または将来の呼び出し元が分岐したい意味的エラー」14 種に絞り、構文エラー・YAML パース・CLI バリデーション系は対象外 — 「YAGNI、識別ニーズが見えてから足す」設計判断を優先
- 選んだ案: 案B
- 理由: 識別ニーズのない sentinel を増やしても、可読性が下がるだけで利点がない。今回明確な識別ニーズがあるのは file/link 解決系・stale 検出・vault escape の 14 種で、構文エラー（`where.go` `convert.go` の `--ToFormat`）や YAML パースエラー（`config.go` `init_meta.go` 既に `%w` 済）は呼び出し元が `errors.Is` で分岐していないため除外した。`query_fetch.go:438` の `"stale index"` は `move/disambiguate` 系の `"... is stale"` と意味が異なるため別カテゴリとしてスコープ外。`delete.go:90` の `"path escapes vault"` と `move_helpers.go:96,153` の `"rewritten link would escape vault"` はメッセージが他の `link escapes vault` と異なるため、メッセージ完全保持制約の下で別 sentinel が必要になる。識別ニーズが出たら別タスクで追加する
