# Goal State

status: done

## Current scope

v0.7.0 の未完了タスクをすべて完了させる → 達成

## Last completed loop

2026-05-07: block scalar 終端判定の過検出バグ修正 + ドキュメント整合性回復

- 直前のレビューで「`# [[B]]` が block scalar 終端後の top-level YAML コメントとして書かれていても edge 化される」過検出を指摘。`collectBlockScalarLines` が `bodyStart..nextKey-1` を一律本文扱いにしていたため、間に挟まる top-level コメント行まで `stripYAMLComment` 除外対象になっていた
- 修正: `collectBlockScalarLines` のシグネチャを `(content, lines)` に変更し、本文行を「インデントが key.Column より深い行（または空行）」に絞る。key indent 以下の非空行で本文を打ち切る。YAML 1.2 §8.1 の block scalar indentation rule に従う
- 単体テスト追加: `TestParseFrontmatterWikilinkTopLevelCommentAfterBlockScalar`（`note: |` 本文の後に top-level `# [[B]]` コメントを置き、`next: "[[C]]"` を続ける構造で C のみが edge 化されることを検証）
- ドキュメント同期（前段の v0.7.0 完了に伴う棚卸し）: `examples/skills/mdhop/SKILL.md` と `references/commands.md` の simplify 記述を frontmatter wikilink 対応版に更新、`README.md` / `README.ja.md` のコマンド一覧に欠けていた `simplify` / `convert` / `repair` / `search` / `init-meta` を追加
- `references/knowledge.md` の block scalar 知見をインデント判定込みに更新
- `go vet ./...` 無出力 / `go test -count=1 ./...` cmd/mdhop と internal/core ともに ok

## Skipped tasks

なし

## Last verification

`go vet ./...`（無出力）、`go test -count=1 ./...`（cmd/mdhop と internal/core ともに ok）

## Next hint

v0.7.0 全タスク完了 + ADR 0013 整合性回復済み。`goal-done.md` を作成。次のループは v0.7.0 リリース手続き or 新バージョン計画（ユーザー指示待ち）。
