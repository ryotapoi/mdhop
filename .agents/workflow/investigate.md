# Investigate Workflow

## ICAR

- **Intent**: 計画や実装に入る前に、必要な事実・不明点・判断材料を揃える。
- **Constraints**:
  - 何が分かれば plan / direct implement / stop に進めるかを先に定義する。
  - コードだけでは確定できない CLI 出力・実 vault・SQLite DB 依存の挙動は、`bin/mdhop <args>` やテスト用 vault で実測する。
  - subagent は、複数ファイル横断・広域 grep・独立した仮説検証を並列化できる場合に使う。
  - 調査中の一時コードは、残す理由がなければ最終成果に含めない。
- **Acceptance**:
  - 判明した事実と残った不明点が説明できる。
  - 次に plan / direct implement / stop のどれに進むか判断できる。
  - 永続化が必要な知見・要求変更が適切な場所に記録されている。
- **Relevant**:
  - ユーザー依頼
  - `backlog/backlog.md` の該当項目
  - 関連する `docs/rules/`, `docs/decisions/`, `llm-wiki/`（作業地図）, `docs/specs/`（あれば）
  - 既存コード、ログ、再現手順

## Use When

- 原因不明のバグ
- 仕様や期待挙動が曖昧
- 技術検証が必要
- CLI 出力・実 vault・SQLite DB 副作用など、コードだけでは確定できない挙動がある

## Recording

- 調査結果が将来も効くなら、特定ソースに紐づくものはそのコードのコメントへ、横断的な挙動は `llm-wiki/` の該当地図へ残す。単一の集約ファイルは作らない。
- 要求や粒度が変わるなら `backlog/backlog.md` に残す。
- ユーザーに聞いた方が早い GUI / 観察依存の挙動だけ確認する。

## Stop Conditions

- ユーザーの観察・判断なしに確定できない挙動がある。
- 調査結果により、元の要求やスコープが変わった。
- 検証のための一時変更を残すか戻すか判断が必要。
