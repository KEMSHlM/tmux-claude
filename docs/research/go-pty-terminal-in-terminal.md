# Go PTY: Terminal-in-Terminal Rendering and Input Forwarding

## Summary

Building a "terminal viewer with input forwarding" in Go requires composing three
independent layers: a PTY manager (creates the subprocess with a pseudo-terminal),
a VT/ANSI parser (interprets escape sequences into a cell grid), and a TUI renderer
(draws the cell grid and routes keyboard input back). All three layers are available
as standalone Go libraries. Several real projects (micro editor, bubbleterm, vibemux)
demonstrate the complete pattern. The main unsolved problem for Japanese IME is that
TUI frameworks intercept input before it reaches the PTY, and IME composition bytes
(multi-byte sequences in-flight) are consumed or mangled at that layer.

---

## Layer 1: PTY Management

### creack/pty

The de-facto standard Go PTY library. Wraps the Unix openpty(3) system call.

**Core API:**

```go
import "github.com/creack/pty"

// Start a command connected to a new PTY; returns the master side
ptmx, err := pty.Start(cmd)

// Or with explicit size
ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})

// Resize after start
pty.Setsize(ptmx, &pty.Winsize{Rows: 30, Cols: 120})
```

**Canonical input forwarding pattern:**

```go
// Put the controlling terminal into raw mode so keystrokes are not
// processed by the local line discipline before forwarding.
oldState, _ := term.MakeRaw(int(os.Stdin.Fd()))
defer term.Restore(int(os.Stdin.Fd()), oldState)

// Read subprocess output
go io.Copy(os.Stdout, ptmx)

// Forward raw keystrokes to subprocess
io.Copy(ptmx, os.Stdin)
```

The critical detail: `term.MakeRaw` disables local echo, line buffering, and signal
generation on the controlling terminal. Without it, multi-byte sequences from IME
composition will be buffered by the kernel until newline.

