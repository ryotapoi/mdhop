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

### 2026-05-07 - MoveDir 分解で Phase 4.3 + Phase 5 を本体に残した

- 対象タスク: `internal/core/move_dir.go` の `MoveDir`（758 行 1 関数）を分解
- 論点: Phase 4.3（disk rename + completedRenames 蓄積）と Phase 5（DB transaction + 5.1〜5.5）も段階別ヘルパーに切り出すか、MoveDir 本体に残すか
- 選択肢:
  - 案A: Phase 4.3・Phase 5 もそれぞれヘルパー化（例: `executeRenames` / `commitMoveTransaction`）— 「全段階を等しく分離」設計判断を優先
  - 案B: Phase 4.3・Phase 5 は本体に残し、defer rollback と組み合わせて記述する — 「rollback の状態 (completedRenames / movedFileBackups / externalBackups / committed) が密結合のため、ヘルパー化すると引数 6〜7 個 + ポインタ受け渡しになり、概念分離の利点より複雑化が勝る」設計判断を優先
- 選んだ案: 案B
- 理由: defer rollback は「Phase 4.2 の write 失敗 / Phase 4.3 の rename 失敗 / Phase 5 の DB error」3 系統からの巻き戻しを集約しており、状態を持つ closure として本体に残すのが自然。ヘルパーに切り出すと rollback closure の引数列に 4〜5 個の状態変数を引き渡す必要があり、概念分離どころか「状態をパラメータ化した可変コンテキスト」になり可読性が落ちる。Phase 0〜3（事前準備）のヘルパー化で 758 → 297 行に縮小しており、目的（読みやすい関数長への縮小）は達成済み

### 2026-05-07 - dirMoveMaps に movedFromTo / movedNodeIDs を混在させた

- 対象タスク: `internal/core/move_dir.go` の `MoveDir` 分解
- 論点: design レビューで「`dirMoveMaps` は地図のスナップショット型なのに `movedFromTo` / `movedNodeIDs` も同居している」と NIT 指摘
- 選択肢:
  - 案A: `dirMoveMaps` を `resolveMaps + 前後 pathSet スナップショット` に絞り、`movedFromTo` / `movedNodeIDs` は別構造体（`moveContext` 等）に分ける — 「概念分離」設計判断を優先
  - 案B: 現状のまま `dirMoveMaps` に同居 — 「現状で十分シンプル」「Phase 1 で同時に構築され Phase 2/3 で同時に参照される一束のコンテキスト」とみなす設計判断を優先
- 選んだ案: 案B
- 理由: `movedFromTo` / `movedNodeIDs` は確かに概念上「ムーブの入力」由来だが、resolveMaps 側の前後スナップショットと**同じタイミングで構築・参照される**ため Phase 2/3 ヘルパーが両方を必要とする。分離すると引数が `(dirMoveMaps, moveContext)` の 2 引数に増えるだけで実害がない。design レビュアーも NIT として「現状で十分シンプル」と明記しており、design-principles 1.2「シンプル > エレガント」を優先

### 2026-05-07 - MoveDir 分解はテスト追加なしで進めた

- 対象タスク: `internal/core/move_dir.go` の `MoveDir` 分解
- 論点: 大規模リファクタリング（758 → 297 行）でテストの網を増やすか
- 選択肢:
  - 案A: 各ヘルパー関数のユニットテストを追加（validateMoveDirOptions / loadMovesFromDB / classifyDiskState 等）— 「分解した個別ユニットを直接検証」設計判断を優先
  - 案B: 既存の MoveDir 統合テスト 388+ 件 + 前ループ追加のロールバックテスト 2 件のみで回帰検出 — 「リファクタはブラックボックスの挙動を変えないため、統合テストで検出できる範囲を信頼する」設計判断を優先
