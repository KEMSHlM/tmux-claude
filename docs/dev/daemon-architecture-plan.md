# Implementation Plan: Remote Daemon Architecture

## Overview

Replace lazyclaude's SSH session management (script injection, base64 encoding, `_LC_WINDOW` hacks, PID-to-window resolution chains, and shell function injection) with a daemon-based architecture. A lazyclaude binary runs on the remote host, manages its own tmux sessions and hooks natively, and exposes a REST API over an SSH tunnel. The local TUI communicates with the remote daemon exclusively through this API.

## Architecture

```
+-- Local Machine -----------------+     +-- Remote Machine ----------------+
|                                  |     |                                  |
|  lazyclaude TUI                  |     |  lazyclaude daemon               |
|       |                          |     |       |                          |
|  tmux -L lazyclaude              | API |  tmux -L lazyclaude              |
|  +------+------+                 |<--->|  +------+------+                 |
|  |local |local |                 |     |  |remote|remote|                 |
|  |sess  |sess  |                 |     |  |sess  |sess  |                 |
|  +------+------+                 |     |  +------+------+                 |
|                                  |     |                                  |
+----------------------------------+     +----------------------------------+
```

Each machine has its own `tmux -L lazyclaude` server. The local TUI calls the remote daemon API. Session creation, hooks, git operations, prompt resolution all happen natively on the remote.

## Design Patterns

### Pattern 1: Composite Provider (Strategy + Composite)

```go
type CompositeProvider struct {
    local    *LocalProvider
    remotes  map[string]*RemoteProvider  // host -> provider
    router   *MessageRouter
}

// Sessions() merges all providers. TUI code unchanged.
func (c *CompositeProvider) Sessions() []SessionItem { ... }
```

TUI sees a single SessionProvider. Routing is internal.

### Pattern 2: Connection State Machine

```
                    connect()
 Disconnected -----------------> Connecting
      ^                               |
      |                          success|fail
      |                               |  |
      |  shutdown()              +----v--v----+
      +--------------------------|  Connected  |
      |                          +------+------+
      |                           tunnel drop
      |                                |
      |  max retries             +-----v------+
      +--------------------------|Reconnecting |
      |                          +------+------+
      |                            success|
 +----v---+                             |
 |  Error  |<------- max retries -------+
 +---------+
```

### Pattern 3: Message Router (Cross-Provider)

```go
func (r *MessageRouter) Send(from, to, body string) error {
    provider := r.findProviderForSession(to)
    return provider.DeliverMessage(from, to, body)
}
```

PM(local) -> Worker(remote) routing is transparent.

### Pattern 4: API Contract with Versioning

```go
const APIVersion = 1
type HealthResponse struct {
    Version  int    `json:"version"`
    Binary   string `json:"binary"`
    Uptime   int    `json:"uptime_s"`
    Sessions int    `json:"sessions"`
}
```

Connection-time version check. Mismatch prompts `lazyclaude deploy`.

### Pattern 5: Event Synchronization (SSE)

```
GET /notifications?since=<last-event-id>

// Daemon restart -> full_sync event -> all session states restored
type FullSyncEvent struct {
    Type     string        `json:"type"`  // "full_sync"
    Sessions []SessionInfo `json:"sessions"`
}
```

### Pattern 6: Graceful Degradation

Disconnected remote shows stale sessions with "(offline)" indicator. Operations return `DisconnectedError`. TUI displays reconnection status.

---

## Refactoring: Code Inventory

### Files to DELETE entirely (~1,500 lines)

| File | Lines | Contents | Reason |
|------|-------|----------|--------|
| `internal/session/ssh.go` | 249 | writeRemoteScript, buildSSHCommand, buildSSHCommandFromScript, RunSSHCommand, writeSSHLauncher, remoteScriptOpts, posixQuote, splitHostPort, lazyClaudeShellFunc, writeLazyClaude, BuildLazygitSSHArgs | Daemon handles all remote execution natively |
| `internal/session/ssh_test.go` | 347 | All tests for above | |
| `internal/session/script.go` | 313 | BuildScript, ScriptConfig, MCPConfig, writeMCPSetup, writeAuthEnv, buildClaudeCmd, lazyClaudeShellFunc, writeLazyClaude | Daemon uses Manager directly. Local worktrees use existing writeWorktreeLauncher |
| `internal/session/script_test.go` | 300 | All tests for above | |

### Files to SIMPLIFY (remove SSH branches)

#### `internal/session/manager.go` — remove ~150 lines of SSH logic

