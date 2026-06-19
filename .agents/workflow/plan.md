# Plan Workflow

## ICAR

- **Intent**: 実装前に、要求・制約・設計判断・検証方針を必要十分な粒度で揃える。
- **Constraints**:
  - 原則 1 plan = 1 workflow = 1 commit。独立した成果が混ざるなら plan を分ける。
  - backlog item や Goal が大きくても、そのまま 1 plan にしない。review / revert / bisect できる 1 commit 単位へ切る。
  - 1 commit 単位は、途中段階でも「その単位として完了している」状態にする。Goal 全体の完了とは別に判断する。
  - 仕様・UX・設計方針に複数の妥当な選択肢が実際にある場合はユーザー確認に回す。
  - 設計判断は `agents/workflow/design-decision-record.md` に従い、採用案・却下案・理由を残す。
  - 検証方針（自動 / ユーザー確認）を plan に明記する。
- **Acceptance**:
  - 実装対象、非対象、検証方針が明確。
  - 必要な `docs/specs/`, `backlog/backlog.md`, `docs/decisions/` の更新方針、および知見をソースコメント / `llm-wiki/` のどこへ残すかが明確。
  - レビュー指摘への対応が済んでいる、または対応しない理由が plan に書かれている。
  - 未解決の不明点がない。ある場合はユーザー確認待ちとして止まっている。
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

Small（`default.md` の Intake 分類）— typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正 — は plan を省略してよい。

## Flow ICAR

### UX Scenario

- **Intent**: UI 変更の Before / After / 操作手順を、具体的な 1 状態で確認できるようにする。
- **Constraints**: ロジックのみの変更なら「N/A — UI 変更なし」と明記してスキップする。
- **Acceptance**: ユーザー確認が必要な UI / 挙動が plan 上で明確になっている。
- **Relevant**: `docs/specs/`（該当する UX / シナリオ仕様）、対象 View / 画面。

### Design

- **Intent**: モジュール配置・共通化方針・型選択を、既存設計と長期保守性に沿って決める。
- **Constraints**:
  - `design-decision` を使い、ルールに当てはめても決まらないときだけユーザー確認する。
  - 新しい型・ファイル・外部依存・責務配置・module/package/target/folder 境界を扱う場合は `module-boundary` を使い、分割レベルと分割しない理由を明確にする。
  - 設計判断の残し方は `agents/workflow/design-decision-record.md` に従う。
  - モジュール配置は依存方向と既存責務で判断する。
  - 共通化は「片方だけ変更したくなったとき、もう片方に影響なく変更できるか？」で判断する。
  - プロジェクト固有制約に触れるなら `project-risk-check` で確認する。観点は skill 側が持つ。
- **Acceptance**: 採用案・却下案・理由・残リスクが plan に残っている。
- **Relevant**: `docs/rules/`（アーキテクチャ・制約）, `llm-wiki/`（作業地図）, 関連コード。

### Refactor Scope

- **Intent**: 理想状態は全体が綺麗であること。ただし 1 plan = 1 commit の粒度では、毎回全体を見直さず、今回の変更範囲で必要な構造改善を判断する。
- **Constraints**:
  - 今の構造を維持すること自体を目的にしない。
  - 調査範囲は、変更対象・直接の呼び出し元/呼び出し先・関連 specs / rules / `llm-wiki/`（作業地図）に絞る。
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
    <!-- slot: project-risk-check 以外の領域固有レビュー skill があれば追記する（例: SwiftUI を触るなら swiftui-pro を使う）。 -->
    <!-- /slot -->
  - High-risk / 設計判断が重い / 曖昧 / 実装後では手戻りが大きい場合だけ、`change-review` などの別視点を plan レビューにも入れる。
  - レビュー周回は最大 3 周。3 周で収束しなければそれ以上回さず打ち切り、残った指摘と周回数を記録して完了報告（Goal なら Goal 完了報告）の `レビュー上限超過` で通知する。
- **Acceptance**: 指摘が plan に反映済み、または対応しない理由が事実と理由で残っている。
- **Relevant**: plan、関連 specs / rules、レビュー観点 skill。

## Stop Conditions

- 1 commit に収まらない。
- 今回の plan が Goal / backlog item 全体をまとめようとしており、自然な commit 単位へ切れていない。
- High-risk なのに検証方針がない。
- 仕様・UX・設計方針をユーザー判断なしに決める必要がある。
