---
paths:
  - "docs/**"
  - "llm-wiki/**"
  - "backlog/**"
---

# ドキュメントルール

権威による配置（docs/ = 正本 / llm-wiki/ = 正本に負ける作業の入口）の正本は `docs/rules/information-management.md`。ここはその要点だけを再掲する。

## 正（source of truth）

- 正本は `docs/`（rules / specs / decisions）と「ソースコード・テスト」のみ。矛盾したら正本が勝つ
- `llm-wiki/` は正本ではない。ソース/正本と矛盾したらそちらが勝つ。各ファイルの `regen`（full / compiled / none）で再生成可否を宣言する
- 同じ情報を2箇所に書かない（DRY）。llm-wiki/ の `regen: full` / `compiled` は正本を再掲せずポインタ（パス・行・ADR 番号）だけを持つ
- 実装の詳細はコードに置く
- 設計判断の理由は ADR に記録する。コードから復元できない情報だけ残す
- ソースから機械再抽出できる索引・地図は手書きで正本に置かない。llm-wiki/ の `regen: full` に置き、腐ったら作り直す
- `regen: none`（外部知見）が設計判断や仕様を拘束し始めたら docs/decisions か docs/specs へ昇格させる

## 外部仕様の編集

- ユーザー視点の挙動・互換性・制約・非目標のみ書く
- 内部実装の手順は書かない
- 詳細はテストとコードに寄せる