- 選んだ案: 案B
- 理由: ヘルパー化はリファクタリングであり、外部から見た MoveDir の挙動は不変。各ヘルパーは MoveDir からのみ呼ばれる private 関数で、独立したユニットテストを書く必然性が薄い。前ループでロールバックパスのテストを張った前提が backlog タスク説明にあり、その網と既存統合テストで「分解前後で同じ振る舞い」が検証できる。実際 vet/test は全てグリーン。3.1 YAGNI と 1.1 直近のコスト（ヘルパーごとの fixture 構築コストは大きいが将来の保守利益は小さい）を踏まえた判断

### 2026-05-06 23:36 - sentinel error 化のスコープを意味的識別ニーズのある範囲に絞った

- 対象タスク: `internal/core` の sentinel error 化
- 論点: `internal/core` には `fmt.Errorf` が 94 箇所ある。全部を機械的に `%w` ラップして sentinel 化するか、識別ニーズのある範囲に絞るか
- 選択肢:
  - 案A: 全 94 箇所を sentinel + `%w` 化 — 「将来の identification ニーズに先回り」設計判断を優先
  - 案B: 「呼び出し元または将来の呼び出し元が分岐したい意味的エラー」14 種に絞り、構文エラー・YAML パース・CLI バリデーション系は対象外 — 「YAGNI、識別ニーズが見えてから足す」設計判断を優先
- 選んだ案: 案B
- 理由: 識別ニーズのない sentinel を増やしても、可読性が下がるだけで利点がない。今回明確な識別ニーズがあるのは file/link 解決系・stale 検出・vault escape の 14 種で、構文エラー（`where.go` `convert.go` の `--ToFormat`）や YAML パースエラー（`config.go` `init_meta.go` 既に `%w` 済）は呼び出し元が `errors.Is` で分岐していないため除外した。`query_fetch.go:438` の `"stale index"` は `move/disambiguate` 系の `"... is stale"` と意味が異なるため別カテゴリとしてスコープ外。`delete.go:90` の `"path escapes vault"` と `move_helpers.go:96,153` の `"rewritten link would escape vault"` はメッセージが他の `link escapes vault` と異なるため、メッセージ完全保持制約の下で別 sentinel が必要になる。識別ニーズが出たら別タスクで追加する

### 2026-05-07 - frontmatter wikilink を build 時の vault-escape / ambiguous 検証対象に含めた

- 対象タスク: backlog v0.7.0「frontmatter 内 wikilink 対応 (1/2): 解析と edge 化」
- 論点: `build.go:87` の検証ループ（`if link.linkType != "wikilink" && link.linkType != "markdown" { continue }`）に `frontmatter_wikilink` を加えるか
- 選択肢:
  - 案A: 加える（本文 wikilink と同じ厳格さで vault-escape / ambiguous link を検出）— 「リンク種別ごとに検証スキップする差をなくす」設計判断を優先
  - 案B: 加えない（frontmatter wikilink は edge 化のみで、検証は本文と分離）— 「frontmatter は誤記しやすいから緩めに扱う」「(2/2) で書き換え対応するときに合わせて入れる」タスク管理を優先
- 選んだ案: 案A
- 理由: `build.go` の検証は「DB に登録する前に vault-escape / ambiguous をフェイルさせる」ものなので、リンク種別が違っても同じ厳格さで扱う方が自然。事前確認で既存 testdata 全 .md に frontmatter+`[[...]]` の組み合わせが無いことを Python スクリプトで確認したため、既存テスト 388+ ケースは全 pass。`add.go:249` / `update.go:136` の同型検証ガード（書き換え対象フィルタではなく「ambiguous 検出ガード」）には今回触らない。これらは新ノート追加 / 更新の経路で、frontmatter_wikilink を含むファイル投入時の検証は (2/2) の書き換えコマンド対応と合わせて整合をとる方がスコープ的にきれい

### 2026-05-07 - parseFrontmatterWikilinks のシグネチャから fileLineOffset を削除した

- 対象タスク: backlog v0.7.0「frontmatter 内 wikilink 対応 (1/2): 解析と edge 化」
- 論点: プランの設計判断 §3 では `parseFrontmatterWikilinks(rawLines []string, startIdx, endIdx, fileLineOffset int) []linkOccur` だったが、実装では `(rawLines []string, startIdx, endIdx int) []linkOccur` に縮小した
- 選択肢:
  - 案A: プランどおり `fileLineOffset` を保つ — 「将来 frontmatter が file 先頭以外から始まる拡張余地を残す」設計判断を優先
  - 案B: `fileLineOffset` を削除し `lineNum = j + 1` で計算 — 「YAGNI、現行仕様では frontmatter は常に file 先頭」設計判断を優先
