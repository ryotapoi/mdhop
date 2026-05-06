# Goal State

status: done

## Current scope

`backlog/backlog.md` の v0.7.0 の全タスク完了。

## Last completed loop

2026-05-07 (A パス): backlog v0.7.0「サンプルスキル更新」を完了。これで v0.7.0 の全タスクが `- [x]` 化されたため、`goal-done.md` を作成して全体ゴール達成。

実装内容:
- `examples/skills/mdhop/SKILL.md`: ヘッダー直下に frontmatter wikilink の挙動 1 段落を追加（query/search で edge として現れる、add/update/move/disambiguate は書き換え対象、convert/simplify/repair は本文のみ）
- `examples/skills/mdhop/references/commands.md`:
  - `move` セクション "Behavior (single file)" の Rewrites links 箇条書きに「Wikilinks inside frontmatter values are rewritten the same way; quoted vs bare YAML style is preserved」を追加
  - `repair` セクションの「URL links, tag links, and frontmatter links are not affected」→「frontmatter values (including wikilinks inside frontmatter)」に正確化
  - `convert` セクションも同様に正確化、加えて変換対象を「note bodies」と明示
  - `simplify` セクションに「Wikilinks inside frontmatter values are not shortened」を箇条書きで追加

設計判断（goal-decisions.md 記録済み）:
- examples/skills/ の構成が memory 記述「3 サブディレクトリ」と異なり実際は `mdhop/` 1 つのみ → 現状を正として更新（memory が古い）
- ドキュメントは「外部仕様の編集」ルール（`.claude/rules/docs.md`）に従い、ユーザー視点の挙動・非目標のみ記述。内部実装（rewrite.go の `pathLinkTypeSQLList` 等）には触れない

レビュー: self-check（ドキュメント修正のみ、Small 分類）

## Skipped tasks

なし

## Last verification

- ドキュメント変更のため `go test ./...` 等は実施せず
- リンク参照（`references/query.md`, `references/commands.md`）の整合性のみ目視確認

## Next hint

v0.7.0 完了済み。次回 runner 起動時、`goal-done.md` の存在を検知して終了する想定。次のゴールは v0.8.0 等の別バックログ化が必要。
