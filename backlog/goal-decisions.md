# Goal Decisions

## Format

### YYYY-MM-DD HH:mm - <判断タイトル>

- 対象タスク:
- 迷った点:
- 選んだ案:
- 選ばなかった案:
- 理由:
- ユーザー意図と違う可能性:
- 後で確認してほしい点:

### 2026-05-06 19:30 - ノード型定数化を untyped const に絞る（named type 化は別タスク）

- 対象タスク: v0.6.1「ノード型文字列リテラル定数化」
- 迷った点: design レビューで「`type NodeType string` の named type にすればコンパイラで型安全保証が完全に得られる」という SHOULD 指摘が出た
- 選んだ案: untyped string const のみ（既存シグネチャ変更なし）。`NodeTypeNote = "note"` 等を `db.go` に追加し、Go コードのリテラル参照を const に置換するだけ
- 選ばなかった案: `type NodeType string` の named type を導入し、`upsertNode` などの `typ string` 引数を `NodeType` に変更
- 理由: named type 化はテスト全件と DB スキャン結果のマッピング（`Type string` フィールド）に影響が及び、純粋な定数化の範囲を超える。1 コミットでスコープが膨らむのを避け、まず定数化を完了させる
- ユーザー意図と違う可能性: backlog の問題説明では「型安全性なく重複しており、タイポしてもコンパイル時に検出できない暗黙結合」と書かれており、named type 化まで意図していた可能性がある
- 後で確認してほしい点: untyped const 化だけで十分か、named type 化を別タスクとして v0.6.1 または後続バージョンに追加すべきか

### 2026-05-06 19:30 - SQL 内シングルクォートリテラルは対象外

- 対象タスク: v0.6.1「ノード型文字列リテラル定数化」
- 迷った点: `db.go:153 'phantom'`, `:184 'tag'`, `:330 'phantom'`, `:357 'tag','phantom','asset'` の SQL 内シングルクォートを const 化対象に含めるか
- 選んだ案: 対象外（SQL 構文の一部として残す）
- 選ばなかった案: `fmt.Sprintf` で SQL を動的生成し const を埋め込む
- 理由: SQL 文字列の動的生成は既存パターン（RAW SQL 文字列）を崩し、SQL injection 対策・可読性の観点でリスク。Intent を「Go コード側のタイポ検出」に絞ることで定数化の効果を保ちつつ範囲を限定
- ユーザー意図と違う可能性: backlog の「内部コア全体に 35 箇所以上散在」「SQL の WHERE 句と Go の switch 文で型安全性なく重複」という問題説明から、SQL 内も含めて統一したい意図だった可能性
- 後で確認してほしい点: SQL 内シングルクォートも const に統一する別タスクが必要か（`fmt.Sprintf` 化または `'note'` 等を bind 変数化）

### 2026-05-06 20:10 - query_entry.go の `sql.ErrNoRows == 比較` を今回は修正しない

- 対象タスク: v0.6.1「`internal/core/query.go` をエントリ解決とデータフェッチに分離」
- 迷った点: review-code go 観点で `query_entry.go:47` `:70` の `err == sql.ErrNoRows` を `errors.Is(err, sql.ErrNoRows)` に直すべきという 🔴 MUST 指摘が出た。今回タスクのスコープ内で対処すべきか
- 選んだ案: 対象外（今回タスクでは修正しない）
- 選ばなかった案: 当該 2 箇所だけ `errors.Is` に書き換え
- 理由: 今回タスクの Intent は「query.go の物理分割」で、ロジック変更を含めない。`sql.ErrNoRows == 比較` は元の `query.go` から無修正で持ち越したコードで、`internal/core` 全体に同種の `==` 比較が 13 箇所あり（goal-state memory 参照）、backlog v0.7.0「`internal/core` の sentinel error 化」で横断対応する範囲。ここだけ部分修正すると残り 11 箇所と一貫性が崩れる
- ユーザー意図と違う可能性: 「分割ついでに直してほしい」と意図していた可能性は低い（goal.md の Constraints「v0.6.1 の範囲外の unrelated refactor はやらない」と整合）
- 後で確認してほしい点: なし。v0.7.0 の sentinel error 化タスクで横断的に修正される
