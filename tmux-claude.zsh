# tmux-claude.zsh
# Source this file from ~/.zshrc to start the persistent MCP server.
#
# Usage (add to ~/.zshrc):
#   source ~/.local/share/tmux/plugins/tmux-claude/tmux-claude.zsh

_tmux_claude_mcp_start() {
  local plugin_dir="${0:A:h}"
  local script="${plugin_dir}/scripts/mcp-server.js"
  local pidfile="/tmp/tmux-claude-mcp.pid"

  # Check PID file first — atomic and race-safe
  if [[ -f "$pidfile" ]]; then
    local pid
    pid=$(<"$pidfile")
    if kill -0 "$pid" 2>/dev/null; then
      return  # already running
    fi
    rm -f "$pidfile"  # stale PID file
  fi

  [[ -f "$script" ]] || return

  node "$script" >> /tmp/tmux-claude-mcp.log 2>&1 &
  disown
}

_tmux_claude_mcp_start
