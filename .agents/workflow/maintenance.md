# Maintenance Workflow

この workflow は診断と報告のみを行う。棚卸し結果を backlog / docs へ反映する場合は、必要な項目を 1 commit 単位に分け、別の Change workflow として実行する。

## ICAR

- **Intent**: global `maintenance-audit` skill をこのプロジェクトの情報源・制約へ接続し、単一タスクの範囲を超えた整合性・設計境界・暫定対応・backlog 鮮度・検証戦略を棚卸しする。
- **Constraints**:
  - タスク内ではなく、節目で呼ぶ。タスク完了の度に呼ぶものではない。
  - 今回の差分ではなく、今後の変更コストを下げる観点で見る。
  - すぐ直すもの、backlog に積むもの、判断が必要なもの、意図的に無視するものを分ける。
  - 棚卸しで仕様や設計方針の変更が必要と分かったら、`docs/decisions/` または `docs/rules/` の更新を検討する。
  - 改善候補は、後続 Change で扱える 1 commit 粒度まで診断上分解する。
- **Acceptance**:
  - docs / tests / source / backlog の整合性、構造上の問題、リファクタ候補、テスト戦略の不足が整理されている。
  - 追跡が必要な改善は、後続 Change で `backlog/backlog.md` に反映できる粒度と根拠で提示されている。
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
- 棚卸し結果はその場で backlog / docs / source へ書き込まず、反映先（`backlog/backlog.md`、`docs/decisions/`、`docs/specs/`、source comment、`llm-wiki/`）と根拠を報告する。必要な反映は後続 Change として実行する。

## Stop Conditions

- 改善が大きすぎて複数タスクに分割すべき。
- プロダクト方針やアーキテクチャ方針の判断が必要。