| Function | Current | After |
|----------|---------|-------|
| `Create(ctx, dirPath, host)` | host branch: buildSSHCommand, pendingWindowFile, hooksJSON | Remove `host` param. Local-only. daemon calls this on remote |
| `CreateWorktree(ctx, name, prompt, root, host)` | host in worktreeOpts, SSHRunner | Remove `host` param. daemon calls this on remote |
| `ResumeWorktree(ctx, path, prompt, root, host)` | Same | Remove `host` param |
| `CreatePMSession(ctx, root, host)` | SSH buildLaunchCommand branch, pendingWindowFile | Remove `host` param |
| `CreateWorkerSession(ctx, name, prompt, root, host)` | Same | Remove `host` param |
| `createWorktreeSession(ctx, opts)` | worktreeOpts.Host, NewGitRunner(host), SSH branch in buildLaunchCommand | Remove Host field from worktreeOpts |
| `launchWorktreeSession(...)` | host param, SSH branch | Remove host param. Always local |
| `buildLaunchCommand(...)` | SSH: BuildScript+writeSSHLauncher vs Local: temp script | Local path only. Delete SSH half |
| `writeSSHLauncher(...)` | Entire function (40 lines) | Delete |
| `launchSession(ctx, sess, cmd, dir, projectRoot, env)` | projectRoot param (added for SSH) | Keep projectRoot (useful for local worktrees too) |

#### `internal/session/gitcmd.go` — remove SSHRunner (~40 lines)

| Keep | Delete |
|------|--------|
| `GitRunner` interface | `SSHRunner` struct + methods |
| `LocalRunner` struct + methods | `NewGitRunner(host)` (always returns LocalRunner) |
| `CreateWorktreeWithRunner` | |
| `ListWorktreesWithRunner` | |

#### `internal/session/role.go` — remove remote file reading (~30 lines)

| Keep | Delete |
|------|--------|
| `resolvePrompt` (local fileReader) | `readRemoteFile` |
| `BuildPMPrompt(ctx, root, id, workers)` — remove host param | `remoteFileReader` |
| `BuildWorkerPrompt(ctx, path, root, id)` — remove host param | `promptFileReader` (simplify to always local) |
| `localFileReader` | `fileReader` type (no longer needed as abstraction) |

#### `internal/session/worktree.go` — remove host param

| Current | After |
|---------|-------|
| `ListWorktrees(ctx, root, host)` | `ListWorktrees(ctx, root)` — always local |

#### `internal/session/store.go` — keep host-aware grouping

| Keep | Reason |
|------|--------|
| `Add(sess, projectRoot)` | projectRoot override still useful for local worktrees |
| `findProjectIdxLocked(path, host)` | Projects still grouped by host (remote sessions have Host field) |
| `projectHost()` | Same |
| `Session.Host` field | Still needed for display and routing |

### Files to MODIFY in server (remove SSH hacks)

#### `internal/server/server.go` — remove ~60 lines

| Remove | Lines | Reason |
|--------|-------|--------|
| `notifyRequest.Window` field | 354 | Daemon uses native PID resolution |
| `stopRequest.Window` field | 573 | Same |
| `sessionStartRequest.Window` field | 626 | Same |
| `promptSubmitRequest.Window` field | 677 | Same |
| `req.Window` bypass in `handleNotify` | 404-412 | Same |
| `req.Window` bypass in `handleStop` | 594-597 | Same |
| `req.Window` bypass in `handleSessionStart` | 646-649 | Same |
| `req.Window` bypass in `handlePromptSubmit` | 697-700 | Same |
| `resolveNotifyWindow` pending-file fallback | 490-499 | No more pending-window files |
| `enrichWithActivity` SSH window-name fallback | 270-282 | Daemon reports activity via its own server |
| PID->window cache `hook-` prefix entries | Multiple | Native PID resolution on daemon |

#### `internal/server/handler.go` — remove pending-window-file (~10 lines)

| Remove | Reason |
|--------|--------|
| `pendingWindowFile` const | No longer needed |
| pending-file fallback in `handleIDEConnected` (lines 109-118) | Daemon's own server handles ide_connected |

#### `internal/core/config/hooks.go` — remove `_LC_WINDOW` (~5 lines)

| Remove | Reason |
|--------|--------|
| `windowJS` const | Daemon hooks don't need _LC_WINDOW |
| `_LC_WINDOW` injection in 5 hook commands | Same |
| `window` field in JSON body of all hooks | Same |

### Files to MODIFY in GUI

#### `internal/gui/app.go` — simplify SessionProvider interface

| Current | After |
|---------|-------|
| `Create(path, host string) error` | `Create(path string) error` |
| `LaunchLazygit(path, host string) error` | `LaunchLazygit(path string) error` |
| `CreateWorktree(name, prompt, root, host string) error` | `CreateWorktree(name, prompt, root string) error` |
| `ResumeWorktree(path, prompt, root, host string) error` | `ResumeWorktree(path, prompt, root string) error` |
| `ListWorktrees(root, host string) ([]WorktreeInfo, error)` | `ListWorktrees(root string) ([]WorktreeInfo, error)` |
| `CreatePMSession(root, host string) error` | `CreatePMSession(root string) error` |
| `CreateWorkerSession(name, prompt, root, host string) error` | `CreateWorkerSession(name, prompt, root string) error` |

