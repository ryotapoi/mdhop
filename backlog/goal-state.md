# Goal State

status: done

## Current scope

v0.7.0 の未完了タスクをすべて完了させる → 達成

## Last completed loop

2026-05-07: 前ループで導入した `stripYAMLComment` の block scalar リグレッションを修正（ADR 0013 整合性回復）

- `internal/core/parse.go` に `collectBlockScalarLines` を追加。`yaml.Node.Style == LiteralStyle/FoldedStyle` の値ノードについて body 行範囲を file line 1-based の set として返す
- `parseFrontmatterWikilinks` のシグネチャに `blockScalarLines map[int]bool` を追加し、対象行は `stripYAMLComment` 適用をスキップ
- 単体テスト追加: `TestParseFrontmatterWikilinkBlockScalarKeepsHash` (`note: |` と `folded: >` の両方で `# [[B]]` / `# [[C]]` が edge 化される)
- build レベル再現テスト追加: `TestBuildFrontmatterWikilinkInBlockScalar` + 新 fixture `testdata/vault_build_frontmatter_block_scalar/`（`A.md` の frontmatter に block scalar / folded scalar を持たせ、それぞれ `B.md` / `C.md` への outgoing edge を期待）
- 経緯: マスターによるレビューで「`stripYAMLComment` が block scalar 内 wikilink を edge から落とす（ADR 0013:43 違反）」を指摘。前ループの `goal-decisions.md` 「YAML コメント除去スコープ」案A は ADR を読まずに行った誤判断と判明し、案A を撤回して block scalar 行を例外扱いする実装に修正
- `go vet ./...` 無出力 / `go test -count=1 ./...` cmd/mdhop と internal/core ともに ok / `go build ./...` 成功

## Skipped tasks

なし

## Last verification

`go vet ./...`（無出力）、`go test -count=1 ./...`（cmd/mdhop と internal/core ともに ok）

## Next hint

v0.7.0 全タスク完了 + ADR 0013 整合性回復済み。`goal-done.md` を作成。次のループは v0.7.0 リリース手続き or 新バージョン計画（ユーザー指示待ち）。