- 選んだ案: 案B
- 理由: mdhop の現行仕様では frontmatter は常にファイル先頭（lines[0] = `---`）から始まる前提。`fileLineOffset` を保ったまま callers が常に 0 を渡す形は、将来必要になる前にパラメータを増やす YAGNI 違反。design レビュー (NIT) でも「YAGNI 改善として正当」と評価された。前提（`rawLines[0] = file line 1`）はコード上のコメントで明示

### 2026-05-07 - linkType 文字列リテラル直書きの増殖を今回スコープ外として残した

- 対象タスク: backlog v0.7.0「frontmatter 内 wikilink 対応 (1/2): 解析と edge 化」
- 論点: 本タスクで `linkType` の文字列リテラル直書き種類が 4 → 5 種類目になり、`build.go`、`resolve.go`、`parse.go`、`rewrite.go`、書き換え系コマンド群（add/move/update/disambiguate/simplify/repair/convert）の各所に分散している。design レビューで「分岐ミスリスクが高まる」SHOULD 指摘
- 選択肢:
  - 案A: 今回タスクで `type LinkType string` の named type 昇格まで実施 — 「コンパイル時の網羅チェック確保」設計判断を優先
  - 案B: 別タスクとして backlog Later に追加し、今回はスコープ外とする — 「タスク粒度を保つ」「NodeType 昇格と同型で別ループにする」タスク管理を優先
- 選んだ案: 案B
- 理由: NodeType 昇格 (v0.7.0 既完了) と同パターンで、シグネチャ変更を含む横断対応になる。今回の (1/2) スコープに混ぜると review 範囲が parse / build / resolve から書き換え系コマンド群まで膨らむ。backlog Later に「`linkType` 値を `type LinkType string` の named type に昇格して定数化」として追加し、独立タスクで対応する

### 2026-05-07 - frontmatter wikilink 書き換え対応で `pathLinkTypeSQLList` / `isPathLinkType` を導入

- 対象タスク: backlog v0.7.0「frontmatter 内 wikilink 対応 (2/2): 書き換えコマンド対応」
- 論点: 書き換え系の `link_type IN ('wikilink', 'markdown')` SQL リテラルが 6 箇所、Go 側 `linkType != "wikilink" && linkType != "markdown"` フィルタが 6 箇所散在しており、`frontmatter_wikilink` を加えると合計 12 箇所の重複拡張が必要
- 選択肢:
  - 案A: 12 箇所すべて手書きで `'wikilink', 'markdown', 'frontmatter_wikilink'` / 3 値 OR 比較に拡張 — 「定数化はコード volume を追加するため YAGNI」設計判断を優先
  - 案B: `pathLinkTypeSQLList` 定数 + `isPathLinkType` ヘルパーを `rewrite.go` に集約し、12 箇所を helper / fmt.Sprintf 経由に書き換え — 「同じ集合の 12 重複を SSoT 化、新 linkType 追加時の grep 漏れリスク低減」設計判断を優先
- 選んだ案: 案B（simplify レビュー時にエージェントが提案、採用）
- 理由: 12 重複は backlog Later「`linkType` 値を `type LinkType string` の named type に昇格して定数化」と同質の保守リスクで、今回タスクで触る箇所だけでも先行集約しておくと named type 昇格時の置き換え対象が SSoT 1 か所で済む。simplify 自動修正で既に 12 箇所が一括変換され、レビューでも DRY 改善として LGTM。なお `simplify.go` / `repair.go` の本文専用フィルタは「frontmatter wikilink を意図的に除外」する書き換え方針（convert と同様の判断）に合致するため、`isPathLinkType` には合流させずコメントで意図明示にとどめた

