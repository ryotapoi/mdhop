# ADR 0003: move auto-detects disk state

## Status

Accepted

## Context

move コマンドはファイル移動 + インデックス更新 + リンク書き換えを一発で行う。
しかし、ユーザーが先に `mv` でファイルを移動済みの状態で `mdhop move` を実行する場面が想定される。
この場合、from がディスクにないためエラーにすると、ユーザーは mv を元に戻す必要がありユーザー体験が悪い。

## Considered Options

- **常にエラー**: from がディスクにない場合は常にエラー。ユーザーに mv を戻させる
- **フラグで切り替え**: `--already-moved` のようなフラグで明示的に切り替える
- **ディスク状態を自動検知**: from/to のディスク存在を見て動作を自動的に切り替える

## Decision

We will auto-detect the disk state and adapt behavior accordingly:

- from がディスクにあり to がない → ディスク移動 + リライト + DB 更新
- from がディスクになく to がある → リライト + DB 更新のみ（ディスク移動スキップ）
- from も to もない → エラー
- from も to もある → エラー（上書き防止）

フラグは不要。4パターンの判定で十分。

## Consequences

- ユーザーは `mv` の前後どちらでも `mdhop move` を呼べる
- 自動検知のため、ユーザーが意図しない状態（例: 別ファイルが to にある）でも動作する可能性がある。ただし DB 上の from が登録されていなければエラーになるため、実害は限定的
- 既に移動済みケースでは stale チェックが to のファイルに対して行われる（`os.Rename` は mtime を保持するため）
