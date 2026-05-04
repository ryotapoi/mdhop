# Plan

## Intent

実装前に、要求・制約・設計判断・検証方針を必要十分な粒度で揃える。

## Use When

- 複数ファイル変更
- 仕様・データモデル・アーキテクチャに影響する変更
- High-risk 変更（`default.md` の Intake 分類）
- 実装方針が複数あり判断が必要
- リファクタを含む

Small（typo、docs、テスト追加だけ、1 ファイルの明確なバグ修正）は plan を省略してよい。

## Inputs

- ユーザー依頼
- `backlog/backlog.md`
- 関連する `rules/`, `specs/`（あれば）, `decisions/`, `references/knowledge.md`
- 関連コードと既存パターン

## UX シナリオ

CLI 出力に関わる変更なら、Before / After / 操作手順を 1 つの具体的な状態で書き、ユーザーに確認する。
内部ロジックのみの変更なら「N/A — CLI 出力変更なし」と明記してスキップ。

```markdown
## UX Scenario

### Before
- 具体的な CLI 入出力の状態を記述

### After
- 同じ操作がどう変わるべきかを記述

### 操作手順
1. ユーザーがどのコマンドを実行したら
2. 出力がどう変わるか
```

ポイント:
- 抽象的な仕様ではなく、具体的な1つの状態で書く
- 操作の前後で出力がどう見えるかを明示する
- 複数のシナリオがある場合は主要なものを2-3個

UX シナリオを書いたら、実装方針に入る前にユーザーに確認する。

## 設計判断

- 設計判断の前に `design-principles` スキルを呼ぶ
- ルールに当てはめても決まらないときだけユーザー確認
- モジュール配置（`cmd/mdhop` と `internal/core` の依存方向）、共通化方針、型選択を判断する
- mdhop 固有制約に触れるなら `review-plan-mdhop` 相当の観点で確認（依存方向、SQL 安全性、`modernc.org/sqlite` の罠、リンク解決ルール、ルート優先ルール、CLI 仕様、vault escape）

L1 にトリガする領域: SQLite スキーマ・マイグレーション、SQL（プレースホルダ・`GROUP BY`・集約関数・NULL 三値論理・exists_flag フィルタ）、リンク解決・ルート優先ルール（ADR 0004）、`cmd/mdhop → internal/core` の依存方向、CLI 破壊的変更、vault escape、`build` の `delete --rm` 等破壊的処理。

## Refactor Guard

変更対象に明らかな構造の悪さがある場合のみ、`refactor-guard` でリファクタ要否を判定する。
小さい修正・ロジック追加だけの変更では呼ばない。
判定で「先にリファクタ」となれば、リファクタを別 plan に切るか今回に含めるか判断する。

## Decision Criteria

- 原則 1 plan = 1 commit。独立した成果が混ざるなら plan を分ける
- 設計判断は採用案・却下案・理由を plan に記録
- 検証方針（自動 / CLI 動作確認 / ユーザー依頼）を plan に明記

## Plan Review

リスクに応じて選ぶ。

- **L0 (Small/Normal の単純なケース)**: self-check のみ。plan を省略する Small は plan review 自体スキップ
- **L1 (領域固有リスクあり)**: `review-plan-all` を呼ぶ。中で mdhop 制約 / split を選別
- **L2 (High-risk / 設計判断が重い / 曖昧)**: L1 に加えて Codex 観点を入れる

`review-plan-all` は内部でリスクに応じた skill を呼ぶ司令塔。詳細は当該スキル参照。

## Acceptance

- 実装対象、非対象、検証方針が明確
- 必要な仕様・backlog・decision の更新方針が明確
- レビュー指摘への対応が済んでいる、または対応しない理由が plan に書かれている
- 未解決の不明点がない（あればユーザー確認待ちとして停止）

## Stop Conditions

- 仕様・CLI 挙動・設計方針に複数の妥当な選択肢がある（即停止して確認）
- 1 commit に収まらない（plan を分ける）
- High-risk なのに検証方針がない
- レビュー周回が 3 周目に入る前 → 状況報告して指示待ち（選択肢提示しない）
