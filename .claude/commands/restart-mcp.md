Restart the MCP server, preserving the current port so Claude Code reconnects automatically.

Run these two bash commands in order:

```bash
PREV_PORT="$(cat /tmp/tmux-claude-mcp.port 2>/dev/null)"; kill "$(cat /tmp/tmux-claude-mcp.pid 2>/dev/null)" 2>/dev/null; rm -f ~/.claude/ide/*.lock; sleep 0.5; TMUX_CLAUDE_PORT="${PREV_PORT:-0}" node /Users/kenshin/.local/share/tmux/plugins/tmux-claude/scripts/mcp-server.js &
```

Then verify it started on the same port:

```bash
sleep 1 && echo "PID: $(cat /tmp/tmux-claude-mcp.pid)" && echo "port: $(cat /tmp/tmux-claude-mcp.port)"
```

Claude Code will reconnect automatically within a few seconds since the port is preserved.