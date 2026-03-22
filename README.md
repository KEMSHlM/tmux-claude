# lazyclaude

A standalone TUI for managing Claude Code sessions, with tmux plugin support, SSH remote sessions, and live permission prompt notifications via a built-in MCP server.

## Features

- Interactive session manager with live preview of Claude Code output
- Permission prompt popups with one-key approval (y/a/n) -- even from another tmux window
- Fullscreen mode with direct keyboard forwarding to Claude Code
- SSH remote sessions with automatic reverse tunnel for MCP notifications
- tmux plugin integration via `display-popup`

## Requirements

- Go 1.25+
- tmux >= 3.4 (for `display-popup -b rounded`)
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-code)

## Installation

### Build from source

```bash
git clone https://github.com/KEMSHlM/lazyclaude ~/.local/share/tmux/plugins/lazyclaude
cd ~/.local/share/tmux/plugins/lazyclaude
make install PREFIX=~/.local
```

### With [TPM](https://github.com/tmux-plugins/tpm)

```tmux
set -g @plugin 'KEMSHlM/lazyclaude'
```

Press `prefix + I` to install. The plugin runs `lazyclaude setup` automatically.

### Standalone (without tmux plugin)

```bash
lazyclaude
```

## Keybindings

### Sessions panel

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate sessions |
| `n` | Create new session |
| `d` | Delete session |
| `Enter` | Fullscreen mode |
| `a` | Attach (direct tmux attach) |
| `1` / `2` / `3` | Send key to pane (permission prompt) |
| `R` | Rename session |
| `D` | Purge orphan sessions |

### Logs panel

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll |
| `G` / `g` | Jump to end / start |
| `v` | Toggle visual select |
| `y` | Copy selection |

### Popup (permission prompt)

| Key | Action |
|-----|--------|
| `y` / `1` | Accept |
| `a` / `2` | Allow always |
| `n` / `3` | Reject |
| `Y` | Accept all |
| `j` / `k` | Scroll (diff view) |
| `Esc` | Hide popup |
| `Left` / `Right` | Switch between stacked popups |

### Fullscreen mode

| Key | Action |
|-----|--------|
| `Ctrl+\` / `Ctrl+D` | Exit fullscreen |
| All other keys | Forwarded to Claude Code |

### Global

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Cycle panel focus |
| `p` | Restore hidden popups |
| `q` / `Ctrl+C` | Quit |

## Architecture

### Two tmux servers

1. **User's tmux** (default socket) -- displays lazyclaude TUI via `display-popup`
2. **lazyclaude tmux** (`-L lazyclaude` socket) -- manages Claude Code session windows

### MCP server

lazyclaude runs a built-in MCP server (WebSocket + HTTP) that Claude Code auto-connects to:

- Listens on `127.0.0.1:<random-port>`
- Port file: `~/.local/share/lazyclaude/port.file`
- Lock file: `~/.claude/ide/<port>.lock`
- Receives permission prompt notifications via `POST /notify`

### Permission prompt detection

Claude Code hooks in `~/.claude/settings.json` detect `permission_prompt` notifications and POST to the MCP server. `lazyclaude setup` installs these hooks automatically.

### SSH remote sessions

When creating a session on an SSH host:

1. Generates a bash script with MCP lock file setup
2. Sets up a reverse tunnel (`-R port:127.0.0.1:port`) to the local MCP server
3. Launches Claude Code with `CLAUDE_CODE_AUTO_CONNECT_IDE=true`
4. Permission prompts on the remote host reach the local TUI through the tunnel

## CLI subcommands

| Command | Description |
|---------|-------------|
| `lazyclaude` | Run the interactive TUI |
| `lazyclaude setup` | Install MCP server and Claude Code hooks |
| `lazyclaude server` | Start MCP server daemon |
| `lazyclaude diff` | Diff viewer popup (used internally) |
| `lazyclaude tool` | Tool confirmation popup (used internally) |

## Configuration

### Environment variables

| Variable | Description |
|----------|-------------|
| `LAZYCLAUDE_DATA_DIR` | Override state directory (default: `~/.local/share/lazyclaude/`) |
| `LAZYCLAUDE_RUNTIME_DIR` | Override runtime directory (default: `/tmp/`) |
| `LAZYCLAUDE_IDE_DIR` | Override IDE lock directory (default: `~/.claude/ide/`) |

### Runtime files

| File | Contents |
|------|----------|
| `~/.local/share/lazyclaude/state.json` | Session persistence |
| `~/.local/share/lazyclaude/port.file` | MCP server port |
| `~/.claude/ide/<port>.lock` | Claude Code IDE discovery |
| `/tmp/lazyclaude/server.log` | MCP server log |

## Development

### Build

```bash
make build    # bin/lazyclaude
make lint     # golangci-lint
```

### Test

```bash
make test          # All tests (race + coverage)
make test-unit     # Unit tests only
make test-vhs TAPE=smoke  # VHS visual E2E (Docker required)
```

### VHS E2E tests

Requires Docker and a Claude Code OAuth token:

```bash
claude setup-token
echo "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-..." > vis_e2e_tests/.env
make test-vhs TAPE=notify_popup
```

Outputs: `vis_e2e_tests/outputs/{name}/` with `.gif`, `.txt`, `.log` files.
