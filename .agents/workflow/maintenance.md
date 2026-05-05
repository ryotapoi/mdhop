# Maintenance Workflow

## ICAR

- **Intent**: 単一タスクの範囲を超えて、構造・負債・重複・テスト戦略を棚卸しし、必要な改善タスクを作る。
- **Constraints**:
  - タスク内ではなく、節目で呼ぶ。タスク完了の度に呼ぶものではない。
  - 今回の差分ではなく、今後の変更コストを下げる観点で見る。
  - すぐ直すものと backlog に積むものを分ける。
  - 改善タスクは 1 commit に収まる粒度にする。
- **Acceptance**:
  - 構造上の問題、リファクタ候補、テスト戦略の不足が整理されている。
  - 必要な改善が `backlog/backlog.md` に追跡可能な形で入っている。
  - すぐ着手する改善と先送りする改善が分かれている。
- **Relevant**:
  - 最近の git history
  - `backlog/backlog.md`
  - 変更が多かったモジュール
  - `rules/architecture.md`
  - `references/knowledge.md`

## Use When

- 複数コミットやマイルストーンの区切り
- 同じ種類の修正が続いている
- 実装中やレビューでリファクタ候補が複数出た
- 久々に広い領域を触った
- review で同種の指摘が繰り返された

## Stop Conditions

- 改善が大きすぎて複数タスクに分割すべき。
- プロダクト方針やアーキテクチャ方針の判断が必要。
