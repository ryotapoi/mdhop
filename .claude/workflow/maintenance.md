# Maintenance (L3 Review)

## Intent

単一タスクの範囲を超えて、構造・負債・重複・テスト戦略を棚卸しし、必要な改善タスクを作る。

## Use When

- 複数コミットやマイルストーン（v0.6, v0.7 など）の区切り
- 同じ種類の修正が続いている
- 実装中やレビューでリファクタ候補が複数出た
- 久々に広い領域を触った
- review で同種の指摘が繰り返された

タスク内ではなく、節目で呼ぶ。タスク完了の度に呼ぶものではない。

## Inputs

- 最近の git history
- `backlog/backlog.md`
- 変更が多かったモジュール（`internal/core/` 配下、`cmd/mdhop/` 等）
- `rules/architecture.md`, `references/knowledge.md`

## Tools

- 軽い棚卸し（goal / マイルストーン区切りの整合性・負債・backlog 鮮度）: `maintenance-review` スキル
- 重い健康診断 + 構造見直し（テスト・カバレッジ・行数・依存方向・凝集度・分割）: `maintenance-audit` スキル
- module / 配置 / 依存方向の境界判断: `module-boundary` スキル

## Decision Criteria

- 今回の差分ではなく、今後の変更コストを下げる観点で見る
- すぐ直すものと backlog に積むものを分ける
- 改善タスクは 1 commit に収まる粒度にする
- 仕様や設計方針の変更が必要なら `decisions/` または `rules/` 更新を検討する

## Acceptance

- 構造上の問題、リファクタ候補、テスト戦略の不足が整理されている
- 必要な改善が `backlog/backlog.md` に追跡可能な形で入っている
- すぐ着手する改善と先送りする改善が分かれている

## Stop Conditions

- 改善が大きすぎて複数タスクに分割すべき
- プロダクト方針やアーキテクチャ方針の判断が必要
