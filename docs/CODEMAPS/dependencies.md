<!-- Generated: 2026-03-30 | Go 1.25 | Token estimate: ~500 -->

# Dependencies

## Go Module Dependencies

| Dependency | Purpose |
|------------|---------|
| `github.com/spf13/cobra` | CLI framework (commands, flags) |
| `nhooyr.io/websocket` | WebSocket server (MCP protocol) |
| `github.com/charmbracelet/x/ansi` | ANSI text width/truncation |
| `github.com/google/uuid` | UUID generation (sessions) |
| `golang.org/x/term` | Terminal control |
| `github.com/stretchr/testify` | Test assertions |
| `go.uber.org/goleak` | Goroutine leak detection (tests) |
| `github.com/ActiveState/termtest` | Terminal testing |

## Vendored Libraries (replace directives)

| Local Path | Upstream | Modifications |
|------------|----------|---------------|
| `third_party_gocui/` | `jesseduffield/gocui` | Paste aggregation, rawEvents pipeline |
| `third_party_tcell/` | `gdamore/tcell/v2` | Minimal build files (see LAZYCLAUDE_PATCHES.md) |

## External Tools (runtime)

| Tool | Usage |
|------|-------|
| `tmux` | Session management, display-popup, control mode |
| `claude` | Claude Code CLI (plugins, MCP, sessions) |
| `git` | Worktree operations, project root detection |
| `ssh` | Remote session support |

## File-based Integration Points

| File | Purpose |
|------|---------|
| `~/.claude/settings.json` | Hook installation target |
| `~/.claude.json` | MCP server definitions (read) |
| `~/.claude/ide/<port>.lock` | IDE discovery lock file |
| `~/.local/share/lazyclaude/state.json` | Session persistence |
| `~/.local/share/lazyclaude/port.file` | Server port + token |
| `/tmp/lazyclaude-q-*.json` | Notification queue (SSH fallback) |
| `/tmp/lazyclaude/server.log` | Server log |

## Environment Variables

```
LAZYCLAUDE_TMUX_SOCKET    -- tmux socket name (default: "lazyclaude")
LAZYCLAUDE_DATA_DIR       -- override state directory
LAZYCLAUDE_RUNTIME_DIR    -- override runtime directory (default: /tmp)
LAZYCLAUDE_IDE_DIR        -- override IDE lock directory
LAZYCLAUDE_HOST_TMUX      -- remote host tmux socket (SSH)
```
