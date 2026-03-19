#!/bin/bash
# Verify `lazyclaude setup` registers hooks and starts MCP server.
#
# PASS: all checks pass
# FAIL: any check fails

set -euo pipefail

BINARY="${1:-lazyclaude}"
SOCKET="setup-test"

cleanup() {
    tmux -L "$SOCKET" kill-server 2>/dev/null || true
    rm -f /tmp/lazyclaude-mcp.port
    rm -f "$HOME/.local/share/lazyclaude/state.json"
    rm -f "$HOME/.claude/settings.json.bak"
}
trap cleanup EXIT

cleanup
sleep 0.3

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

echo "=== lazyclaude setup test ===" >&2

# Backup existing settings
[ -f "$HOME/.claude/settings.json" ] && cp "$HOME/.claude/settings.json" "$HOME/.claude/settings.json.bak"

# Remove existing hooks to test fresh install
rm -f "$HOME/.claude/settings.json"

# Need a tmux session for setup to work
tmux -L "$SOCKET" new-session -d -s test -x 80 -y 24

# Run setup
LAZYCLAUDE_TMUX_SOCKET="$SOCKET" "$BINARY" setup 2>&1

# 1. MCP server port file should exist (server starts async, wait)
for i in $(seq 1 30); do [ -f /tmp/lazyclaude-mcp.port ] && break; sleep 0.1; done
R=0; [ -f /tmp/lazyclaude-mcp.port ] || R=1
check "MCP port file exists" $R

if [ $R -eq 0 ]; then
    PORT=$(cat /tmp/lazyclaude-mcp.port)
    echo "  MCP port: $PORT" >&2
fi

# 2. Claude settings.json should have hooks
R=0; [ -f "$HOME/.claude/settings.json" ] || R=1
check "settings.json exists" $R

if [ $R -eq 0 ]; then
    R=0; grep -q "/notify" "$HOME/.claude/settings.json" || R=1
    check "settings.json contains /notify hook" $R

    R=0; grep -q "PreToolUse" "$HOME/.claude/settings.json" || R=1
    check "settings.json contains PreToolUse" $R

    R=0; grep -q "Notification" "$HOME/.claude/settings.json" || R=1
    check "settings.json contains Notification" $R
fi

# 3. Running setup again should be idempotent (no error)
R=0; LAZYCLAUDE_TMUX_SOCKET="$SOCKET" "$BINARY" setup 2>&1 || R=1
check "setup idempotent (no error on re-run)" $R

# Restore backup
[ -f "$HOME/.claude/settings.json.bak" ] && mv "$HOME/.claude/settings.json.bak" "$HOME/.claude/settings.json"

echo "" >&2
echo "Results: $PASS passed, $FAIL failed" >&2
[ "$FAIL" -eq 0 ]
