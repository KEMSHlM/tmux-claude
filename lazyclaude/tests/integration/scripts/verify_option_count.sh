#!/bin/bash
# Verify popup action bar adapts to 2-option vs 3-option dialogs.
#
# Enqueues notifications with max_option=2 and max_option=3,
# checks that the TUI popup action bar shows y/n or y/a/n accordingly.
#
# PASS: all checks pass
# FAIL: any check fails

set -euo pipefail

BINARY="${1:-lazyclaude}"
UI_SOCKET="option-count-test"

cleanup() {
    tmux -L "$UI_SOCKET" kill-server 2>/dev/null || true
    tmux -L lazyclaude kill-server 2>/dev/null || true
    rm -f /tmp/lazyclaude-mcp.port
    rm -f "$HOME/.local/share/lazyclaude/state.json"
    rm -f /tmp/lazyclaude-q-*.json
}
trap cleanup EXIT

cleanup
sleep 0.5

capture() {
    tmux -L "$UI_SOCKET" capture-pane -p -t test 2>/dev/null
}

wait_for() {
    local pattern="$1" timeout="${2:-5}" i=0
    while [ $i -lt $((timeout * 10)) ]; do
        if capture | grep -q "$pattern"; then return 0; fi
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

echo "Option count test: $BINARY" >&2
echo "" >&2

tmux -L "$UI_SOCKET" new-session -d -s test -x 80 -y 30 "$BINARY; sleep 999"
wait_for "no sessions" 5 || { echo "FAIL: TUI did not start" >&2; exit 1; }

# --- Test 1: 2-option dialog (Bash, max_option=2) ---
echo "--- Test 1: 2-option dialog ---" >&2
TS=$(date +%s%N)
echo "{\"tool_name\":\"Bash\",\"input\":\"{\\\"command\\\":\\\"ls /tmp\\\"}\",\"window\":\"@0\",\"timestamp\":\"2026-01-01T00:00:01Z\",\"max_option\":2}" \
    > /tmp/lazyclaude-q-${TS}.json
sleep 1

wait_for "Bash" 3 || true

C=$(capture)
echo "$C" >&2

# Should show y/n, NOT y/a/n
R=0; echo "$C" | grep -q "y/a/n" && R=1
check "2-option: does NOT show y/a/n" $R

R=0; echo "$C" | grep -q "y/n" || R=1
check "2-option: shows y/n" $R

# Dismiss with 'y'
tmux -L "$UI_SOCKET" send-keys -t test y
sleep 0.3

# --- Test 2: 3-option dialog (Write, max_option=3) ---
echo "" >&2
echo "--- Test 2: 3-option dialog ---" >&2
TS=$(date +%s%N)
echo "{\"tool_name\":\"Write\",\"input\":\"{\\\"file_path\\\":\\\"/tmp/hello.txt\\\",\\\"content\\\":\\\"hello\\\"}\",\"window\":\"@0\",\"timestamp\":\"2026-01-01T00:00:02Z\",\"max_option\":3}" \
    > /tmp/lazyclaude-q-${TS}.json
sleep 1

wait_for "Write" 3 || true

C=$(capture)
echo "$C" >&2

# Should show y/a/n
R=0; echo "$C" | grep -q "y/a/n" || R=1
check "3-option: shows y/a/n" $R

# Dismiss
tmux -L "$UI_SOCKET" send-keys -t test y
sleep 0.3

# --- Test 3: default (max_option=0 or missing → treated as 3) ---
echo "" >&2
echo "--- Test 3: default (no max_option) ---" >&2
TS=$(date +%s%N)
echo "{\"tool_name\":\"Read\",\"input\":\"{\\\"file_path\\\":\\\"/tmp/test.txt\\\"}\",\"window\":\"@0\",\"timestamp\":\"2026-01-01T00:00:03Z\"}" \
    > /tmp/lazyclaude-q-${TS}.json
sleep 1

wait_for "Read" 3 || true

C=$(capture)
echo "$C" >&2

# Default should show y/a/n (backward compat)
R=0; echo "$C" | grep -q "y/a/n" || R=1
check "default: shows y/a/n (backward compat)" $R

tmux -L "$UI_SOCKET" send-keys -t test y
sleep 0.3

echo "" >&2
echo "Results: $PASS passed, $FAIL failed" >&2
[ "$FAIL" -eq 0 ]
