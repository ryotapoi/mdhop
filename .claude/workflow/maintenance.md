# Maintenance Workflow

この workflow も、実行する場合は 1 commit に収まる棚卸し・backlog 整理単位として扱う。
Goal の節目で構造を見る場合も、Goal 全体の実装 commit とは混ぜない。

## ICAR

- **Intent**: global `maintenance-audit` skill をこのプロジェクトの情報源・制約へ接続し、単一タスクの範囲を超えた整合性・設計境界・暫定対応・backlog 鮮度・検証戦略を棚卸しする。
- **Constraints**:
  - タスク内ではなく、節目で呼ぶ。タスク完了の度に呼ぶものではない。
  - 今回の差分ではなく、今後の変更コストを下げる観点で見る。
  - すぐ直すもの、backlog に積むもの、判断が必要なもの、意図的に無視するものを分ける。
  - 棚卸しで仕様や設計方針の変更が必要と分かったら、`docs/decisions/` または `docs/rules/` の更新を検討する。
  - 改善タスクは 1 commit に収まる粒度にする。
- **Acceptance**:
  - docs / tests / source / backlog の整合性、構造上の問題、リファクタ候補、テスト戦略の不足が整理されている。
  - 必要な改善が `backlog/backlog.md` に追跡可能な形で入っている。
  - すぐ着手する改善と先送りする改善が分かれている。
- **Relevant**:
  - global `maintenance-audit` skill
  - 最近の git history
  - `backlog/backlog.md`
  - 変更が多かったモジュール
  - `docs/rules/`, `docs/specs/`, `docs/decisions/`, `llm-wiki/`（作業地図）

## Use When

- 複数コミットやマイルストーンの区切り
- 同じ種類の修正が続いている
- 実装中やレビューでリファクタ候補が複数出た
- 久々に広い領域を触った
- review で同種の指摘が繰り返された

## Flow ICAR

### Tools

- `maintenance-audit` skill を入口にし、軽い整合性・負債・backlog 鮮度の light pass から、テスト・カバレッジ・行数・依存方向・凝集度・分割の deep pass まで、scope で深さを指定する。
- 設計判断が必要な場合は `design-decision`、配置・target・package・外部依存・責務境界が論点なら `module-boundary` を併用する。
- `llm-wiki/` の地図健全性を見る節目では `wiki-lint` を併用し、孤立・リンク切れ・sources 切れと 4 不変条件（速い / docs レベルでない / 嘘がない / 拾える）を点検する。
- プロジェクト固有制約に触れる場合は `project-risk-check` を併用する。構造劣化リスクがあれば `thermo-nuclear-code-quality-review` を使う。

### Adapter

- `docs/rules/`, `docs/specs/`, `docs/decisions/`, `backlog/`, `llm-wiki/`, tests, source code を同じプロジェクト文書群として扱い、矛盾や古い前提を探す。
- backlog 自体も鮮度確認の対象にする。完了済み、重複、古い前提、粒度過大、現状 architecture と矛盾する項目は整理候補にする。
- 一時対応、固定値で通したテスト、skip / disabled test、TODO / FIXME、暫定 fallback、設計・境界判断の先送りが残っていないかを見る。
- 棚卸し結果は、必要に応じて `backlog/backlog.md` に 1 commit 粒度で反映する。後から制約になる判断は `docs/decisions/`、振る舞い仕様は `docs/specs/`、技術知見は特定ソースの罠ならコードコメント・横断的な挙動なら `llm-wiki/` の地図に同期する。
- 棚卸しで見つけた改善は診断のみで、その場で修正に着手しない。修正着手の可否は `boundary-control` で分類する（maintenance の active scope は診断と backlog 反映であり、改善の実装は adjacent として別タスク化する）。

## Stop Conditions

- 改善が大きすぎて複数タスクに分割すべき。
- プロダクト方針やアーキテクチャ方針の判断が必要。
