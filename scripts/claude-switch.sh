#!/bin/zsh
# claude-switch.sh - claude session manager with preview and send keys

TMUX_BIN=$(command -v tmux)

if ! $TMUX_BIN has-session -t "claude" 2>/dev/null; then
  $TMUX_BIN display-message "No claude session running"
  exit 0
fi

CURRENT_WINDOW=$($TMUX_BIN display-message -p -t claude '#{window_name}' 2>/dev/null)

# --- Live preview refresh via fzf --listen ---

REFRESH_PID=
FZF_PORT=

_cleanup() {
  [ -n "$REFRESH_PID" ] && kill "$REFRESH_PID" 2>/dev/null
}
trap _cleanup EXIT INT TERM

FZF_PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("",0)); print(s.getsockname()[1]); s.close()' 2>/dev/null)

if [ -n "$FZF_PORT" ]; then
  # Send reload-preview every second; exits automatically when fzf stops listening
  (
    sleep 0.5
    while curl -s -XPOST "http://localhost:${FZF_PORT}" -d 'reload-preview' 2>/dev/null; do
      sleep 1
    done
  ) &
  REFRESH_PID=$!
fi

# --- fzf session list ---

SELECTED=$($TMUX_BIN list-windows -t claude -F "#{window_name}	#{pane_current_command}	#{pane_current_path}	#{pane_path}" | \
  while IFS=$'\t' read name cmd dirpath oscpath; do
    if [ "$cmd" = "ssh" ]; then
      remote_host=$(echo "$oscpath" | sed 's|file://\([^/]*\).*|\1|')
      remote_path=$(echo "$oscpath" | sed 's|file://[^/]*||')
      if [ -n "$remote_path" ]; then
        label="[$remote_host] $remote_path"
      else
        label="[remote] $name"
      fi
    else
      label="[local] $dirpath"
    fi
    [ "$name" = "$CURRENT_WINDOW" ] && marker="*" || marker=" "
    printf "claude:=%s\t%s %s\n" "$name" "$marker" "$label"
  done | \
  fzf \
    ${FZF_PORT:+--listen $FZF_PORT} \
    --delimiter='\t' \
    --with-nth=2 \
    --border rounded \
    --padding 1,2 \
    --header $'  Claude Sessions\n  Enter: open  1/2/3: send  ctrl-x: kill\n' \
    --header-first \
    --preview "$TMUX_BIN capture-pane -t {1} -p -e -S - 2>/dev/null | tail -50" \
    --preview-window 'right:60%:wrap:border-left' \
    --bind "1:execute-silent($TMUX_BIN send-keys -t {1} '1' Enter)" \
    --bind "2:execute-silent($TMUX_BIN send-keys -t {1} '2' Enter)" \
    --bind "3:execute-silent($TMUX_BIN send-keys -t {1} '3' Enter)" \
    --bind "ctrl-x:execute-silent($TMUX_BIN kill-window -t {1})+abort")

[ -z "$SELECTED" ] && exit 0

WINDOW=$(echo "$SELECTED" | cut -f1 | sed 's/claude:=//')
[ -z "$WINDOW" ] && exit 0

if [ "${CLAUDE_SWITCH_MODE:-}" = "select" ]; then
  echo "$WINDOW"
else
  exec env -u TMUX $TMUX_BIN attach-session -t "claude:=$WINDOW"
fi
