# ADR 0001: 技術スタック方針

## Status

Accepted

## Context

- 主な利用者は LLM/Coding Agent であり、速度は最優先ではない
- ただし通常の CLI ツールとしても利用するため、起動や操作が軽い方が望ましい
- ローカル完結の SQLite を前提に進める

## Decision

- 実装言語は Go
- データベースは SQLite（ドライバ: `modernc.org/sqlite`、純 Go 実装）
- CLI は標準ライブラリ `flag` のみ（外部フレームワークなし）
- Go 1.21 以上

## Rationale

- Go は単一バイナリで配布・実行が簡単
- CLI 実装が軽量で起動が速い
- SQLite 統合が容易
- 仕様検証と実装の反復がしやすい

## Consequences

- cgo 依存なしの単一バイナリ配布
- 外部 CLI フレームワークなしにより依存が最小限
- Rust への移行は現実的ではなくなった（全コマンド実装済み）
