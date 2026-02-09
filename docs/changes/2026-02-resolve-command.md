# resolve コマンド: コアロジック実装

## Touches
- `internal/core/resolve.go` (新規) — `Resolve()`, `resolveLinkFromDB()`, エッジ検証, ノード取得
- `internal/core/resolve_test.go` (新規) — 15 テストケース
- `internal/core/db.go` — `upsertTag`/`upsertPhantom` の `LastInsertId` バグ修正
- `CLAUDE.md` — レビュースキル名更新・実装レビュー手順追加

## Verification
- `go test ./... -count=1` — 全 60 テスト PASS (23 parse + 22 build + 15 resolve)
- `/codex-impl-review` によるレビュー実施・指摘反映済み

## Pitfalls
- **`modernc.org/sqlite` の `LastInsertId` 挙動**: `ON CONFLICT DO NOTHING` 時に `LastInsertId()` が 0 ではなく直前の INSERT の rowid を返す。`upsertTag`/`upsertPhantom` で `RowsAffected()` による判定に修正。この問題は同一タグが複数ファイルから参照されると Build 時のエッジが不正な target_id を持つ形で顕在化する
- **DB read-only オープン**: `openDBAt(dbp + "?mode=ro")` は `modernc.org/sqlite` がクエリパラメータをファイル名の一部として扱い、別 DB を開いてしまうため断念。Resolve は DB を書き換えないので実害なし
