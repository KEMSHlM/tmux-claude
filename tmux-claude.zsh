# tmux-claude.zsh
# Source this file from ~/.zshrc to start the persistent MCP server.
#
# Usage (add to ~/.zshrc):
#   source ~/.local/share/tmux/plugins/tmux-claude/tmux-claude.zsh

_tmux_claude_mcp_start() {
  local plugin_dir="${0:A:h}"
  local script="${plugin_dir}/scripts/mcp-server.js"
  local pidfile="/tmp/tmux-claude-mcp.pid"

  # Already running?
  if [[ -f "$pidfile" ]] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
    return
  fi

  [[ -f "$script" ]] || return

  node "$script" >> /tmp/tmux-claude-mcp.log 2>&1 &
  disown
}

_tmux_claude_mcp_start
