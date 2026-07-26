# モデル定義

Goal workflow の役割（Conductor / Implementer / Gatekeeper / Advisor / Auditor）に指定できるモデルと reasoning effort の正本。ルール（`goal.md` 等）はこの表を参照し、モデル名・序列・effort 値をハードコードしない。モデルの世代交代はこのファイルだけ更新する。

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
| GPT 系 | none / low / medium / high / xhigh / max（`-c model_reasoning_effort=<値>` で指定） | medium |

GPT 系の有効値は GPT-5.6 公式ガイド（2026-07-25 参照）による。GPT-5.5 世代にあった `minimal` は 5.6 で廃止され、`max` が追加された。`xhigh` までは 5.5 世代の API エラー応答で実測済み（2026-07-13）、`max` は未実測。

## 役割の既定

| 入口 | Conductor | Implementer | Gatekeeper | Advisor | Auditor |
|---|---|---|---|---|---|
| Claude 側（`.claude/workflow/`） | opus | sonnet | sonnet | fable 固定 | opus |
| GPT 側（`.agents/workflow/`） | sol | terra | terra | sol 固定 | sol |

Conductor は Goal を起動した main セッション自身なので、この列は workflow が選択する値ではなく、ユーザーがセッションを起動する時の推奨モデル。他の役割は workflow が subagent / exec 起動時にこの表の値を使う。

Goal Review の reviewer は入口によらず 2 本で、系統ごとに次を既定とする。

| 系統 | Goal Review reviewer | 起動経路 |
|---|---|---|
| Claude 系 | fable | `claude-fresh-review` skill（呼び出し元がレビューモデルを明示する） |
| GPT 系 | sol | `codex-fresh-review` skill（skill 側がモデルを固定して持つ。呼び出し側からは指定しない） |

effort の既定はどの入口・役割でも系統のベンダー推奨既定。例外が 1 つある: Claude 系の Implementer は、Intake 分類が High-risk の Change では `xhigh` を既定にする（Sonnet 5 公式ガイドが最難関のコーディング・agentic タスクに `xhigh` を推奨しているため）。この既定は Change の Intake 分類に紐づき、失敗観測後にモデルを引き上げた場合も High-risk の間は変わらない。GPT 系には対応する事前引き上げを置かない（GPT-5.6 公式ガイドは high / xhigh を「測って効果がある場合」に限って勧めており、一律の事前推奨の根拠がないため。GPT 系の引き上げは失敗観測後の是正のみ）。

Implementer / Gatekeeper は Goal の呼び出し文でモデルと effort を役割ごとに明示指定できる。指定できる短名は入口の系統に限る（Claude 側の workflow は Claude 系短名のみ、GPT 側の workflow は GPT 系短名のみ）。他系統の短名を指定されたら停止してユーザーに確認する。Conductor / Advisor / Auditor は既定固定で、Goal 呼び出し文からは指定しない。
