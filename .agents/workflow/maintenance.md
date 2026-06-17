# Maintenance Workflow

## ICAR

- **Intent**: 単一タスクの範囲を超えて、構造・負債・重複・テスト戦略を棚卸しし、必要な改善タスクを作る。
- **Constraints**:
  - タスク内ではなく、節目で呼ぶ。タスク完了の度に呼ぶものではない。
  - 今回の差分ではなく、今後の変更コストを下げる観点で見る。
  - すぐ直すものと backlog に積むものを分ける。
  - 改善タスクは 1 commit に収まる粒度にする。
  - 仕様や設計方針の変更が必要なら `docs/decisions/` または `docs/rules/` 更新を検討する。
- **Acceptance**:
  - 構造上の問題、リファクタ候補、テスト戦略の不足が整理されている。
  - 必要な改善が `backlog/backlog.md` に追跡可能な形で入っている。
  - すぐ着手する改善と先送りする改善が分かれている。
- **Relevant**:
  - 最近の git history
  - `backlog/backlog.md`
  - 変更が多かったモジュール
  - `docs/rules/architecture.md`
  - `llm-wiki/`（作業地図）

## Tools

- 棚卸し・健康診断: `maintenance-audit` skill（軽い整合性・負債・backlog 鮮度の light pass から、テスト・カバレッジ・行数・依存方向・凝集度・分割の deep pass まで、scope で深さを指定）
- module / 配置 / 依存方向の境界判断: `module-boundary` skill
- `llm-wiki/` の地図健全性: `wiki-lint` skill（孤立・リンク切れ・sources 切れの機械検証＋「速い / docs レベルでない / 嘘がない / 拾える」の不変条件照合）

## Flow ICAR

### Maintenance Pass

- **Intent**: 単一差分を超える構造劣化、負債、重複、テスト戦略の不足を見つける。
- **Acceptance**: すぐ直すものと backlog に積むものが分かれている。
- **Relevant**: 最近の git history、変更が多かったモジュール、関連 rules / llm-wiki。

### Backlog Sync

- **Intent**: 棚卸し結果を追跡可能な改善タスクにする。
- **Acceptance**: 必要な改善が `backlog/backlog.md` に入っている。
- **Relevant**: `backlog/backlog.md`。

## Use When

- 複数コミットやマイルストーンの区切り
- 同じ種類の修正が続いている
- 実装中やレビューでリファクタ候補が複数出た
- 久々に広い領域を触った
- review で同種の指摘が繰り返された

## Stop Conditions

- 改善が大きすぎて複数タスクに分割すべき。
- プロダクト方針やアーキテクチャ方針の判断が必要。
