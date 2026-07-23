# Investigate Workflow

## ICAR

- **Intent**: 計画や実装に入る前に、必要な事実・不明点・判断材料を揃える。
- **Constraints**:
  - 机上で分からない挙動はコード読みを続けず、計測・確認手段に切り替える。
    <!-- slot: コード確認以外に使いたい確認手段があれば記載する（例: Preview / アプリ起動 / 公式ドキュメント、CLI なら実行して挙動を見る、実機・外部連携はユーザー確認）。 -->
    - CLI 挙動は `go build -o bin/mdhop ./cmd/mdhop` 後に `bin/mdhop <args>` を実行し、stdout / stderr / 終了コード / DB 副作用を確認する。
    <!-- /slot -->
  - subagent は、複数ファイル横断・広域 grep・独立した仮説検証を並列化できる場合に使う。
- **Acceptance**:
  - 判明した事実と残った不明点が説明できる。
  - 次に plan / direct implement / stop のどれに進むか判断できる。
  - 永続化が必要な知見・要求変更が適切な場所に記録されている。
- **Relevant**:
  - ユーザー依頼
  - `backlog/backlog.md` の該当項目
  - 関連する `docs/rules/`, `docs/specs/`, `docs/decisions/`, `llm-wiki/`（作業地図）
  - 既存コード、ログ、再現手順

## Use When

- 原因不明のバグ
- 仕様や期待挙動が曖昧
- 技術検証が必要
- UI / 実機 / 外部 API など、コードだけでは確定できない挙動がある

## Recording

- 調査で得た知見・確定した要求変更の記録先は `docs/rules/information-management.md` に従う。

## Stop Conditions

- 自力で取れる証拠や代替確認では、完了判断に必須の UI / 挙動を確定できない。
- 調査結果により、元の要求やスコープが変わり、現在の Goal / Change のまま進めることが不適切。
- 検証のための一時変更を残すか戻すか判断が必要。
