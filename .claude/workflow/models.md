# モデル定義

Goal workflow の役割（Conductor / Implementer / Gatekeeper / Advisor）に指定できるモデルと reasoning effort の正本。ルール（`goal.md` 等）はこの表を参照し、モデル名・序列・effort 値をハードコードしない。モデルの世代交代はこのファイルだけ更新する。

## モデル一覧

同一系統内で下の行ほど上位。「1 段上」は同系統で 1 行下のモデルを指す（最上位に上はない）。系統は跨がない。

### Claude 系（起動はネイティブ subagent。`model` に短名をそのまま渡す）

| 短名 | 起動指定 |
|---|---|
| haiku | `haiku` |
| sonnet | `sonnet` |
| opus | `opus` |
| fable | `fable` |

### GPT 系（起動は codex exec。`-m` にフル ID を渡す）

| 短名 | 起動指定 |
|---|---|
| luna | `-m gpt-5.6-luna` |
| terra | `-m gpt-5.6-terra` |
| sol | `-m gpt-5.6-sol` |

## reasoning effort

| 系統 | 有効値（低→高） | ベンダー推奨既定 |
|---|---|---|
| Claude 系 | low / medium / high / xhigh / max | high |
| GPT 系 | none / minimal / low / medium / high / xhigh（`-c model_reasoning_effort=<値>` で指定） | medium |

GPT 系の有効値は API のエラー応答で実測済み（2026-07-13、`xhigh` は実動確認済み）。

## 役割の既定

| 入口 | Conductor | Implementer | Gatekeeper | Advisor |
|---|---|---|---|---|
| Claude 側（`.claude/workflow/`） | opus | sonnet | sonnet | fable 固定 |
| GPT 側（`.agents/workflow/`） | terra | terra | terra | sol 固定 |

Goal Review の reviewer は入口によらず 2 本で、系統ごとに次を既定とする。

| 系統 | Goal Review reviewer | 起動経路 |
|---|---|---|
| Claude 系 | fable | `claude-fresh-review` skill（呼び出し元がレビューモデルを明示する） |
| GPT 系 | sol | `codex-fresh-review` skill（skill 側がモデルを固定して持つ。呼び出し側からは指定しない） |

effort の既定はどの入口・役割でも系統のベンダー推奨既定。

Implementer / Gatekeeper は Goal の呼び出し文でモデルと effort を役割ごとに明示指定できる。指定できる短名は入口の系統に限る（Claude 側の workflow は Claude 系短名のみ、GPT 側の workflow は GPT 系短名のみ）。他系統の短名を指定されたら停止してユーザーに確認する。Conductor と Advisor は既定固定で、Goal 呼び出し文からは指定しない。
