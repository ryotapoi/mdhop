# Plan

## Intent

実装前に、要求・制約・設計判断・検証方針を必要十分な粒度で揃える。

## Plan Mode

plan mode（`EnterPlanMode` / `ExitPlanMode`）は使わない。承認待ちが `/goal` の自動進行と噛み合わないため。計画は内部で立て、そのまま `change/implement.md` へ進む。ユーザー確認が必要なのは Stop Conditions に該当する場合だけ。

## Use When

- 複数ファイル変更
- 仕様・UX・データモデル・アーキテクチャに影響する変更
- High-risk 変更
- 実装方針が複数あり判断が必要
- リファクタを含む

Small（`change/workflow.md` の Intake 分類）— typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正 — は plan を省略してよい。

## Solo Plan File

execution mode `solo`（Implementer が計画と実装を一体で行う）では、plan を作る Change は実装前に plan を `tmp/solo-plan-<change>.md` へ書き出す。

- 内容は変更意図・触るファイル・設計判断・検証方針。仕様の記述が薄いタスクほど、このファイルが実装意図の記録の主役になる。
- 実装後に書き直さない。「実装前の意図」の記録として、review lane への文脈提供（`change/review.md` の Review Lane Delegation）と後続 Change の参照に使うため。Goal Review には渡さない（Goal Review は commit range だけを対象にする）。
- Gatekeeper は実装文脈を引き継がない fresh subagent のため、diff だけでは意図が読み取りにくい変更では、このファイルが Gatekeeper への文脈提供にもなる（review lane への文脈提供と同じ経路）。
- plan を省略した Small では書き出しも不要（review 側も L0 self-check のみ）。
- execution mode `delegate` では plan file がなく、代わりに Implementer subagent が実装エージェントへ要求した実装前判断（全サイト列挙・責務配置・テスト方針）を `tmp/delegate-plan-<change>.md` に一時 artifact として保存し、Gatekeeper の照合元にする（`change/delegate.md` の委譲後節参照）。

## Inputs

- ユーザー依頼
- `backlog/backlog.md`
- 関連する `docs/rules/`, `docs/specs/`, `docs/decisions/`, `llm-wiki/`（作業地図）
- 関連コードと既存パターン

## UX シナリオ

UI / 出力に関わる変更なら、Before / After / 操作手順を 1 つの具体的な状態で plan に書く。
ロジックのみの変更なら「N/A — UI 変更なし」と明記してスキップ。

ユーザーへの確認は plan の必須ステップではない。仕様・UX・設計方針の複数案は `change/workflow.md` の判断境界に従う。可逆で影響が小さい選択は採用案で進め、複数の妥当案が残って非可逆またはやり直しコストが大きい、または正本と矛盾する場合は Stop Conditions に従う。見た目・操作の確認は実装後に `change/verify.md` の方針で自動検証を優先し、確定できない場合だけ Stop Condition または残存リスクとして扱う。

product decision（UX・データ意味・cross-surface 等）を変える plan では、Product Decision Ledger の Alternative Check を行う。カテゴリ一覧と記録・報告基準の正本は `.claude/workflow/design-decision-record.md`。
現在の要求 / backlog / docs / decisions に明記済みの内容や、判断系 skill で実装判断として解ける内容は、Goal 完了報告の `ユーザー判断が必要` に混ぜない。

## 設計判断

- 設計判断の前に `design-decision` スキルを呼ぶ。判断境界は `change/workflow.md` に従う
- モジュール配置・共通化方針・型選択を判断する
- プロジェクト固有制約に触れるなら `project-risk-check` で確認する。観点は skill 側が持つ。
- 採用案・却下案・理由・残リスクを plan に残す。実装寄りの設計判断と、ステークホルダーに報告すべき product decision を混ぜない。

## 先行リファクタ判定

変更対象に明らかな構造の悪さがある場合のみ、機能追加の前に直すべきか判断する。判断は `design-decision` / `module-boundary` を使い、先行必須か別件か、今回に混ぜるか別 plan に切るかで分ける。
小さい修正・ロジック追加だけの変更では判定しない。

`backlog/backlog.md` の直近バージョンに計画済みのリファクタ指摘は既知として無視してよい。

## Decision Criteria

- 原則 1 plan = 1 commit。独立した成果が混ざるなら plan を分ける
- backlog item や Goal が大きくても、そのまま 1 plan にしない。review / revert / bisect できる 1 commit 単位へ切る
- 1 commit 単位は、途中段階でも「その単位として完了している」状態にする。Goal 全体の完了とは別に判断する
- 設計判断は採用案・却下案・理由を plan に記録
- 検証方針（自動 / ユーザー確認）を plan に明記
- **モジュール配置**: 「依存の方向に違反しないか」「既存モジュールの責務を逸脱しないか」で判断する。新モジュールを切るならその理由を書く
- **共通化と分離**: 「片方だけ変更したくなったとき、もう片方に影響なく変更できるか？」で判断する。無理に共通化して分岐だらけになるなら分ける

## Plan Review

- 通常は実装後レビュー（`change/review.md`）を標準とし、plan review は self-check でよい。
- 実装差分レビューでは Small 以外を原則 `/code-review xhigh` 観点ベースのレビューに通すため、plan 時点でもレビュー深度と追加 skill の要否を明記する。
- 領域固有リスクがあれば該当観点の skill を plan に当てる。`project-risk-check` 以外で固有制約に触れる場合は次の slot のマッピングに従う。
  <!-- slot: project-risk-check 以外の領域固有レビュー skill があれば追記する（例: UI 層を触るなら対応する specialist skill）。 -->
  <!-- /slot -->
- High-risk / 設計判断が重い / 曖昧 / 実装後では手戻りが大きい場合だけ、`claude-fresh-review` でプランファイルを実装文脈を引き継がない fresh reviewer に回す。別系統エージェントへのクロスレビューは plan 段階では行わず、Goal Review 側に置く。
- plan review 後に再レビューするかは、指摘対応で plan の構造・risk・検証方針・設計判断が大きく変わったかで判断する。機械的な反映だけなら再レビューせず実装へ進んでよい。
- plan review では `レビュー上限超過` を使わない。残る懸念は plan の残リスク、追加検証、または実装後 review で見る観点として残す。

## Acceptance

- 実装対象、非対象、検証方針が明確
- 必要な仕様・backlog・decision の更新方針が明確
- レビュー指摘への対応が済んでいる、または対応しない理由が plan に書かれている
- レビュー指摘に対応しない場合は、plan に**考慮したこと**（不要と判断した理由・別タスクに切り出す理由・トレードオフ）を事実と理由で書く（「対処済み」だけの完了宣言は不可）
- 実装に進めるだけの判断材料が揃っている。重要なユーザー判断候補が残る場合は、Product Decision Ledger から採用案、別案、報告が必要な理由を説明できる。

## Stop Conditions

- `change/workflow.md` の判断境界で Stop に該当する仕様・UX・設計方針が残っている
- 1 commit に収まらない（plan を分ける）
- High-risk なのに必須の検証方針を代替手段も含めて立てられない
