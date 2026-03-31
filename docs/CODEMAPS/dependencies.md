<!-- Generated: 2026-04-01 | Go 1.25 | Token estimate: ~500 -->

# Dependencies

**Last Updated:** 2026-04-01

## Go Module Dependencies

| Dependency | Purpose | Version |
|------------|---------|---------|
| `github.com/spf13/cobra` | CLI framework (commands, flags) | latest |
| `nhooyr.io/websocket` | WebSocket server (MCP protocol) | latest |
| `github.com/charmbracelet/x/ansi` | ANSI text width/truncation | latest |
| `github.com/google/uuid` | UUID generation (sessions) | latest |
| `golang.org/x/term` | Terminal control (isatty, raw mode) | latest |
| `github.com/stretchr/testify` | Test assertions & mocking | latest |
| `go.uber.org/goleak` | Goroutine leak detection (tests) | latest |
| `github.com/ActiveState/termtest` | Terminal testing | latest |

## Vendored Libraries (replace directives in go.mod)

| Local Path | Upstream | Key Modifications |
|------------|----------|-------------------|
| `third_party_gocui/` | `jesseduffield/gocui` | Paste aggregation, rawEvents pipeline, editor callbacks |
| `third_party_tcell/` | `gdamore/tcell/v2` | Minimal build files (see LAZYCLAUDE_PATCHES.md) |

## External Tools (runtime dependencies)

| Tool | Minimum Version | Usage |
|------|-----------------|-------|
| `tmux` | 3.4 | Session management, display-popup, control mode |
| `claude` | Latest | Claude Code CLI (plugins, MCP, sessions) |
| `git` | 2.25+ | Worktree operations, project root detection |
| `ssh` | OpenSSH | Remote session support (reverse tunnel setup) |
| `bash` | 4.0+ | Script execution for remote sessions |

## File-based Integration Points

| File | Purpose | Ownership |
|------|---------|-----------|
| `~/.claude/settings.json` | Hook installation target (--settings flag) | Claude Code |
| `~/.claude.json` | MCP server definitions (read-only) | Claude Code |
| `~/.claude/ide/<port>.lock` | IDE discovery lock file | Claude Code |
| `~/.local/share/lazyclaude/state.json` | Session persistence | lazyclaude |
| `~/.local/share/lazyclaude/port.file` | Server port + token | lazyclaude |
| `/tmp/lazyclaude-q-*.json` | Notification queue (SSH fallback) | lazyclaude |
| `/tmp/lazyclaude/server.log` | Server log (prefix: lazyclaude-srv:) | lazyclaude |

## Environment Variables

```
LAZYCLAUDE_TMUX_SOCKET    -- tmux socket name (default: "lazyclaude")
LAZYCLAUDE_DATA_DIR       -- override state directory (~/.local/share/lazyclaude)
LAZYCLAUDE_RUNTIME_DIR    -- override runtime directory (default: /tmp)
LAZYCLAUDE_IDE_DIR        -- override IDE lock directory (~/.claude/ide)
LAZYCLAUDE_HOST_TMUX      -- remote host tmux socket (SSH remote sessions)

LAZYCLAUDE_HOOK_PORT      -- Injected at hook time (server port discovery)
LAZYCLAUDE_HOOK_TOKEN     -- Injected at hook time (server auth token)
```

Variables injected via `--settings` flag during setup:
- `LAZYCLAUDE_HOOK_PORT` -- used by PreToolUse hook for /notify POST
- `LAZYCLAUDE_HOOK_TOKEN` -- used by PreToolUse hook for server auth

## Claude Code Hook Integration

Hooks installed in `~/.claude/settings.json` via `--settings` flag:

```
PreToolUse:
  1. Inject LAZYCLAUDE_* env vars
  2. POST /notify with tool_info (triggers permission prompt)
  3. Publish ActivityNotification (Running state)

UserPromptSubmit:
  1. Inject LAZYCLAUDE_* env vars
  2. POST /prompt-submit (triggers NeedsInput state)
  3. Publish ActivityNotification (NeedsInput state)
```

## Testing Infrastructure

| Tool | Purpose |
|------|---------|
| `go test` | Unit + integration tests (race detection, coverage) |
| `docker compose` | VHS E2E test environment |
| `vhs` | Terminal session recording/playback |
| `playwright` | Optional E2E test automation (e2e-runner agent) |

VHS test output: `vis_e2e_tests/outputs/{name}/` with `.gif`, `.txt`, `.log` files.
