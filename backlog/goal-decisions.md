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

### 2026-05-07 - YAML コメント除去スコープ（block scalar / 複数行 quoted）

- 対象タスク: frontmatter wikilink の生 raw scan が YAML コメントを edge 化する不具合
- 論点: `stripYAMLComment` をどこまで YAML 仕様に忠実にするか。block scalar (`|`, `>`) や複数行にまたがる quoted scalar 中の `#` も「コメントではない」と判定する必要があるか
- 選択肢:
  - 案A: 1 行内の single-quoted / double-quoted のみ追跡し、block scalar・複数行 quoted は対象外。タスク説明の「quoted/bare の判定」最小実装に絞る — 「YAGNI、現実報告された不具合とテストでカバー可能な範囲のみ修正」局所修正を優先
  - 案B: yaml.Node の Style 情報を再走査して block scalar / 複数行 quoted の行範囲を計算し、その範囲では `#` をコメントとして扱わない — 「YAML 仕様に忠実、将来の追加 frontmatter スタイルでも壊れない」整合性を優先
- 選んだ案: 案A
- 理由: タスク説明には「quoted/bare の判定ロジックを共有 or 抽出する必要があり」と書かれており、block scalar への言及がない。frontmatter で wikilink を block scalar に書くケースは現実的にほぼ無く（バグ報告も bare 行の `#` コメント）、案B は再走査ロジック追加で複雑性が増す。仕様に書かれていない範囲の改善は YAGNI として後回し。実害が出たら同 helper を `yaml.Node.Style` 参照に拡張する形で対応可能（Later タスク化はせず、必要時にバックログ追加で十分）

### 2026-05-07 - add_test 既存ヘルパー検証の fixture 拡張方法

- 対象タスク: `add_test.go` の既存ヘルパー検証で `frontmatter_wikilink` を含めるよう拡張
- 論点: `frontmatter_wikilink` edge を持つ状態で `TestAddAutoDisambiguateDBUpdated` / `TestAddAutoDisambiguateRebuildConsistent` を流すために、既存 `vault_add_disambiguate/A.md` に frontmatter を追記するか、別 vault を新設するか
- 選択肢:
  - 案A: `vault_add_disambiguate/A.md` に frontmatter wikilink を追記し、既存 12 箇所のテスト（特に `TestAddAutoDisambiguateBasic` の行リテラル検証 + `Rewritten = 5` 期待値、`TestAddAutoDisambiguateExtensionPreserved` 等）の期待値を連動更新 — 「1 vault で auto-disambiguate の挙動を網羅する」整合性を優先
  - 案B: 別 vault `vault_add_disambiguate_frontmatter` を新設し、新規テスト 2 本（DBUpdated / RebuildConsistent 相当）でフィルタ統一の検証強化を担う。既存テストはフィルタ修正のみ（`isPathLinkType` 統一）に留める — 「フィクスチャの責務をテーマ別に分割し、既存テスト群への副作用を避ける」局所修正を優先
- 選んだ案: 案B
- 理由: `vault_add_disambiguate/A.md` を変更すると `TestAddAutoDisambiguateBasic` が行番号ベースで A.md 内容を直接アサートしている (`lines[0]..lines[4]`)、`Rewritten = 5` を hard-code、`TestAddAutoDisambiguateExtensionPreserved`/`InlineCodeIgnored`/`Embed` 等が独自に A.md を上書きしているなど、影響範囲が大きい。前ループの simplify でも同パターン（別 vault `vault_simplify_frontmatter` 新設）を採用しており判断の一貫性が取れる。frontmatter_wikilink テーマだけを切り出した小さい vault の方が、後続の修正でも fixture 変更が局所化できる

### 2026-05-07 - 前ループ「YAML コメント除去スコープ」判断の修正（ADR 0013 違反リグレッション）

- 対象タスク: frontmatter wikilink の block scalar 内 `# [[B]]` が edge 化されないリグレッションの修正
- 論点: 直前ループ（goal-decisions「YAML コメント除去スコープ（block scalar / 複数行 quoted）」案A）で「block scalar は対象外、bare 行の `#` のみ追跡」を選んだが、レビューで ADR 0013 (`decisions/0013-frontmatter-wikilink-detection.md:43`) が「block scalar (`key: |`) も行範囲走査で自動的にカバーされる」を肯定的 Consequence として明記していると指摘された。前ループ判断は ADR を読まずに行われた誤判断
- 選択肢:
  - 案A: 今ループ内で修正＋テスト追加。`yaml.Node.Style` で block scalar 行範囲を判定し `stripYAMLComment` 適用を除外。`decisions/0013` の保証を尊重 — 「ADR と実装の整合性を即時回復、リグレッションを 1 ループで閉じる」整合性を優先
  - 案B: backlog に積んで次ループで対応 — 「1 セッション 1 ループ運用を厳守」タスク管理を優先
  - 案C: ADR 0013 側を「block scalar 非対応」に改訂 — 「実装の現状に合わせて ADR を縮小」局所修正を優先
- 選んだ案: 案A
- 理由: ADR の Consequences として明文化された保証を破ったままにすると、ADR を信じて読むエージェント / 人間の意思決定が壊れる。`yaml.Node.Style == LiteralStyle/FoldedStyle` で静的に判定できるため追加複雑度は限定的（追加コード 25 行）。`design-principles` 1.3「より上位の仕組みで防ぐ」（ADR は仕組みの一種）と 1.1「直近の作業量を軸にしない」に当てはめて案A。案C は ADR の Considered Options で B（生 raw scan）を選んだ動機（block scalar カバー）自体を否定することになり、選んだ理由が消える。マスター承認済み

### 2026-05-07 - LinkType 昇格時のテスト側リテラル定数化スコープ

- 対象タスク: `linkType` 値を `type LinkType string` の named type に昇格して定数化
- 論点: テストファイル内に残る `linkType` リテラルのうち、どこまでを `LinkType` 定数参照に置換するか
- 選択肢:
  - 案A: テストの `==` / `!=` 直接比較のみ定数化し、`filterByType(links, "wikilink")` のような関数引数や `t.Errorf("... want wikilink", ...)` の出力テキストはそのまま — 「型昇格の意図はテストの比較に行き渡らせるが、untyped const → LinkType の暗黙変換が成立する箇所は文字列リテラルでも害がない」局所修正を優先
  - 案B: テスト内に現れる `wikilink`/`markdown`/`tag`/`frontmatter`/`frontmatter_wikilink` の文字列リテラルすべてを定数に置換 — 「テストでも意図を 100% 徹底する」整合性を優先
- 選んだ案: 案A
- 理由: NodeType 昇格時に Later タスクとして残った「テスト側の untyped string リテラル比較の定数化」（backlog/backlog.md Later 節）と同じ構造だが、今回は比較箇所までは即時対応した。`filterByType` 引数は untyped const として呼出時に `LinkType` へ自動変換されるため、定数化しなくても型安全性は失われない。`t.Errorf` のメッセージ文字列は人間向け出力で型ではないため対象外。残った文字列リテラルでも将来の追加（例えば 6 種類目の linkType）でテストが壊れる経路は無い

