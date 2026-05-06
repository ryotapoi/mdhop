# Investigate (goal)

## Intent

計画や実装に入る前に、必要な事実・不明点・判断材料を揃える。

## Use When

- 原因不明のバグ
- 仕様や期待挙動が曖昧
- 技術検証が必要
- CLI 出力・`bin/mdhop` の挙動・実 vault・SQLite DB 依存など、コードだけでは確定できない挙動がある

## Inputs

- goal-loop で選ばれた1タスク
- `backlog/backlog.md` の該当項目
- 関連する `rules/`, `specs/`（あれば）, `decisions/`, `references/knowledge.md`
- 既存コード、ログ、再現手順

## Decision Criteria

- 何が分かれば plan / direct implement / stop に進めるかを先に定義する
- 机上で分からない挙動はコード読みを続けず、`bin/mdhop <args>` を実行して挙動を見る。実機ユーザー観察が必須なら `goal-blocked.md` を作成して停止する
- 複数ファイル横断や広域 grep は Explore subagent に委譲する。ファイル 1〜2 個で済むなら main で Read する
- ユーザーに聞かないと進めない領域（CLI 出力フォーマット、引数の解釈、エラー時の振る舞い、再現コマンド）は `goal-blocked.md` を作成して停止する
- 調査結果が将来も効くなら `references/knowledge.md`、要求や粒度が変わるなら `backlog/backlog.md` に記録する（B パスに移行する場合は backlog 更新後 A 中断）
- 調査用の一時コードは、残す理由がなければ最終成果に含めない

## Acceptance

- 判明した事実と残った不明点が説明できる
- 次に plan / direct implement / stop のどれに進むか判断できる
- 永続化が必要な知見・要求変更が適切な場所に記録されている

## Stop Conditions

次の場合は `goal-blocked.md` を作成して停止する。

- ユーザーの観察・判断なしに確定できない CLI 挙動・実機再現がある
- 調査結果により元の要求やスコープが変わり、自動判断ではタスク再定義不能（B パスでも対応不能）
- 検証用の一時変更を残すか戻すか判断が必要