**Platform support:** Linux, macOS, FreeBSD. Windows support via ConPTY is in a
draft PR (#155) and not yet merged (as of 2025).

**Source:** [github.com/creack/pty](https://github.com/creack/pty)

### arhat.dev/pty

Cross-platform alternative with Windows ConPTY support built in.
Less widely used than creack/pty but worth considering for Windows targets.

**Source:** [pkg.go.dev/arhat.dev/pty](https://pkg.go.dev/arhat.dev/pty)

---

## Layer 2: VT/ANSI Parsing (Terminal State Machine)

A PTY gives you a raw byte stream including ANSI escape sequences. To display
subprocess output inside a TUI pane, you must parse those sequences into a cell
grid (character + color + attribute per cell position).

### hinshun/vt10x (and forks)

The most widely used Go VT100 state machine. Influenced by st, rxvt, xterm.
Used internally by micro-editor/terminal.

```go
import "github.com/hinshun/vt10x"

state := &vt10x.State{}
vt := vt10x.New(state)

// Feed raw bytes from PTY into the parser
vt.Write(rawBytes)

// Read the cell grid
state.Lock()
for y := 0; y < state.Rows; y++ {
    for x := 0; x < state.Cols; x++ {
        cell := state.Cell(x, y)
        // cell.Char, cell.FG, cell.BG
    }
}
cx, cy := state.Cursor()
state.Unlock()
```

**Forks:**
- `github.com/ActiveState/vt10x` - extends with `NewVT10XConsole` multiplexer
- `github.com/micro-editor/terminal` - same codebase, used in micro editor

**Source:** [pkg.go.dev/github.com/hinshun/vt10x](https://pkg.go.dev/github.com/hinshun/vt10x)

### Azure/go-ansiterm

Microsoft's ANSI parser, used in Docker/Moby. Callback-based rather than cell-grid
based. Better suited for log rendering than interactive terminal emulation.

**Source:** [github.com/Azure/go-ansiterm](https://github.com/Azure/go-ansiterm)

### jaguilar/vt100 and vito/vt100

Simpler VT100 implementations. Less complete than vt10x but with cleaner APIs.
`vito/vt100` is used by Concourse CI for log rendering.

---

## Layer 3: TUI Integration

### bubbleterm (taigrr/bubbleterm)

The most complete "terminal-in-terminal" library for Go. Pairs a PTY with a
Bubble Tea component that renders the cell grid as ANSI strings.

**Architecture:**
- Wraps vt10x (or similar) for ANSI parsing
- Exposes a Bubble Tea `Model` implementing `Init / Update / View`
- `View()` returns the rendered terminal screen as a string for Bubble Tea to print
- Input forwarding via `SendInput(string)` or through `Update(tea.Msg)`

```go
// With a new subprocess
terminal, err := bubbleterm.NewWithCommand(80, 24, exec.Command("bash"))

// With existing pipes (attach to running process)
terminal, err := bubbleterm.NewWithPipes(80, 24, stdout, stdin)

// In your Bubble Tea model:
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    m.terminal, cmd = m.terminal.Update(msg)
    return m, cmd
}

func (m Model) View() string {
    return m.terminal.View()
}
```

**IME note:** No explicit IME handling documented. Uses standard Bubble Tea key
event routing, which processes input through tcell/termenv. Composition sequences
may be fragmented.

**Source:** [pkg.go.dev/github.com/taigrr/bubbleterm](https://pkg.go.dev/github.com/taigrr/bubbleterm)

### micro-editor/terminal

The terminal pane implementation inside the micro text editor. Uses `hinshun/vt10x`
for VT parsing and `creack/pty` for PTY management. One of the most complete
production implementations of an embedded terminal in a Go TUI.

**API:**

```go
import "github.com/micro-editor/terminal"

state := new(terminal.State)

// Start a command in a PTY
vt, ptyFile, err := terminal.Start(state, exec.Command("bash"))

// Or wrap an existing reader
vt, err := terminal.Create(state, reader)

// Parse loop (runs in goroutine)
for {
    err := vt.Parse()
    if err != nil { break }
    // state is now updated; render it
}

// Render: lock state, read cells
state.Lock()
ch := state.Changed()
if ch & terminal.ChangedScreen != 0 {
    for y := 0; y < rows; y++ {
        for x := 0; x < cols; x++ {
            cell := state.Cell(x, y)
        }
    }
}
state.Unlock()
```

The micro editor's `TermPane` in `internal/action/termpane.go` demonstrates the
full integration: PTY start, parse loop goroutine, gapbuffer-backed cell grid,
and keyboard event forwarding.

**Source:** [pkg.go.dev/github.com/micro-editor/terminal](https://pkg.go.dev/github.com/micro-editor/terminal)

---

## Real Projects Using the Full Stack

### vibemux (UgOrange/vibemux)

A multi-pane TUI for running parallel Claude Code agents. Uses:
- `creack/pty` for PTY management
- Bubble Tea + Lip Gloss for TUI
- Dual-mode input: control mode (TUI navigation) vs. terminal mode (F12 to toggle)
  where all keystrokes bypass the TUI and go directly to the focused PTY

Explicitly documents Chinese Pinyin IME compatibility, meaning it routes raw bytes
to the PTY in terminal mode rather than translating through Bubble Tea's key event
system.

**Source:** [github.com/UgOrange/vibemux](https://github.com/UgOrange/vibemux)

### micro editor (micro-editor/micro)

Full production terminal-in-terminal. The `:term` command opens a terminal split
pane backed by a PTY and rendered via vt10x. Used by thousands of developers.
The terminal pane handles resize (SIGWINCH forwarding), alternate screen, mouse
events, and keyboard pass-through.

**Source:** [github.com/micro-editor/micro](https://github.com/micro-editor/micro)

### gritty (viktomas/gritty)

A standalone terminal emulator in Go using the Gio GUI framework. Not a TUI
(uses GPU-rendered GUI), but the VT parsing pipeline is instructive. The controller
mediates between keyboard input encoding and PTY writes.

**Source:** [github.com/viktomas/gritty](https://github.com/viktomas/gritty)

---

## The IME Input Forwarding Problem

This is where all existing Go TUI libraries have a gap.

### Why IME breaks with standard TUI input handling

Standard TUI frameworks (gocui, Bubble Tea, tview) intercept input by putting
the terminal into raw mode and reading individual runes or key events. IME
composition works differently:

1. The user presses keys that produce intermediate (uncommitted) Unicode code
   points or escape sequences (e.g., Japanese kana pre-composition on macOS
   uses special key codes).
2. The IME commits a composed character by sending a separate event.
3. Some IMEs send escape sequences that TUI frameworks misinterpret as control keys.

### What raw PTY pass-through actually requires

For IME to work correctly when forwarding input to a subprocess PTY, the TUI
must NOT process input at all when the terminal pane is focused. Instead:

```
Host terminal -> raw bytes -> directly written to subprocess PTY master
                                       ↓
                              subprocess receives composed characters
```

This means the TUI framework must suspend its own input loop and hand the
controlling TTY directly to the PTY. This is essentially what terminal multiplexers
(tmux, zellij) do: they read raw bytes from the host terminal and forward them
unmodified to the active pane's PTY.

### The vibemux approach (F12 toggle)

When terminal mode is active, the Bubble Tea program stops intercepting keys and
routes raw bytes directly to the PTY write end. This works for IME because the
composition bytes are forwarded intact.

**Limitation:** The user must manually toggle modes. The TUI cannot simultaneously
render a terminal pane (which requires reading its PTY output) and forward raw
input (which requires bypassing TUI input processing) without explicit mode switching.

### tmux's approach (reference)

tmux reads from the host terminal in raw mode via a dedicated file descriptor,
maintains a separate event loop for its own key bindings (prefix key), and forwards
everything else byte-for-byte to the active pane's PTY. The prefix key is the only
point of interception; all other bytes including IME composition sequences pass through.

If lazyclaude needs to support IME in a Claude Code pane, the tmux model is the
correct architectural reference: lazyclaude already runs inside tmux, so it can
delegate the PTY pass-through entirely to tmux's own send-keys mechanism rather
than implementing it in Go.

---

## Recommended Stack for a "Terminal Viewer with Input Forwarding" in Go

| Requirement | Library | Notes |
|---|---|---|
| Start subprocess with PTY | `creack/pty` | Mature, Linux/macOS |
| Parse VT escape sequences | `hinshun/vt10x` or `micro-editor/terminal` | vt10x is the Go standard |
| Render cell grid in TUI | `bubbleterm` (Bubble Tea) or custom renderer | bubbleterm does both |
| Forward input (ASCII/UTF-8) | `io.Copy(ptmx, inputReader)` | Simple goroutine |
| Forward input (IME) | Mode-switch pattern (vibemux) | No seamless solution exists |
| Resize notification | `pty.Setsize` + SIGWINCH | Must implement separately |

---

## Key References

- [creack/pty](https://github.com/creack/pty) - PTY interface for Go
- [hinshun/vt10x](https://pkg.go.dev/github.com/hinshun/vt10x) - VT10x terminal emulation backend
- [micro-editor/terminal](https://pkg.go.dev/github.com/micro-editor/terminal) - Production VT+PTY package
- [taigrr/bubbleterm](https://pkg.go.dev/github.com/taigrr/bubbleterm) - Bubble Tea terminal emulator component
- [UgOrange/vibemux](https://github.com/UgOrange/vibemux) - Multi-pane PTY TUI with IME toggle
- [micro-editor/micro](https://github.com/micro-editor/micro) - Production Go editor with embedded `:term` pane
- [viktomas/gritty](https://github.com/viktomas/gritty) - Reference terminal emulator in Go (Gio GUI)
- [Azure/go-ansiterm](https://github.com/Azure/go-ansiterm) - Microsoft ANSI parser (callback-based)
- [Build A Simple Terminal Emulator In 100 Lines of Golang](https://ishuah.com/2021/03/10/build-a-terminal-emulator-in-100-lines-of-go/) - Tutorial with creack/pty + tcell
- [ActiveState/vt10x](https://github.com/ActiveState/vt10x) - vt10x fork with multiplexer support