### 2026-05-07 - simplify/repair の `frontmatter_wikilink` 除外を手書きリテラル + コメントで残した

- 対象タスク: backlog v0.7.0「frontmatter 内 wikilink 対応 (2/2): 書き換えコマンド対応」
- 論点: `simplify.go:98` / `repair.go:83` の `lo.linkType != "wikilink" && lo.linkType != "markdown"` フィルタを `isPathLinkType` に統一すべきか。design レビューでは「`isPathLinkType` と乖離して保守リスク」と SHOULD 指摘
- 選択肢:
  - 案A: `isBodyPathLinkType` 等の新述語を切り出し、両ファイルを述語ベースに統一 — 「型変更時の取りこぼし防止」設計判断を優先
  - 案B: 手書きリテラルを残し、両ファイルに「frontmatter_wikilink を意図的に除外」コメントを追加。新述語化は別タスクへ — 「(2/2) スコープに含まれる「書き換え対応」と同質の本文専用処理ではない（簡略化/repair）ため、述語追加は保守改善として独立タスクが適切」タスク管理を優先
- 選んだ案: 案B
- 理由: simplify は「path → basename 簡略化」、repair は「broken/escape 修復」で、いずれも frontmatter wikilink には適用しない方針（convert と同じ）。convert.go では既存のリテラル比較が現状のまま意図一致しており、simplify/repair も同パターンで意図明示するのが整合的。`isBodyPathLinkType` ヘルパーを新設すると述語がもう 1 種類増え、用途が違う（書き換え系 vs 本文整理系）2 系統を同 helper で扱う設計責務になりやすい。design レビュアーも「実害リスクは低い」と評価。派生タスクとして backlog Later に「simplify/repair の手書き 2 型フィルタを述語化」を追加し、`linkType` named type 昇格と同タイミングで一括処理できるようにした

### 2026-05-07 - 既存 add_test.go ヘルパー検証の linkType フィルタは据え置き

- 対象タスク: backlog v0.7.0「frontmatter 内 wikilink 対応 (2/2): 書き換えコマンド対応」
- 論点: `add_test.go:672` / `add_test.go:935` の既存テスト（`TestAddAutoDisambiguateBasic` 等）が edge をループする際 `e.linkType == "wikilink" || e.linkType == "markdown"` でフィルタしており、`frontmatter_wikilink` が漏れている。facts レビュー SHOULD 指摘
- 選択肢:
  - 案A: 既存 2 箇所を `isPathLinkType(e.linkType)` に統一 — 「テスト間の一貫性」タスク管理を優先
  - 案B: 据え置き、別タスク化 — 「対象 vault `vault_add_disambiguate` に frontmatter_wikilink edge が無く現状実害ゼロ。既存テストヘルパーの修正はカバレッジ向上テーマで別タスク」タスク管理を優先
- 選んだ案: 案B
- 理由: 当該テストの vault には frontmatter wikilink を含むファイルが無いため、現状フィルタの 2 型でも検証は機能している。frontmatter_wikilink を含む edge をテスト対象に追加するならテスト fixture 自体の変更も必要で、(2/2) のスコープ「書き換えコマンドの linkType 拡張」とは別の「テストカバレッジ向上」軸。backlog Later に「`add_test.go` の既存ヘルパー検証で `frontmatter_wikilink` を含めるよう拡張」を追加して別タスク化

### 2026-05-07 - サンプルスキル更新の対象範囲を `examples/skills/mdhop/` のみに絞った

- 対象タスク: backlog v0.7.0「サンプルスキル更新」
- 論点: memory 記述では「`examples/skills/` 配下に `mdhop`, `mdhop-query`, `mdhop-workflow` の 3 サブディレクトリ」とあったが、実際は `mdhop/` 1 つしか存在しない。3 サブディレクトリ構成へ拡張するか、現状の単一ディレクトリ構成のまま更新するか
- 選択肢:
  - 案A: memory どおり `mdhop-query` `mdhop-workflow` を新規作成し、責務別 3 サブディレクトリに分ける — 「サンプル粒度を機能別に細分化」設計判断を優先
  - 案B: 現状の単一 `mdhop/` 構成のまま frontmatter wikilink 対応分のみ追記 — 「memory が古いと判断し現状を正とする」「リリース直前タスクでスコープを最小化」タスク管理を優先
