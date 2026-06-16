# Investigate

## Intent

計画や実装に入る前に、必要な事実・不明点・判断材料を揃える。

## Use When

- 原因不明のバグ
- 仕様や期待挙動が曖昧
- 技術検証が必要
- CLI 出力・`bin/mdhop` の挙動・実 vault・SQLite DB 依存など、コードだけでは確定できない挙動がある

## Inputs

- ユーザー依頼
- `backlog/backlog.md` の該当項目
- 関連する `docs/rules/`, `docs/specs/`（あれば）, `docs/decisions/`, `llm-wiki/knowledge.md`
- 既存コード、ログ、再現手順

## Decision Criteria

- 何が分かれば plan / direct implement / stop に進めるかを先に定義する
- 机上で分からない挙動はコード読みを続けず、`bin/mdhop <args>` を実行して挙動を見るか、ユーザーに再現手順を確認する
- 複数ファイル横断や広域 grep は Explore subagent に委譲する。ファイル 1〜2 個で済むなら main で Read する
- ユーザーに聞いた方が早い領域は遠慮せず聞く（CLI 出力フォーマット、引数の解釈、エラー時の振る舞い、再現コマンド）
- 調査結果が将来も効くなら `llm-wiki/knowledge.md`、要求や粒度が変わるなら `backlog/backlog.md` に記録する
- 調査用の一時コードは、残す理由がなければ最終成果に含めない

## Acceptance

- 判明した事実と残った不明点が説明できる
- 次に plan / direct implement / stop のどれに進むか判断できる
- 永続化が必要な知見・要求変更が適切な場所に記録されている

## Stop Conditions

- ユーザーの観察・判断なしに確定できない CLI 挙動・実機再現がある
- 調査結果により元の要求やスコープが変わった
- 検証用の一時変更を残すか戻すか判断が必要
