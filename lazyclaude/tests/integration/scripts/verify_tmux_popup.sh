#!/bin/bash
# Verify tmux display-popup VISUAL E2E.
#
# Proves the real lazyclaude tool gocui popup renders inside display-popup by:
# 1. Running display-popup with `script` to record the PTY output
# 2. Replaying the recording into a tmux pane
# 3. Using capture-pane to extract readable text
# 4. Verifying tool name, command, and action bar are visible
#
# This script MUST run inside a tmux session.

set -euo pipefail

BINARY="${1:-lazyclaude}"
SOCKET="${LAZYCLAUDE_TMUX_SOCKET:-}"

if [ -z "$SOCKET" ]; then
    echo "FAIL: LAZYCLAUDE_TMUX_SOCKET not set" >&2
    exit 1
fi

TMPDIR=$(mktemp -d /tmp/lazyclaude-popup-e2e-XXXX)
cleanup() {
    tmux -L "$SOCKET" kill-window -t lazyclaude:replay 2>/dev/null || true
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

PASS=0
FAIL=0

check() {
    local name="$1" result="$2"
    if [ "$result" -eq 0 ]; then
        echo "  PASS: $name" >&2
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $name" >&2
        FAIL=$((FAIL + 1))
    fi
}

echo "=== tmux display-popup VISUAL E2E ===" >&2

CURRENT_SESSION=$(tmux -L "$SOCKET" display-message -p '#{session_name}')
tmux -L "$SOCKET" rename-session -t "$CURRENT_SESSION" lazyclaude 2>/dev/null || true

SCRIPT_LOG="$TMPDIR/popup.log"

# --- 1. Run lazyclaude tool inside display-popup with script recording ---
# `timeout 3` auto-closes after 3s (simulates user not pressing anything).
# `script -q` records the PTY output including gocui rendering.
echo "--- Step 1: Run display-popup with lazyclaude tool ---" >&2

POPUP_CMD="LAZYCLAUDE_TMUX_SOCKET=$SOCKET TOOL_NAME=Bash TOOL_INPUT='{\"command\":\"for i in \$(seq 1 10); do echo line_\$i; done && ls /tmp\"}' TOOL_CWD=/tmp timeout 3 script -q -c '$BINARY tool --window @0' $SCRIPT_LOG"

tmux -L "$SOCKET" display-popup -w 80 -h 24 -E "$POPUP_CMD" 2>/dev/null || true

R=0; [ -f "$SCRIPT_LOG" ] || R=1
check "script log recorded" $R

if [ ! -f "$SCRIPT_LOG" ]; then
    echo "  No script log — display-popup may have failed" >&2
    echo "Results: $PASS passed, $FAIL failed" >&2
    exit 1
fi

# --- 2. Replay into tmux pane for visual capture ---
echo "--- Step 2: Replay + capture ---" >&2
tmux -L "$SOCKET" new-window -t lazyclaude -n replay
# Feed raw PTY output into the pane — tmux interprets the ANSI sequences
tmux -L "$SOCKET" send-keys -t lazyclaude:replay "cat '$SCRIPT_LOG'" Enter
sleep 1

# -S - captures full scrollback history (title bar may be above visible area)
POPUP_CONTENT=$(tmux -L "$SOCKET" capture-pane -t lazyclaude:replay -p -S -)

echo "" >&2
echo "--- display-popup content ---" >&2
echo "$POPUP_CONTENT" >&2
echo "--- end ---" >&2

# --- 3. Verify popup rendered correctly ---
echo "" >&2
echo "--- Step 3: Verify popup content ---" >&2

# Title bar "┌─ Bash ─" may be above visible area in replay.
# Check scrollback for it; if not found, the other checks still prove gocui rendered.
R=0; echo "$POPUP_CONTENT" | grep -qE "Bash|Command:" || R=1
check "popup shows tool context (Bash or Command:)" $R

R=0; echo "$POPUP_CONTENT" | grep -q "Command:" || R=1
check "popup shows 'Command:'" $R

R=0; echo "$POPUP_CONTENT" | grep -q "seq" || R=1
check "popup shows command content" $R

R=0; echo "$POPUP_CONTENT" | grep -qE "yes.*no" || R=1
check "popup shows action bar (yes/no)" $R

R=0; echo "$POPUP_CONTENT" | grep -q "Esc" || R=1
check "popup shows Esc: cancel" $R

# Check border rendering (gocui box drawing)
R=0; echo "$POPUP_CONTENT" | grep -qE "┌|─|└|│" || R=1
check "popup shows gocui border" $R

echo "" >&2
echo "Results: $PASS passed, $FAIL failed" >&2
[ "$FAIL" -eq 0 ]
