#!/bin/zsh
# claude-popup.sh - popup wrapper for claude windows
# Runs attach-session in a loop; when prefix+O sets the flag, shows switcher inline

WINDOW="${1:-}"
TMUX_BIN=$(command -v tmux)
SCRIPT_DIR="${0:A:h}"
FLAG_FILE="/tmp/claude-popup-switch"

while true; do
  # Get window ID before attaching — used to detect if the window is destroyed
  WINDOW_ID=$($TMUX_BIN display-message -p -t "claude:=$WINDOW" '#{window_id}' 2>/dev/null)
  [ -z "$WINDOW_ID" ] && break

  # Monitor in background: if the window is destroyed while attached,
  # detach-client so attach-session exits (instead of switching to another window)
  (
    while $TMUX_BIN list-windows -t claude -F '#{window_id}' 2>/dev/null | grep -q "^${WINDOW_ID}$"; do
      sleep 0.3
    done
    $TMUX_BIN detach-client 2>/dev/null
  ) &
  MONITOR_PID=$!

  env -u TMUX $TMUX_BIN attach-session -t "claude:=$WINDOW"

  kill $MONITOR_PID 2>/dev/null
  wait $MONITOR_PID 2>/dev/null

  # attach-session exited — check if switcher was requested
  if [ -f "$FLAG_FILE" ]; then
    rm -f "$FLAG_FILE"
    NEW_WINDOW=$(CLAUDE_SWITCH_MODE=select "$SCRIPT_DIR/claude-switch.sh")
    [ -n "$NEW_WINDOW" ] && WINDOW="$NEW_WINDOW" && continue
  fi
  break
done
