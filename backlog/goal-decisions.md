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

### 2026-05-07 - simplify frontmatter wikilink テスト用 vault の場所

- 対象タスク: `simplify` を frontmatter wikilink 対応に拡張
- 論点: frontmatter wikilink を含む simplify テストの fixture を、既存 `testdata/vault_simplify` に追加するか別 vault として新設するか
- 選択肢:
  - 案A: 既存 `vault_simplify` に frontmatter wikilink 入りファイルを追記 — 「1 vault で simplify の挙動を網羅する」整合性を優先
  - 案B: 別 vault `vault_simplify_frontmatter` を新設 — 「フィクスチャの責務をテーマ別に分割し、既存テスト（A.md / sub/B.md など）への副作用を避ける」局所修正を優先
- 選んだ案: 案B
- 理由: `vault_simplify` 既存ファイルに frontmatter を後付けすると、TestSimplifyDryRun / TestSimplifyInlineCodeIgnored など既存ファイルの内容を直接検証するテストの期待値が連動して変わるリスクがある。frontmatter wikilink テーマだけを切り出した小さい vault の方が、後続ループの修正（YAML コメント除外バグ等）でも fixture 変更が局所化できる

### 2026-05-07 - LinkType 昇格時のテスト側リテラル定数化スコープ

- 対象タスク: `linkType` 値を `type LinkType string` の named type に昇格して定数化
- 論点: テストファイル内に残る `linkType` リテラルのうち、どこまでを `LinkType` 定数参照に置換するか
- 選択肢:
  - 案A: テストの `==` / `!=` 直接比較のみ定数化し、`filterByType(links, "wikilink")` のような関数引数や `t.Errorf("... want wikilink", ...)` の出力テキストはそのまま — 「型昇格の意図はテストの比較に行き渡らせるが、untyped const → LinkType の暗黙変換が成立する箇所は文字列リテラルでも害がない」局所修正を優先
  - 案B: テスト内に現れる `wikilink`/`markdown`/`tag`/`frontmatter`/`frontmatter_wikilink` の文字列リテラルすべてを定数に置換 — 「テストでも意図を 100% 徹底する」整合性を優先
- 選んだ案: 案A
- 理由: NodeType 昇格時に Later タスクとして残った「テスト側の untyped string リテラル比較の定数化」（backlog/backlog.md Later 節）と同じ構造だが、今回は比較箇所までは即時対応した。`filterByType` 引数は untyped const として呼出時に `LinkType` へ自動変換されるため、定数化しなくても型安全性は失われない。`t.Errorf` のメッセージ文字列は人間向け出力で型ではないため対象外。残った文字列リテラルでも将来の追加（例えば 6 種類目の linkType）でテストが壊れる経路は無い

