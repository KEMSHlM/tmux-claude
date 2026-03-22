# SSH Launch Refactor Plan

## Problem

SSH session creation is broken. The command passes through 4 quoting layers
(Go string → tmux new-window → local shell → SSH → remote shell) and every
escaping approach has failed.

The tmux plugin path and standalone path share almost no code, making debugging
twice as hard.

## Design Principles

1. **One entry point**: `scripts/lazyclaude-launch.sh` for both tmux plugin and standalone
2. **One job per layer**: Shell handles shell, Go handles logic
3. **Zero nested quoting**: Remote command is a plain file, never embedded in a string

## Architecture

```
lazyclaude.tmux                  standalone
      │                              │
      │ run-shell                    │ user runs directly
      │ (expands #{} formats)        │
      ▼                              ▼
scripts/lazyclaude-launch.sh ◄─── SAME SCRIPT
      │
      │ args: pane_cmd pane_pid pane_tty pane_path pane_current_path
      │ (tmux plugin passes args; standalone queries tmux)
      │
      ├─ export LAZYCLAUDE_PANE_*
      ├─ display-popup (tmux plugin) or direct exec (standalone)
      │
      ▼
bin/lazyclaude (Go binary)
      │
      │ n key → Create SSH session
      │
      ├─ writeRemoteScript() → /tmp/lazyclaude-ssh-{id}.sh (plain bash, no quoting)
      │
      └─ tmux new-window "ssh -t HOST bash -s < /tmp/lazyclaude-ssh-{id}.sh"
                          └── ONE quoting layer (local shell interprets <)
                              remote bash reads stdin raw
```

## Phase 1: lazyclaude.tmux + launcher unification

### lazyclaude.tmux

Keybinding does ONE thing: call the launcher with pane info as arguments.

```bash
tmux bind-key -T root "$launch_key" run-shell \
    "$LAUNCHER '#{pane_current_command}' '#{pane_pid}' '#{pane_tty}' '#{pane_path}' '#{pane_current_path}'"
```

### scripts/lazyclaude-launch.sh

```bash
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="${SCRIPT_DIR}/../bin/lazyclaude"

# tmux plugin: args provided by run-shell
# standalone:  no args, query tmux directly
if [ $# -ge 4 ]; then
    PANE_CMD="$1" PANE_PID="$2" PANE_TTY="$3" PANE_PATH="$4" PANE_CWD="${5:-.}"
else
    PANE_CMD=$(tmux display-message -p '#{pane_current_command}')
    PANE_PID=$(tmux display-message -p '#{pane_pid}')
    PANE_TTY=$(tmux display-message -p '#{pane_tty}')
    PANE_PATH=$(tmux display-message -p '#{pane_path}')
    PANE_CWD=$(tmux display-message -p '#{pane_current_path}')
fi

export LAZYCLAUDE_PANE_CMD="$PANE_CMD"
export LAZYCLAUDE_PANE_PID="$PANE_PID"
export LAZYCLAUDE_PANE_TTY="$PANE_TTY"
export LAZYCLAUDE_PANE_PATH="$PANE_PATH"

# tmux plugin mode: open popup
if [ -n "$LAZYCLAUDE_POPUP_MODE" ]; then
    exec "$BINARY" "$@"
fi

# standalone: open display-popup with same binary
LAZYCLAUDE_HOST_TMUX="$TMUX"
exec tmux display-popup -B -w 80% -h 80% -d "$PANE_CWD" \
    -E "LAZYCLAUDE_HOST_TMUX='$LAZYCLAUDE_HOST_TMUX' \
        LAZYCLAUDE_PANE_CMD='$PANE_CMD' \
        LAZYCLAUDE_PANE_PID='$PANE_PID' \
        LAZYCLAUDE_PANE_TTY='$PANE_TTY' \
        LAZYCLAUDE_PANE_PATH='$PANE_PATH' \
        LAZYCLAUDE_POPUP_MODE=tmux \
        env -u TMUX $BINARY"
```

## Phase 2: SSH command — script file approach

Replace `buildSSHCommand()` and `buildRemoteCommand()`.

### writeRemoteScript(sess, mcpPort, token) → filepath

Writes a plain bash script to `/tmp/lazyclaude-ssh-{id}.sh`:

```bash
#!/bin/bash
mkdir -p "$HOME/.claude/ide"
cat > "$HOME/.claude/ide/61544.lock" << 'LOCKEOF'
{"pid":0,"authToken":"xxx","transport":"ws"}
LOCKEOF
trap 'rm -f "$HOME/.claude/ide/61544.lock"' EXIT
cd "/home/user/project"
CLAUDE_CODE_AUTO_CONNECT_IDE=true exec claude
```

No `shell.Quote()`. No escaping. Plain bash. Heredoc for JSON.

### buildSSHCommand(sess, mcpPort, token) → string

```go
scriptPath := writeRemoteScript(sess, mcpPort, token)
// ONE quoting layer: local shell interprets <, rest is raw stdin
return fmt.Sprintf("exec ssh -t %s %s bash -s < %s", sshFlags, host, scriptPath)
```

### Cleanup

`Manager.Delete()` removes `/tmp/lazyclaude-ssh-{id}.sh`.

## Phase 3: Test each layer independently

1. **writeRemoteScript**: unit test — call function, read file, verify bash content
2. **buildSSHCommand**: unit test — verify format of returned string
3. **launcher script**: integration test — run with mock args, verify env vars set
4. **E2E**: Docker SSH test — full flow

## Risks

- **File timing**: tmux new-window spawns a shell that reads `< /tmp/script.sh`.
  The file exists before `new-window` is called, so this is safe.
- **Cleanup on crash**: `/tmp` files persist. Acceptable — they contain only
  the MCP auth token which is session-scoped.
- **stdin conflict**: `bash -s` reads script from stdin. SSH `-t` allocates PTY
  for interactive use. These are compatible — bash reads the script then
  becomes interactive. Verified: `ssh -t host bash -s < script.sh` works.
