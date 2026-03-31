<!-- Generated: 2026-04-01 | Files scanned: 154 src | Token estimate: ~850 -->

# Backend

**Last Updated:** 2026-04-01

## CLI Commands (Cobra)

```
lazyclaude           -- Interactive TUI (default), flags: --debug, --log-file
lazyclaude server    -- MCP daemon, flags: --port, --token
lazyclaude setup     -- Install hooks + keybindings (--settings flag)
lazyclaude diff      -- Diff viewer popup (internal use)
lazyclaude tool      -- Tool confirmation popup (internal use)
```

## MCP Server (internal/server/)

HTTP routes (mux in server.go):
```
POST   /notify               -> handleNotify              (permission prompt from hooks)
POST   /prompt-submit        -> handlePromptSubmit        (prompt entry detection)
POST   /msg/send             -> handleMsgSend             (send message to session)
GET    /msg/sessions         -> handleMsgSessions         (list active sessions)
POST   /msg/create           -> handleMsgCreate           (API-driven session spawning)
UPGRADE /                    -> WebSocket MCP              (JSON-RPC protocol)
```

WebSocket MCP methods (handler.go):
```
initialize                  -> protocol handshake (version + capabilities)
notifications/initialized   -> async ack (no response)
ide_connected               -> IDE connection event
openDiff                    -> diff viewer popup
```

Activity & Notification Events (server.go):
```
PreToolUse hook              -> publish ActivityNotification (Running state + tool name)
/prompt-submit endpoint      -> publish ActivityNotification (NeedsInput state)
Window death                 -> publish ActivityNotification (Dead state)
Idle detection               -> publish ActivityNotification (Idle state)
Tool error                   -> publish ActivityNotification (Error state)
```

Key files:
```
server/server.go       (485 lines) -- HTTP/WS server lifecycle + activity tracking
server/handler.go      (205 lines) -- MCP message dispatch
server/handler_msg.go  (290+ lines) -- message-specific handlers + /msg/create
server/state.go        (183 lines) -- connection/session state
server/lock.go         (182 lines) -- IDE lock file management
server/ensure.go       (169 lines) -- health checks + startup
server/jsonrpc.go      -- JSON-RPC protocol utilities
```

## Session Management (internal/session/)

```
manager.go     (667+ lines) -- CRUD, tmux sync, project grouping, role management
store.go       (518 lines) -- JSON persistence (~/.local/share/lazyclaude/state.json)
service.go     -- SessionService interface
project.go     -- project grouping by git root
role.go        -- PM/Worker role management + session creation
worktree.go    -- git worktree operations
ssh.go         -- SSH remote session setup
gc.go          -- orphan session garbage collector
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

config/hooks.go   (165+ lines) -- Claude Code hook installation + --settings flag
config/config.go  (76 lines)  -- path management (IDEDir, DataDir, RuntimeDir)

event/broker.go   (124 lines) -- generic pub/sub broker

lifecycle/lifecycle.go (83 lines) -- LIFO cleanup

model/notification.go   (140+ lines) -- ToolNotification, ActivityNotification, ActivityState enum
model/notification_test.go -- comprehensive notification + state tests

shell/quote.go    -- shell escaping
choice/choice.go  -- Choice enum (1/2/3)
```

## State Models (internal/core/model/)

ActivityState enum values:
```go
const (
    ActivityStateRunning   // Tool execution in progress
    ActivityStateNeedsInput // User prompt awaiting response
    ActivityStateIdle      // Idle, awaiting commands
    ActivityStateError     // Last operation errored
    ActivityStateDead      // Window terminated
)
```

Events published by server:
- `ActivityNotification` (ActivityState + tool name + window ID)
- `ToolNotification` (permission prompts)
- Generic `Event` for pubsub
