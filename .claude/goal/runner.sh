#!/usr/bin/env bash
# Goal runner: claude -p を1ループずつ呼び出す。
#
# 使い方:
#   .claude/goal/runner.sh
#
# 全体目標は backlog/goal.md、1ループの手順は backlog/goal-loop.md に書く。
# runner は内容を解釈しない。
#
# 環境変数:
#   MAX_LOOPS   ループ上限 (default: 50)
#   GOAL_DIR    backlog 配下の goal 関連ファイルがある場所 (default: backlog)
#
# 停止条件:
#   - backlog/goal-stop.md が存在 → exit 0
#   - backlog/goal-blocked.md が存在 → exit 2
#   - backlog/goal-done.md が存在 → exit 0 (Claude が Acceptance 達成時に作成)
#   - MAX_LOOPS 到達 → exit 3
#
# 権限について:
#   無人ループのため --permission-mode bypassPermissions を使う。
#   これは「全 tool / skill の許可確認をバイパス」する設定で、Bash 含めて何でも実行される。
#   runner はユーザーが信頼する環境（自分のマシン）で起動する前提。
#   信頼できない環境で回さないこと。

set -euo pipefail

GOAL_DIR="${GOAL_DIR:-backlog}"
GOAL="$GOAL_DIR/goal.md"
LOOP="$GOAL_DIR/goal-loop.md"
STATE="$GOAL_DIR/goal-state.md"
DECISIONS="$GOAL_DIR/goal-decisions.md"
STOP="$GOAL_DIR/goal-stop.md"
BLOCKED="$GOAL_DIR/goal-blocked.md"
DONE="$GOAL_DIR/goal-done.md"
WORKFLOW=".claude/goal/workflow.md"
MAX_LOOPS="${MAX_LOOPS:-50}"

for f in "$GOAL" "$LOOP" "$WORKFLOW"; do
  if [[ ! -f "$f" ]]; then
    echo "error: $f not found" >&2
    exit 1
  fi
done

echo "Goal file: $GOAL"
echo "Loop file: $LOOP"
echo "Max loops: $MAX_LOOPS"

for i in $(seq 1 "$MAX_LOOPS"); do
  echo
  echo "== goal loop $i / $MAX_LOOPS =="

  if [[ -f "$STOP" ]]; then
    echo "Stopped by $STOP"
    exit 0
  fi

  if [[ -f "$BLOCKED" ]]; then
    echo "Blocked by $BLOCKED"
    exit 2
  fi

  if [[ -f "$DONE" ]]; then
    echo "All tasks done ($DONE exists)"
    exit 0
  fi

  PROMPT=$(cat <<PROMPT
$WORKFLOW と $GOAL を読み、$GOAL の指示に従って処理してください。

前回からの引き継ぎ情報:
- $STATE
- $DECISIONS

読まないファイル:
- .claude/workflow/ 配下の phase ファイル（goal 用の派生は .claude/goal/workflow/ 配下にある）

実行条件:
- 質問しない
- 迷ったら自動判断する
- ユーザー意図と違う可能性がある判断だけ $DECISIONS に書く
- 状態は $STATE に書く
- EnterPlanMode は使わない
PROMPT
)

  claude -p "$PROMPT"
done

echo
echo "MAX_LOOPS reached: $MAX_LOOPS"
exit 3
