# docs

mdhop のドキュメントは層を分けて管理する。
依存方向は「下向きのみ」。上流の内容を下流に反映する。

## 1. Architecture（内部設計）

コードから復元できない情報と、内部設計の骨格を保持する。

- `architecture/01-concept.md`: コンセプト・狙い・思想・ユースケース
- `architecture/02-requirements.md`: 要件（機能/非機能/運用）
- `architecture/03-data-model.md`: データモデルと詳細

## 2. 外部仕様（ユーザー視点）

ユーザー視点の挙動、互換性、制約、非目標をまとめる。

- `external/overview.md`: 外部仕様（統合版）
- `external/stories.md`: 代表ストーリー（実際の操作フロー）

## 3. 設計判断の理由（ADR）

設計判断の背景と理由を残す。

- `adr/0001-tech-stack.md`: 技術スタックの方針（言語/DB/運用）

## 4. 変更ログ

実装完了後の記録。「どこを触ったか・何で確認したか・何で詰まったか」を残す。

- `changes/YYYY-MM-機能名.md`: 各実装の Change Note
