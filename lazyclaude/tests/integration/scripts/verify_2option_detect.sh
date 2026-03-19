#!/bin/bash
# Verify that lazyclaude popup detects 2-option dialog from real Claude Code.
#
# Flow:
# 1. Start lazyclaude TUI in Docker
# 2. Start Claude Code, send a Bash prompt
# 3. Claude shows 2-option permission dialog (Yes/No)
# 4. Hooks fire -> MCP server -> notification with max_option
# 5. TUI popup shows y/n (NOT y/a/n)
#
# FAIL if popup shows y/a/n (3-option) for a 2-option dialog
#
# Requires: CLAUDE_CODE_OAUTH_TOKEN

set -euo pipefail

BINARY="${1:-lazyclaude}"
OAUTH_TOKEN="${CLAUDE_CODE_OAUTH_TOKEN:-}"
UI_SOCKET="2opt-test"

if [ -z "$OAUTH_TOKEN" ]; then
    echo "SKIP: CLAUDE_CODE_OAUTH_TOKEN not set" >&2
    exit 0
fi

cleanup() {
    tmux -L "$UI_SOCKET" kill-server 2>/dev/null || true
    tmux -L lazyclaude kill-server 2>/dev/null || true
    rm -f /tmp/lazyclaude-mcp.port
    rm -f "$HOME/.local/share/lazyclaude/state.json"
    rm -f /tmp/lazyclaude-pending-window
    rm -f /tmp/lazyclaude-q-*.json
}
trap cleanup EXIT

cleanup
sleep 0.5

capture() {
    tmux -L "$UI_SOCKET" capture-pane -p -t test 2>/dev/null
}

wait_for() {
    local pattern="$1" timeout="${2:-10}" i=0
    while [ $i -lt $((timeout * 10)) ]; do
        if capture | grep -qE "$pattern"; then return 0; fi
        sleep 0.1
        i=$((i + 1))
    done
    return 1
}

PASS=0
FAIL=0

check() {
    local name="$1" result="$2"
    if [ "$result" -eq 0 ]; then
        echo "  PASS: $name" >&2
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $name" >&2
        echo "  --- capture ---" >&2
        capture >&2
        echo "  --- end ---" >&2
        FAIL=$((FAIL + 1))
    fi
}

echo "=== 2-option detect E2E ===" >&2

# 1. Start lazyclaude TUI
# Start TUI with LAZYCLAUDE_TMUX_SOCKET matching UI_SOCKET so MCP server
# can resolve PIDs in the same tmux server where Claude runs.
tmux -L "$UI_SOCKET" new-session -d -s test -x 120 -y 40 \
    "LAZYCLAUDE_TMUX_SOCKET=$UI_SOCKET $BINARY; sleep 999"
wait_for "no sessions" 10 || { echo "FAIL: TUI did not start" >&2; exit 1; }
check "TUI started" 0

# 2. Wait for MCP server
for i in $(seq 1 100); do [ -f /tmp/lazyclaude-mcp.port ] && break; sleep 0.1; done
[ -f /tmp/lazyclaude-mcp.port ] || { echo "FAIL: MCP port file" >&2; exit 1; }
MCP_PORT=$(cat /tmp/lazyclaude-mcp.port)
echo "  MCP port: $MCP_PORT" >&2

# 3. Read auth token
LOCK_FILE="$HOME/.claude/ide/${MCP_PORT}.lock"
for i in $(seq 1 50); do [ -f "$LOCK_FILE" ] && break; sleep 0.1; done
[ -f "$LOCK_FILE" ] || { echo "FAIL: lock file" >&2; exit 1; }
AUTH_TOKEN=$(node -e "console.log(JSON.parse(require('fs').readFileSync('$LOCK_FILE','utf8')).authToken)")
check "MCP server ready" 0

# 4. Start Claude in a new window within the same tmux session
rm -rf /tmp/2opt-test && mkdir -p /tmp/2opt-test
rm -rf ~/.claude/projects/ ~/.claude/statsig/ ~/.claude/todos/ 2>/dev/null || true
cat > ~/.claude.json <<'CJSON'
{"hasCompletedOnboarding":true,"numStartups":10,"projects":{"/tmp/2opt-test":{"hasTrustDialogAccepted":true,"allowedTools":[]}}}
CJSON

tmux -L "$UI_SOCKET" new-window -t test -n claude -c /tmp/2opt-test

# Get the window ID of the Claude window and write as pending-window
CLAUDE_WIN_ID=$(tmux -L "$UI_SOCKET" display-message -t test:claude -p '#{window_id}')
echo "$CLAUDE_WIN_ID" > /tmp/lazyclaude-pending-window
echo "  Claude window ID: $CLAUDE_WIN_ID" >&2

tmux -L "$UI_SOCKET" send-keys -t test:claude \
    "CLAUDE_CODE_OAUTH_TOKEN='$OAUTH_TOKEN' CLAUDE_CODE_AUTO_CONNECT_IDE=true claude" Enter

# Wait for Claude to initialize
sleep 10

# 6. Send a Bash prompt
tmux -L "$UI_SOCKET" send-keys -t test:claude \
    'Run this exact bash command: for i in $(seq 1 10); do echo "line $i"; done && ls /tmp && ps aux | head -5 && echo "done"' Enter

echo "  Waiting for Claude permission dialog..." >&2
sleep 15

# Show what Claude is displaying
echo "--- Claude pane ---" >&2
tmux -L "$UI_SOCKET" capture-pane -p -t test:claude >&2
echo "--- end Claude ---" >&2

# 7. Check lazyclaude TUI popup (switch back to first window)
echo "--- Step 7: Check TUI popup ---" >&2
# Switch to the first window to capture TUI
tmux -L "$UI_SOCKET" select-window -t test:0

# Wait for popup
wait_for "Bash|command" 10 || true

echo "--- TUI popup ---" >&2
C=$(capture)
echo "$C" >&2
echo "--- end TUI ---" >&2

# 8. Verify
R=0; echo "$C" | grep -qE "Bash|command" || R=1
check "popup appeared" $R

if [ $R -eq 0 ]; then
    # Check what Claude actually shows (2 or 3 options)
    CLAUDE_PANE=$(tmux -L "$UI_SOCKET" capture-pane -p -t test:claude 2>/dev/null)
    CLAUDE_OPTIONS=$(echo "$CLAUDE_PANE" | grep -cE '^\s*[❯>]?\s*[0-9]+[.)]' || true)
    echo "  Claude dialog options detected: $CLAUDE_OPTIONS" >&2

    if [ "$CLAUDE_OPTIONS" -le 2 ]; then
        # 2-option: y/a/n is WRONG
        R=0; echo "$C" | grep -q "y/a/n" && R=1
        check "2-option dialog: popup does NOT show y/a/n" $R
        R=0; echo "$C" | grep -q "y/n" || R=1
        check "2-option dialog: popup shows y/n" $R
    else
        # 3-option: y/a/n is CORRECT
        R=0; echo "$C" | grep -q "y/a/n" || R=1
        check "3-option dialog: popup shows y/a/n" $R
    fi
fi

echo "" >&2
echo "Results: $PASS passed, $FAIL failed" >&2
[ "$FAIL" -eq 0 ]
