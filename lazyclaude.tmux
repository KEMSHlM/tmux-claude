#!/usr/bin/env bash
# lazyclaude TPM plugin entry point.
# 1. Runs `lazyclaude setup` (MCP server + Claude Code hooks)
# 2. Registers tmux keybindings from @claude-* options
#
# Configurable options (set in tmux.conf / plugins.conf):
#   @claude-launch-key    key to launch lazyclaude TUI (default: C-\)
#   @claude-suppress-keys space-separated keys to disable inside lazyclaude session

BINARY="$(command -v lazyclaude 2>/dev/null)"

if [ -z "$BINARY" ]; then
  echo "lazyclaude: binary not found in PATH" >&2
  exit 1
fi

# Capture the directory containing lazyclaude (and likely claude).
LAUNCH_BIN_DIR="$(dirname "$BINARY")"

# Run Go setup (MCP server + Claude Code hooks)
"$BINARY" setup

# Read tmux options
launch_key=$(tmux show-option -gqv @claude-launch-key 2>/dev/null)
suppress_keys=$(tmux show-option -gqv @claude-suppress-keys 2>/dev/null)

launch_key="${launch_key:-C-\\}"

# Register keybinding on user's tmux: launch-key opens popup.
# Popup bypasses tmux key processing, so this binding does NOT conflict
# with the lazyclaude server's detach binding for the same key.
POPUP_CMD="LAZYCLAUDE_HOST_TMUX=\$TMUX env -u TMUX PATH='$LAUNCH_BIN_DIR':\$PATH LAZYCLAUDE_POPUP_MODE=tmux $BINARY"
tmux bind-key -T root "$launch_key" display-popup -B -w 80% -h 80% -d "#{pane_current_path}" -E "$POPUP_CMD"

# Register detach binding on lazyclaude tmux server (same key).
# Only effective when a client is attached to the lazyclaude server.
tmux -L lazyclaude bind-key -T root "$launch_key" detach-client 2>/dev/null

# Suppress specified keys on the lazyclaude tmux server only.
for key in $suppress_keys; do
  tmux -L lazyclaude unbind-key -T prefix "$key" 2>/dev/null
done