- 選んだ案: 案B
- 理由: backlog タスクの目的は「`examples/skills/` 配下を最新仕様に合わせる」で、構造変更ではなく内容更新が要点。memory には「`examples/skills/mdhop/`, `examples/skills/mdhop-query/`, `examples/skills/mdhop-workflow/`」とあるが、現行リポジトリには `mdhop/` のみで、過去のリファクタで統合された可能性が高い（`SKILL.md` 自体に query/file-operation の両方が含まれる）。今回タスクで構造再分割を始めると、責務切り分け方針を新たに決める必要が出てスコープが膨らむ。memory の方を後で更新する形で整合させる

### 2026-05-07 - examples/skills/mdhop 更新は self-check で済ませた

- 対象タスク: backlog v0.7.0「サンプルスキル更新」
- 論点: ドキュメント変更のみだが対象がエージェント向け SKILL であり、説明の精度が低いと利用者の振る舞いに影響する。並列レビュー（review-code-all）に回すか self-check で済ますか
- 選択肢:
  - 案A: `/review-code-all` 系で並列レビュー — 「外部仕様ドキュメントの精度確保」設計判断を優先
  - 案B: self-check で済ます（内容は v0.7.0 で確定した frontmatter wikilink 仕様の反映のみ、検証ガード/書き換えコマンドのソース・ADR と整合確認済み） — 「ドキュメント修正は Small、`docs.md` ルール（ユーザー視点の挙動のみ・内部実装非掲載）に照らして自己確認可能」タスク管理を優先
- 選んだ案: 案B
- 理由: 追加内容は (1) frontmatter 内 wikilink が edge として拾われる (2) `add/update/move/disambiguate` は書き換え対象 (3) `convert/simplify/repair` は本文のみ、の 3 点に限定され、いずれも v0.7.0 で確定済みの実装挙動。decisions/0013 と internal/core/rewrite.go の `pathLinkTypeSQLList` で SSoT 化されており、ドキュメント記述と実装の整合は ADR 経由で再確認できる。go コードに変更がなくテスト実行不要

### 2026-05-07 - frontmatter 内 wikilink 対応を解析側 / 書き換え側に2分割

- 対象タスク: backlog v0.7.0「frontmatter 内 wikilink 対応」
- 論点: backlog エントリに「設計論点」が 5 つ（新 linkType / bare 検出 / 書き換え戦略 / meta 両立 / alias-subpath）並び、影響範囲も「parse + edge 化 + add/update/move/disambiguate/convert の 5 コマンド書き換え」と幅広い。1 ループ 1 コミットで全部入れるか、解析側と書き換え側で 2 タスクに分けるか
- 選択肢:
  - 案A: 1 タスクとして残し、1 ループで全部実装（parse 拡張 + 5 コマンド対応 + テスト全件）— 「機能としての完成形を 1 コミットでまとめて出す」タスク管理を優先
  - 案B: (1/2) 解析と edge 化 / (2/2) 5 コマンドの書き換え対応 に分割 — 「設計判断の重さを 2 分割し、レビュー粒度を保つ」タスク管理を優先
- 選んだ案: 案B
- 理由: 解析側（新 linkType / bare 検出 / meta 両立 / alias 流用）と書き換え側（行ベース置換 / quoted/bare 維持 / 5 コマンド × テスト）は決定軸が独立しており、合算すると 1 コミットの review 対象が parse.go + build.go + resolve.go + 5 コマンド + 多数のテストに膨らむ。前ループまでの分解粒度（NodeType 昇格 / sentinel error / init_meta 分割 / move_dir 分解）と揃え、1 ループあたり 200〜500 行スケールに収める。(1/2) のマージ後 (2/2) を実装する依存関係も自然で、(1/2) で edge が増えるだけなら既存の書き換え系は新 linkType を未対応として無視するため挙動破壊もない
