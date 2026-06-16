# Plan Workflow

## ICAR

- **Intent**: 実装前に、要求・制約・設計判断・検証方針を必要十分な粒度で揃える。
- **Constraints**:
  - 原則 1 plan = 1 commit。独立した成果が混ざるなら plan を分ける。
  - 仕様・CLI 挙動・設計方針に複数の妥当な選択肢がある場合はユーザー確認に回す。
  - 設計判断は採用案・却下案・理由を残す。
  - 検証方針（自動 / CLI 動作確認 / ユーザー確認）を plan に明記する。
- **Acceptance**:
  - 実装対象、非対象、検証方針が明確。
  - 必要な `docs/rules/`, `backlog/backlog.md`, `docs/decisions/`, `llm-wiki/knowledge.md`, `docs/specs/` の更新方針が明確。
  - レビュー指摘への対応が済んでいる、または対応しない理由が plan に書かれている。
  - 未解決の不明点がない。ある場合はユーザー確認待ちとして止まっている。
- **Relevant**:
  - ユーザー依頼
  - `backlog/backlog.md`
  - 関連する `docs/rules/`, `docs/decisions/`, `llm-wiki/knowledge.md`, `docs/specs/`（あれば）
  - 関連コードと既存パターン

## Use When

- 複数ファイル変更
- 仕様・データモデル・アーキテクチャに影響する変更
- High-risk 変更
- 実装方針が複数あり判断が必要
- リファクタを含む

Small（`default.md` の Intake 分類）は plan を省略してよい。

## Flow ICAR

### CLI Scenario

- **Intent**: CLI 変更の Before / After / 操作手順を、具体的な 1 状態で確認できるようにする。
- **Constraints**: 内部ロジックのみの変更なら「N/A — CLI 出力変更なし」と明記してスキップする。
- **Acceptance**: ユーザー確認が必要な CLI 挙動が plan 上で明確になっている。
- **Relevant**: `docs/specs/overview.md`, 対象 command, 関連テスト。

### Design

- **Intent**: モジュール配置・共通化方針・型選択を、既存設計と長期保守性に沿って決める。
- **Constraints**:
  - `design-decision` を使い、ルールに当てはめても決まらないときだけユーザー確認する。
  - mdhop 固有制約に触れるなら `mdhop-risk-check` で確認する。
  - モジュール配置は `docs/rules/architecture.md` の依存方向と既存責務で判断する。
  - 共通化は「片方だけ変更したくなったとき、もう片方に影響なく変更できるか？」で判断する。
- **Acceptance**: 採用案・却下案・理由・残リスクが plan に残っている。
- **Relevant**: `docs/rules/architecture.md`, `docs/rules/03-data-model.md`, `docs/specs/overview.md`, `llm-wiki/knowledge.md`, 関連コード。

### Refactor Scope

- **Intent**: 今回の変更範囲で必要な構造改善を判断する。
- **Constraints**:
  - 毎回全体を見直さず、変更対象・直接の依存先/依存元・関連 rules / knowledge に絞る。
  - その範囲で実装が歪む、重複が増える、責務境界が曖昧になるなら、先に局所リファクタするか今回の plan に含める。
  - 1 commit に収まらない広い構造改善は `backlog/backlog.md` または `maintenance.md` の対象に切り出す。
- **Acceptance**: そのまま実装 / 先に局所リファクタ / 今回に含める / 別 task に切る、の判断が plan にある。
- **Relevant**: 変更対象コード、直接の依存先/依存元、`backlog/backlog.md`, `maintenance.md`。

### Plan Review

- **Intent**: 実装前に plan の事実誤認・設計劣化・検証不足を見つける。
- **Constraints**:
  - 通常は実装後レビュー（`review.md`）を標準とし、plan review は self-check でよい。
  - 領域固有リスクがあれば `mdhop-risk-check` を plan に当てる。
  - High-risk / 設計判断が重い / 曖昧 / 実装後では手戻りが大きい場合だけ、`change-review` などの別視点を入れる。
- **Acceptance**: 指摘が plan に反映済み、または対応しない理由が事実と理由で残っている。
- **Relevant**: plan、関連 rules、レビュー観点 skill。

## Stop Conditions

- 1 commit に収まらない。
- High-risk なのに検証方針がない。
- 仕様・CLI 挙動・設計方針をユーザー判断なしに決める必要がある。
