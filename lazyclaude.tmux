#!/usr/bin/env bash
# lazyclaude TPM plugin entry point.
# 1. Finds the lazyclaude binary
# 2. Runs `lazyclaude setup` (MCP server + Claude Code hooks)
# 3. Registers tmux keybindings from @claude-* options
#
# Configurable options (set in tmux.conf / plugins.conf):
#   @claude-launch-key    key to launch lazyclaude TUI (default: space)
#   @claude-suppress-keys space-separated keys to disable inside lazyclaude session

CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
BINARY="${CURRENT_DIR}/bin/lazyclaude"

if [ ! -x "$BINARY" ]; then
    BINARY="$(command -v lazyclaude 2>/dev/null)"
fi

if [ -z "$BINARY" ]; then
    echo "lazyclaude: binary not found in ${CURRENT_DIR}/bin/ or PATH" >&2
    exit 1
fi

# Run Go setup (MCP server + Claude Code hooks)
"$BINARY" setup

# Read tmux options
launch_key=$(tmux show-option -gqv @claude-launch-key 2>/dev/null)
suppress_keys=$(tmux show-option -gqv @claude-suppress-keys 2>/dev/null)

launch_key="${launch_key:-space}"

# Register keybindings
# Use new-window so the TUI gets a proper terminal
tmux bind-key "$launch_key" new-window -n lazyclaude "$BINARY"

# Suppress specified keys inside lazyclaude session
for key in $suppress_keys; do
    original=$(tmux list-keys -T prefix 2>/dev/null | awk -v k="$key" '$4 == k {$1=$2=$3=$4=""; sub(/^[[:space:]]+/,""); print}')
    if [ -n "$original" ]; then
        tmux bind-key "$key" if -F '#{==:#{session_name},lazyclaude}' '' "$original"
    else
        tmux bind-key "$key" if -F '#{==:#{session_name},lazyclaude}' '' ''
    fi
done