Host routing is handled by CompositeProvider, not individual method params.

#### `internal/gui/app_actions.go` — remove `currentSessionHost()` calls

| Remove | Reason |
|--------|--------|
| `currentSessionHost()` helper | CompositeProvider handles routing |
| All `host := a.currentSessionHost()` calls | Same |
| Host passing to Create/Worktree/PM functions | Same |

#### `internal/gui/keybindings.go` — remove host capture

| Remove | Reason |
|--------|--------|
| `host := a.currentSessionHost()` in worktree Enter handler | Same |
| `host` param in worktree resume Enter handler | Same |

#### `cmd/lazyclaude/root.go` — restructure session adapter

| Current | After |
|---------|-------|
| `sessionAdapter` with host-forwarding methods | `LocalProvider` implementing simplified SessionProvider |
| `sessionCreatorAdapter` passing host="" | Same but no host param |
| `BuildLazygitSSHArgs` call | Delete (daemon handles lazygit natively) |

### Knowledge that carries forward

| From SSH implementation | Reused in daemon architecture |
|------------------------|-------------------------------|
| `DetectSSHHost()` / `DetectRemotePath()` (sshdetect.go) | TUI still detects SSH panes to auto-connect to daemon |
| `splitHostPort()` | SSH tunnel management in `daemon/tunnel.go` |
| `GitRunner` interface + `LocalRunner` | Daemon uses LocalRunner directly |
| `findProjectIdxLocked(path, host)` + `projectHost()` | Project grouping still host-aware |
| `SessionItem.Host` / `ProjectItem.Host` | Display and routing |
| Activity state machine (5-stage) | Same pipeline, daemon has native hooks |
| Hook node.js one-liners | Run natively on remote, no injection |
| `showError()` dual-pane display | UX improvement stays |
| `TextArea.Clear()` dialog fix | Bug fix stays |
| `posixQuote()` pattern | Rewritten in `daemon/tunnel.go` for SSH commands |

---

## Risks

### Connection / Network

| Risk | Severity | Mitigation |
|------|----------|------------|
| SSH tunnel silent disconnection | High | `/health` polling (5s). SSE disconnect detection. TUI status bar shows connection state |
| Operations lost during reconnection | High | Operations return `DisconnectedError`. TUI shows error. No silent queuing |
| Frequent tunnel drops (unstable network) | Medium | Exponential backoff. SSE `last-event-id` for event replay |

### Daemon Lifecycle

| Risk | Severity | Mitigation |
|------|----------|------------|
| Daemon crash | High | `remain-on-exit` preserves sessions. Local TUI detects crash via health check. Auto-restart via SSH. `full_sync` event on reconnect restores state |
| Daemon restart loses in-memory activityMap | High | Next hook event naturally restores state. `full_sync` SSE event sends current snapshot on reconnect |
| Local MCP server restart | Medium | Daemon detects upstream disconnect. Reconnects when tunnel re-established |
| Multiple daemon instances (port conflict) | Medium | Port file lock. Startup checks for existing daemon: connect or kill+restart |
| Daemon left running (forgotten) | Low | Idle timeout (30min). `lazyclaude deploy --cleanup` |

### State Management

| Risk | Severity | Mitigation |
|------|----------|------------|
| Local/remote session list desync | Medium | TUI fetches from API on every render cycle (cached with TTL). SSE for real-time updates |
| Cross-provider message routing failure | Medium | MessageRouter looks up session in all providers. Unknown session returns clear error |
| Worktree path inconsistency | Low | Daemon manages paths locally. TUI displays as-is from API |

### Deployment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Architecture mismatch (darwin/arm64 -> linux/amd64) | Medium | `uname -m` detection. Cross-compile matrix. `GOOS/GOARCH` build |
| API version mismatch | Medium | `/health` returns version. Connection-time check. Prompt `lazyclaude deploy` on mismatch |
| Remote lacks tmux/permissions | Medium | `deploy` pre-checks: `which tmux`, `mkdir -p` test, disk space |
| lazyclaude not in remote PATH | Low | Deploy to `~/.local/bin`. Daemon session startup verifies PATH |

### Security

| Risk | Severity | Mitigation |
|------|----------|------------|
| Daemon API unauthorized access | Medium | `127.0.0.1` bind + token auth. Tunnel-only access |
| Token in port file | Low | File permission `0600`. User-specific dir `/tmp/lazyclaude-$USER/` |
| SSH agent forwarding leak | Low | Tunnel SSH uses `-a` (disable agent forwarding) |

