# Plan Workflow

## ICAR

- **Intent**: 実装前に、要求・制約・設計判断・検証方針を必要十分な粒度で揃える。
- **Constraints**:
  - 原則 1 plan = 1 workflow = 1 commit。独立した成果が混ざるなら plan を分ける。
  - backlog item や Goal が大きくても、そのまま 1 plan にしない。review / revert / bisect できる 1 commit 単位へ切る。
  - 1 commit 単位は、途中段階でも「その単位として完了している」状態にする。Goal 全体の完了とは別に判断する。
  - 仕様・UX・設計方針の複数案は `change/workflow.md` の判断境界に従う。可逆で影響が小さい選択は採用案で進め、複数の妥当案が残って非可逆またはやり直しコストが大きい、または正本と矛盾する場合は Stop Conditions に従う。
  - 設計判断は `.agents/workflow/design-decision-record.md` に従い、採用案・却下案・理由を残す。
  - product decision（UX・データ意味・cross-surface 等）を変える plan では Product Decision Ledger の Alternative Check を行う。カテゴリ一覧と記録・報告基準の正本は `.agents/workflow/design-decision-record.md`。
  - 現在の要求 / backlog / docs / decisions に明記済みの内容や、判断系 skill で実装判断として解ける内容は、Goal 完了報告の `ユーザー判断が必要` に混ぜない。
  - 検証方針（自動 / ユーザー確認）を plan に明記する。
- **Acceptance**:
  - 実装対象、非対象、検証方針が明確。
  - 必要な `docs/specs/`, `backlog/backlog.md`, `docs/decisions/` の更新方針、および知見をソースコメント / `llm-wiki/` のどこへ残すかが明確。
  - レビュー指摘への対応が済んでいる、または対応しない理由が plan に書かれている。
  - 実装に進めるだけの判断材料が揃っている。重要なユーザー判断候補が残る場合は、Product Decision Ledger から採用案、別案、報告が必要な理由を説明できる。
- **Relevant**:
  - ユーザー依頼
  - `backlog/backlog.md`
  - 関連する `docs/rules/`, `docs/specs/`, `docs/decisions/`, `llm-wiki/`（作業地図）
  - 関連コードと既存パターン

## Use When

- 複数ファイル変更
- 仕様・UX・データモデル・アーキテクチャに影響する変更
- High-risk 変更
- 実装方針が複数あり判断が必要
- リファクタを含む

Small（`change/workflow.md` の Intake 分類）— typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正 — は plan を省略してよい。

## Plan Artifact

Goal 経由で plan を作る Change では、Implementer は実装を始める前に、実装前判断と plan を `tmp/implementer-plan-<change>.md` へ保存する。`<change>` は Conductor が Change brief とともに渡す、その Goal 内で一意な識別子とする。

- 内容は変更意図、対象・非対象、触るファイル、設計判断、検証方針、review 深度と追加観点とする。
- 実装後の結果に合わせて書き直さず、「実装前の意図」の記録として保持する。実装中に生じた逸脱・追加判断は Implementer の handoff で別に明示する。
- Implementer は handoff で artifact の正確な path を Conductor に返す。Conductor は Gatekeeper 起動時に Change brief と同じ不変入力としてその path を渡し、Gatekeeper は brief / plan 照合に使う。
- Gatekeeper は review lane を起動する際、finder が要求・設計意図・検証方針との照合に必要な場合に同じ artifact path を渡す。Goal Review には渡さない。
- plan を省略した Small では artifact も不要とし、Gatekeeper へ切り替えた場合は Change brief と実差分・検証結果を照合元にする。

## Flow ICAR

### UX Scenario

- **Intent**: UI 変更の Before / After / 操作手順を、具体的な 1 状態で確認できるようにする。
- **Constraints**: ロジックのみの変更なら「N/A — UI 変更なし」と明記してスキップする。
- **Acceptance**: UI / 挙動の確認方法と、Product Decision Ledger へ残すべきステークホルダー判断候補の有無が plan 上で明確になっている。
- **Relevant**: `docs/specs/`（該当する UX / シナリオ仕様）、対象 View / 画面。

