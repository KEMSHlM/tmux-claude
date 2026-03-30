<!-- Generated: 2026-03-30 | Files scanned: 86 src + 78 test | Lines: ~13,494 src | Token estimate: ~900 -->

# Architecture

## System Overview

Go TUI application for managing Claude Code sessions as a tmux plugin.
Two-tier tmux architecture with built-in MCP WebSocket/HTTP server.

## High-Level Data Flow

```
Claude Code (local/SSH)
    |
    +---> POST /notify (permission prompt)
    |         |
    +---> MCP Server (WebSocket + HTTP, random port)
    |     |-- event.Broker (in-process pub/sub)
    |     +-- notify queue (file-based, SSH fallback)
    |         |
    +---> TUI (gocui)
    |     |-- popup (permission prompt overlay)
    |     +-- user choice (1/2/3)
    |         |
    +---> tmuxadapter.SendToPane
    |         |
    +---> Claude Code (receives keystroke)
```

## Two-Tier Tmux

1. **User tmux** (default socket) -- `display-popup` shows lazyclaude TUI
2. **lazyclaude tmux** (`-L lazyclaude` socket) -- manages Claude Code session windows

## Package Dependency Graph

```
cmd/lazyclaude (CLI entry, Cobra)
  +-- gui           (TUI rendering, gocui)
  +-- session        (session CRUD, tmux sync, persistence)
  +-- server         (MCP WebSocket/HTTP server)
  +-- mcp            (MCP server config management)
  +-- plugin          (claude plugins CLI wrapper)
  +-- notify          (file-based notification queue)
  +-- adapter/tmuxadapter (sendkeys, detect)
  +-- core/
       +-- tmux      (tmux client interface, exec + control)
       +-- config    (paths, hooks)
       +-- event     (generic pub/sub broker)
       +-- lifecycle (LIFO cleanup)
       +-- model     (ToolNotification, Event)
       +-- choice    (Choice enum)
       +-- shell     (quoting)
```

## Key Design Patterns

- **Pub/Sub:** `core/event.Broker` for in-process notification dispatch
- **Adapter:** SessionAdapter, PluginAdapter, MCPAdapter bridge internal types to GUI
- **Interface-based testing:** Tmux client interface with mock implementation
- **LIFO Cleanup:** `core/lifecycle` for ordered resource teardown

## Entry Points

- `cmd/lazyclaude/main.go` -- Cobra root command
- `scripts/lazyclaude-launch.sh` -- tmux plugin entry (display-popup)
- `lazyclaude setup` -- install hooks + keybindings
- `lazyclaude server` -- start MCP daemon (manual)
