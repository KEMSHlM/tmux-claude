#!/bin/bash
# Runner for verify_tmux_popup.sh — starts tmux via `script` (PTY required for
# display-popup) and captures clean output.

set -euo pipefail

BINARY="${1:-lazyclaude}"
SOCKET="popup-e2e-runner"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
RESULT_FILE="/tmp/popup-e2e-result-$$"
LOG_FILE="/tmp/popup-e2e-log-$$"

cleanup() {
    tmux -L "$SOCKET" kill-server 2>/dev/null || true
    rm -f "$RESULT_FILE" "$LOG_FILE"
}
trap cleanup EXIT
cleanup

# Run the test inside tmux via script (allocates PTY for display-popup).
# Redirect test stderr to LOG_FILE for clean output.
script -q -c "
    tmux -L $SOCKET new-session -d -s runner -x 120 -y 40
    tmux -L $SOCKET send-keys -t runner \
        'LAZYCLAUDE_TMUX_SOCKET=$SOCKET bash $SCRIPT_DIR/verify_tmux_popup.sh $BINARY 2>$LOG_FILE; echo RESULT=\$? > $RESULT_FILE; tmux -L $SOCKET kill-server' Enter
    tmux -L $SOCKET attach -t runner
" /dev/null > /dev/null 2>&1 || true

# Print clean test output
if [ -f "$LOG_FILE" ]; then
    cat "$LOG_FILE" >&2
fi

# Exit with test result
if [ -f "$RESULT_FILE" ]; then
    EXIT_CODE=$(grep -oP 'RESULT=\K\d+' "$RESULT_FILE" 2>/dev/null || echo "1")
    exit "$EXIT_CODE"
else
    echo "FAIL: test did not produce result file" >&2
    exit 1
fi