### Design

- **Intent**: モジュール配置・共通化方針・型選択を、既存設計と長期保守性に沿って決める。
- **Constraints**:
  - `design-decision` を使い、判断境界は `change/workflow.md` に従う。
  - 新しい型・ファイル・外部依存・責務配置・module/package/target/folder 境界を扱う場合は `module-boundary` を使い、分割レベルと分割しない理由を明確にする。
  - 設計判断の残し方は `.agents/workflow/design-decision-record.md` に従う。
  - モジュール配置は依存方向と既存責務で判断する。
  - 共通化は「片方だけ変更したくなったとき、もう片方に影響なく変更できるか？」で判断する。
  - プロジェクト固有制約に触れるなら `project-risk-check` で確認する。観点は skill 側が持つ。
- **Acceptance**: 採用案・却下案・理由・残リスクが plan に残っている。実装寄りの設計判断と、ステークホルダーに報告すべき product decision が混ざっていない。
- **Relevant**: `docs/rules/`（アーキテクチャ・制約）, `llm-wiki/`（作業地図）, 関連コード。

### Refactor Scope

- **Intent**: 理想状態は全体が綺麗であること。ただし 1 plan = 1 commit の粒度では、毎回全体を見直さず、今回の変更範囲で必要な構造改善を判断する。
- **Constraints**:
  - 今の構造を維持すること自体を目的にしない。
  - 調査範囲は、変更対象・直接の呼び出し元/呼び出し先・関連 `docs/specs/` / `docs/rules/` / `llm-wiki/`（作業地図）に絞る。
  - その範囲で実装が歪む、重複が増える、責務境界が曖昧になるなら、先に局所リファクタするか今回の plan に含める。
  - 1 commit に収まらない広い構造改善は、今回に混ぜず `backlog/backlog.md` または `maintenance.md` の対象に切り出す。
  - `backlog/backlog.md` の直近バージョンに計画済みのリファクタ指摘は既知として扱う。
- **Acceptance**: そのまま実装 / 先に局所リファクタ / 今回に含める / 別 task に切る、の判断が plan にある。
- **Relevant**: 変更対象コード、直接の依存先/依存元、`backlog/backlog.md`, `maintenance.md`。

### Plan Review

- **Intent**: 実装前に plan の事実誤認・設計劣化・検証不足を見つける。
- **Constraints**:
  - 通常は実装後 review を標準とし、plan review は self-check でよい。
  - 実装差分レビューでは Small 以外を原則 `change-review` に通すため、plan 時点でもレビュー深度と追加 skill の要否を明記する。
  - 設計判断には `design-decision` を使う。
  - プロジェクト固有制約に触れるなら `project-risk-check` を使う。
    <!-- slot: project-risk-check 以外の領域固有レビュー skill があれば追記する（例: UI 層を触るなら対応する specialist skill）。 -->
    <!-- /slot -->
  - High-risk / 設計判断が重い / 曖昧 / 実装後では手戻りが大きい場合だけ、`change-review` などの別視点を plan レビューにも入れる。別系統エージェントへのクロスレビューは plan 段階では行わず、Goal Review 側に置く。
  - plan review 後に再レビューするかは、指摘対応で plan の構造・risk・検証方針・設計判断が大きく変わったかで判断する。機械的な反映だけなら再レビューせず実装へ進んでよい。
  - plan review では `レビュー上限超過` を使わない。残る懸念は plan の残リスク、追加検証、または実装後 review で見る観点として残す。
- **Acceptance**: 指摘が plan に反映済み、または対応しない理由が事実と理由で残っている。
- **Relevant**: plan、関連 `docs/specs/` / `docs/rules/`、レビュー観点 skill。

## Stop Conditions

- 1 commit に収まらない。
- 今回の plan が Goal / backlog item 全体をまとめようとしており、自然な commit 単位へ切れていない。
- High-risk なのに必須の検証方針を代替手段も含めて立てられない。
- `change/workflow.md` の判断境界で Stop に該当する仕様・UX・設計方針が残っている。
