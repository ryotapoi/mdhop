#!/usr/bin/env bash
# Goal reset: 次のゴールを始める前に状態ファイルをテンプレートで初期化する。
#
# 使い方:
#   .claude/goal/reset.sh
#
# 動作:
#   - backlog/goal-done.md を削除（あれば）
#   - backlog/goal-blocked.md を削除（あれば）
#   - backlog/goal.md / goal-loop.md / goal-state.md / goal-decisions.md を
#     .claude/goal/templates/ で上書き
#
# タイミング:
#   ゴール達成後、goal-decisions.md の振り返りと codex レビューが終わってから
#   マスターが手動で叩く。runner.sh は自動で叩かない。

set -euo pipefail

GOAL_DIR="${GOAL_DIR:-backlog}"
TEMPLATES_DIR=".claude/goal/templates"

for f in goal.md goal-loop.md goal-state.md goal-decisions.md; do
  if [[ ! -f "$TEMPLATES_DIR/$f" ]]; then
    echo "error: $TEMPLATES_DIR/$f not found" >&2
    exit 1
  fi
done

echo "Removing runtime flags..."
rm -f "$GOAL_DIR/goal-done.md"
rm -f "$GOAL_DIR/goal-blocked.md"

echo "Restoring templates..."
cp "$TEMPLATES_DIR/goal.md" "$GOAL_DIR/goal.md"
cp "$TEMPLATES_DIR/goal-loop.md" "$GOAL_DIR/goal-loop.md"
cp "$TEMPLATES_DIR/goal-state.md" "$GOAL_DIR/goal-state.md"
cp "$TEMPLATES_DIR/goal-decisions.md" "$GOAL_DIR/goal-decisions.md"

echo "Done. 次のゴール開始前に backlog/goal.md の Intent / Acceptance / Relevant / Constraints を埋めてください。"
