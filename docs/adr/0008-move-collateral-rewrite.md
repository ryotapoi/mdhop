# ADR 0008: move コマンドでコラテラルリンク書き換え

## Status

Accepted

## Context

`move` コマンドで basename 衝突が発生する場合（例: `A.md` → `sub2/B.md`、既存 `sub1/B.md`）、第三者ファイルの `[[B]]` リンクが曖昧になるケースや、移動ファイル自身の outgoing basename リンクの解決先が変わるケースが存在する。

従来はこれらをエラーにして move を拒否していた。しかし、既存リンクの指し先は移動前の DB から明確に判明しており、フルパスに書き換えれば意味を保てる。

## Considered Options

- **A: エラーで拒否（現状維持）**: 安全だがユーザーが手動でリンクを書き換えてから再試行する必要がある
- **B: 自動でフルパスに書き換え（コラテラル書き換え）**: move を成功させつつリンクの意味を保持する
- **C: `--force` フラグで選択式**: ユーザーに判断を委ねるが、CLI の複雑性が増す

## Decision

We will automatically rewrite ambiguous or meaning-changing basename links to full paths (option B). This applies to:

1. Third-party files whose basename links would become ambiguous after the move (collateral rewrite)
2. The moved file's own outgoing basename links whose resolution would change after the move

The rewrite target path is determined from the pre-move DB state, ensuring the link's original meaning is preserved.

## Consequences

- move コマンドが以前エラーだったケースで成功するようになり、ユーザー体験が向上する
- stale チェックの対象がコラテラル書き換え対象ファイルにも拡大する（安全性は維持）
- フルパス化されたリンクは basename リンクより冗長だが、意味の正確性を優先する
- `add` のデフォルト動作（auto-disambiguate）と同様のアプローチで一貫性がある
