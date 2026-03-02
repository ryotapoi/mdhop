# ADR 0005: delete --rm flag for disk file removal

## Status

Accepted

## Context

delete コマンドは現在、ファイルが事前に手動削除されていることを前提とする。move はファイルリネームを自動実行するため、delete と非対称。Agent がファイル削除→インデックス更新を1コマンドで完結できると効率的。

## Considered Options

- **CLI 層で os.Remove**: 簡単だが、登録チェック→ファイル削除→DB更新の順序保証が崩れる
- **core 層で os.Remove（選択）**: 登録チェック後にファイル削除し、DB更新まで一連で実行
- **常にファイル削除**: 後方互換が崩れる。事前に手動削除済みのワークフローが壊れる

## Decision

We will add `--rm` flag to delete, implemented in core layer:

- `DeleteOptions.RemoveFiles` が true の場合、core 層で `os.Remove` を実行
- 登録チェック（Phase 1）→ vault 内チェック → `os.Remove` → DB 更新の順
- `os.IsNotExist` は無視（冪等）、その他のエラーは即座に返却
- `RemoveFiles=false` は従来通り（ファイル事前削除を前提）

## Consequences

- move と対称的な操作が可能になる（ファイル変更系コマンドが fs 操作を持つ）
- `os.Remove` 成功後に TX 失敗した場合、ファイルは復元しない（冪等性により `--rm` なしで再実行すれば DB は復旧可能）
- パストラバーサル対策として vault 内チェックを行う