### Architecture (Framework)

| Risk | Severity | Mitigation |
|------|----------|------------|
| API design mistake | Critical | Phase 0 single Worker. Derive from existing SessionProvider 1:1. Review before Phase 1 starts |
| SSE over SSH tunnel reliability | Medium | WebSocket as alternative. Polling fallback. Reconnect with last-event-id |
| Multiple remote hosts simultaneously | Low (future) | CompositeProvider registry. Design for N hosts, implement for 1 initially |

---

## Phases

### Phase 0: Framework Design (CRITICAL PATH)

Must complete before all other phases. A single Worker defines the API contract and interfaces. **Types and interfaces only, no implementation.**

**0.1** `internal/daemon/api.go` — API types, versioning, event types
**0.2** `internal/daemon/connection.go` — ConnectionState, ConnectionManager interface
**0.3** `internal/session/composite_provider.go` — CompositeProvider (SessionProvider impl)
**0.4** `internal/daemon/router.go` — MessageRouter (cross-provider routing)
**0.5** `internal/daemon/client.go` — daemon.Client interface (HTTP + SSE + reconnect)

### Phase 1: Remote Daemon (after Phase 0)

**1.1** `internal/daemon/daemon.go` — HTTP server wrapping Manager/Server/Broker
**1.2** `cmd/lazyclaude/daemon_cmd.go` — `lazyclaude daemon` subcommand
**1.3** API handlers — thin wrappers around Manager methods
**1.4** SSE `/notifications` — event broker subscription + streaming

### Phase 2: SSH Tunnel + Deploy (after Phase 0, parallel with Phase 1)

**2.1** `internal/daemon/tunnel.go` — SSH local port forwarding + health check
**2.2** `cmd/lazyclaude/deploy_cmd.go` — binary deployment via scp
**2.3** `internal/daemon/lifecycle.go` — remote daemon start/stop/discover

### Phase 3: Local TUI Integration (after Phase 0, parallel with Phase 1 and 2)

**3.1** `internal/session/remote_provider.go` — RemoteSessionProvider (daemon.Client wrapper)
**3.2** Wire CompositeProvider in `root.go`
**3.3** Interactive attach/lazygit via SSH (bypasses API)

### Phase 4: Delete Old SSH Code (after Phase 1-3 verified)

**4.1** Delete: ssh.go, script.go, ssh_test.go, script_test.go
**4.2** Simplify: manager.go (remove all `host` params and SSH branches)
**4.3** Simplify: gitcmd.go (remove SSHRunner), role.go (remove remoteFileReader), worktree.go (remove host param)
**4.4** Simplify: server.go, handler.go (remove _LC_WINDOW, pending-window-file, Window fields)
**4.5** Simplify: hooks.go (remove windowJS from hook commands)
**4.6** Simplify: app.go SessionProvider interface (remove host params)
**4.7** Simplify: app_actions.go (remove currentSessionHost), keybindings.go (remove host capture)
**4.8** Simplify: root.go (remove host forwarding in adapters)

### Phase 5: Hardening (after Phase 4)

**5.1** Reconnection with exponential backoff + TUI status indicator
**5.2** Daemon auto-update (version mismatch detection)

## Parallel Worker Assignment

```
Worker A: Phase 0 (Framework) ---- MUST COMPLETE FIRST
Worker B: Phase 2.2 (Deploy) ----- independent, start immediately
Worker C: Phase 1 (Daemon) ------- after Phase 0
Worker D: Phase 2.1+2.3 (Tunnel) - after Phase 0
Worker E: Phase 3 (TUI) ---------- after Phase 0
Worker F: Phase 4 (Delete) ------- after C, D, E verified
Worker G: Phase 5 (Hardening) ---- after F
```

Critical path: **Phase 0 -> Phase 1 + Phase 3 -> Phase 4 -> Phase 5**

## Success Criteria

- [ ] `lazyclaude deploy user@host` installs binary on remote
- [ ] `lazyclaude daemon` runs and serves the API
- [ ] Session CRUD via TUI works through remote daemon
- [ ] Activity state flows from remote to local TUI in real-time (SSE)
- [ ] Message routing works between local PM and remote Workers
- [ ] Worktree/PM/Worker sessions work on remote
- [ ] Custom prompt resolution reads from remote filesystem natively
- [ ] All old SSH code (~1,500 lines) deleted
- [ ] SessionProvider interface has no `host` parameters
- [ ] manager.go has no `if host != ""` branches
- [ ] server.go has no `_LC_WINDOW` or pending-window-file logic
- [ ] All local-session tests pass unchanged
- [ ] 80%+ test coverage on new daemon/client/provider code
