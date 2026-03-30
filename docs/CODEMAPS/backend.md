<!-- Generated: 2026-03-30 | Files scanned: 86 src | Token estimate: ~800 -->

# Backend

## CLI Commands (Cobra)

```
lazyclaude           -- Interactive TUI (default), flags: --debug, --log-file
lazyclaude server    -- MCP daemon, flags: --port, --token
lazyclaude setup     -- Install hooks + keybindings
```

## MCP Server (internal/server/)

HTTP routes (mux in server.go):
```
POST   /notify        -> handleNotify     (permission prompt from hooks)
POST   /msg/send      -> handleMsgSend    (send message to session)
GET    /msg/sessions  -> handleMsgSessions (list active sessions)
UPGRADE /             -> WebSocket MCP     (JSON-RPC protocol)
```

WebSocket MCP methods (handler.go):
```
initialize                  -> protocol handshake (version + capabilities)
notifications/initialized   -> async ack (no response)
ide_connected               -> IDE connection event
openDiff                    -> diff viewer popup
```

Key files:
```
server/server.go       (435 lines) -- HTTP/WS server lifecycle
server/handler.go      (205 lines) -- MCP message dispatch
server/handler_msg.go  (274 lines) -- message-specific handlers
server/state.go        (183 lines) -- connection/session state
server/lock.go         (182 lines) -- IDE lock file management
server/ensure.go       (169 lines) -- health checks + startup
server/jsonrpc.go      -- JSON-RPC protocol utilities
```

## Session Management (internal/session/)

```
manager.go  (667 lines) -- CRUD, tmux sync, project grouping
store.go    (518 lines) -- JSON persistence (~/.local/share/lazyclaude/state.json)
service.go  -- SessionService interface
project.go  -- project grouping by git root
role.go     -- PM/Worker role management
worktree.go -- git worktree operations
ssh.go      -- SSH remote session setup
gc.go       -- orphan session garbage collector
```

## MCP Config (internal/mcp/)

```
config.go   (177 lines) -- read/write ~/.claude.json + project settings
manager.go  (140 lines) -- MCP server operations
cli.go      -- `claude mcp` CLI wrapper
model.go    -- data models
```

## Plugin Management (internal/plugin/)

```
cli.go      (197 lines) -- `claude plugins` subprocess wrapper
manager.go  (156 lines) -- plugin operations coordinator
model.go    -- data models
```

## Notification (internal/notify/)

```
notify.go   (100 lines) -- file-based queue (/tmp/lazyclaude-q-*.json)
```

## Core Libraries (internal/core/)

```
tmux/exec.go      (432 lines) -- tmux command execution
tmux/control.go   (380 lines) -- multiplexed control mode client
tmux/mock.go      (207 lines) -- mock for testing
config/hooks.go   (149 lines) -- Claude Code hook installation
config/config.go  (76 lines)  -- path management (IDEDir, DataDir, RuntimeDir)
event/broker.go   (124 lines) -- generic pub/sub broker
lifecycle/lifecycle.go (83 lines) -- LIFO cleanup
shell/quote.go    -- shell escaping
choice/choice.go  -- Choice enum (1/2/3)
model/             -- ToolNotification, Event types
```
