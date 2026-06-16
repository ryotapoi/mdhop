# Plan

## Intent

実装前に、要求・制約・設計判断・検証方針を必要十分な粒度で揃える。

## Plan Mode

plan mode（`EnterPlanMode` / `ExitPlanMode`）は使わない。承認待ちが Goal の自動進行と噛み合わないため。計画は内部で立て、そのまま `implement.md` へ進む。ユーザー確認が必要なのは Stop Conditions に該当する場合だけ。

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
- 関連する `docs/rules/`, `docs/specs/`（あれば）, `docs/decisions/`, `llm-wiki/knowledge.md`
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

ユーザーへの確認は plan の必須ステップではない。仕様・CLI 挙動に複数の妥当な選択肢が実際にある（Stop Conditions 該当）場合に止まって確認する。出力の見え方の確認は実装後に `verify.md` の方針で自動検証（`bin/mdhop` 実行）を優先し、確定できない場合だけ Stop Condition または残存リスクとして扱う。

## 設計判断

- 設計判断の前に `design-decision` スキルを呼ぶ
- ルールに当てはめても決まらないときだけユーザー確認
- モジュール配置（`cmd/mdhop` と `internal/core` の依存方向）、共通化方針、型選択を判断する。配置・責務・依存方向そのものを問う場合は `module-boundary` を使う
- mdhop 固有制約に触れるなら `mdhop-risk-check` で確認する（対象領域の一覧は `review.md` の領域固有 supplement を参照）

## 先行リファクタ判定

変更対象に明らかな構造の悪さがある場合のみ、機能追加の前に直すべきか判断する。判断は `design-decision` / `module-boundary` を使い、先行必須か別件か、今回に混ぜるか別 plan に切るかで分ける。
小さい修正・ロジック追加だけの変更では判定しない。

`backlog/backlog.md` の直近バージョンに計画済みのリファクタ指摘は既知として無視してよい。

## Decision Criteria

- 原則 1 plan = 1 commit。独立した成果が混ざるなら plan を分ける
- 設計判断は採用案・却下案・理由を plan に記録
- 検証方針（自動 / CLI 動作確認 / ユーザー依頼）を plan に明記

## Plan Review

- 通常は実装後レビュー（`review.md`）を標準とし、plan review は self-check でよい。
- 実装差分レビューでは Small 以外を原則 `/code-review xhigh` に通すため、plan 時点でもレビュー深度と追加 skill の要否を明記する。
- 領域固有リスクがあれば `mdhop-risk-check` を plan に当てる。
- High-risk / 設計判断が重い / 曖昧 / 実装後では手戻りが大きい場合だけ、`codex-review` でプランファイルを別系統レビューに回す。

## Acceptance

- 実装対象、非対象、検証方針が明確
- 必要な仕様・backlog・decision の更新方針が明確
- レビュー指摘への対応が済んでいる、または対応しない理由が plan に書かれている
- レビュー指摘に対応しない場合は、plan に**考慮したこと**（不要と判断した理由・別タスクに切り出す理由・トレードオフ）を事実と理由で書く（「対処済み」だけの完了宣言は不可）
- 未解決の不明点がない（あればユーザー確認待ちとして停止）

## Stop Conditions

- 仕様・CLI 挙動・設計方針に複数の妥当な選択肢がある（即停止して確認。ただし `design-decision` で結論が出る範囲なら止まらず採否を決める）
- 1 commit に収まらない（plan を分ける）
- High-risk なのに検証方針がない
