# Bracketed Paste Handling in Terminal TUI Applications

## Summary

Bracketed paste is a terminal protocol extension where the terminal wraps pasted text in escape
sequences (`ESC[200~` ... `ESC[201~`), allowing applications to distinguish pasted content from
typed keystrokes. The application opts in by writing `\e[?2004h` to stdout and must clean up with
`\e[?2004l` on exit. When implemented correctly, applications can buffer the entire paste before
acting on it, skipping per-character processing and intermediate redraws. Failure to handle this
causes large pastes to freeze TUI applications because each character goes through the full render
pipeline.

---

## Key Concepts

### Bracketed Paste Protocol

- **Enable sequence**: `\e[?2004h` (sent by the application to the terminal at startup)
- **Disable sequence**: `\e[?2004l` (sent on exit or when leaving an input field)
- **Paste start marker**: `\e[200~` (sent by the terminal before the pasted text)
- **Paste end marker**: `\e[201~` (sent by the terminal after the pasted text)

Without bracketed paste, the terminal sends pasted text as if it were typed—one character (or byte
sequence) at a time. The application has no way to distinguish a 10,000-character paste from rapid
typing.

### tmux as an Intermediary

tmux intercepts bracketed paste escape sequences rather than passing them raw to the PTY of the
active pane. tmux maintainer Nicholas Marriott confirmed this behavior: "it never just passes
something through unless explicitly asked to by the application." tmux tracks the mode flag
(`MODE_BRACKETPASTE`) independently per pane, similar to how it tracks bold and color state.

When a paste occurs:
1. The outer terminal sends `\e[200~` + content + `\e[201~` to tmux.
2. tmux checks if the currently active pane has bracketed paste mode enabled (`\e[?2004h` was sent
   by the inner application).
3. If so, tmux re-wraps the content with the markers before forwarding to the pane's PTY.
4. If not, tmux strips the markers and forwards only the raw content.

**Critical implication for `display-popup`**: A TUI running inside `display-popup` is just another
pane from tmux's perspective. If the inner application enables bracketed paste mode, tmux will
deliver the markers. If the inner application does not enable it (or uses `\e[?2004l`), the markers
are stripped and the application receives raw characters—triggering the freeze-on-large-paste
problem.

A common misconfiguration is using tmux pass-through escaping (`\ePtmux;\e...\e\\`), which
bypasses tmux's tracking and causes double-wrapping or missing markers.

---

## Neovim's Implementation

### Input Stack

Neovim does not use tcell. Its input stack is:

```
PTY bytes -> libuv (async I/O) -> libtermkey -> tinput_read_cb -> paste state machine
```

- **libuv** provides non-blocking async I/O via `RStream`. The `tinput_read_cb` callback is called
  when bytes are available.
- **libtermkey** parses raw bytes into structured key events (key name, modifiers, etc.). For
  bracketed paste, libtermkey passes through the raw bytes; Neovim's TUI layer detects the markers
  itself.
- Source files: `src/nvim/tui/tui.c`, `src/nvim/tui/input.c`

### Phase-Tracked State Machine

`tinput_read_cb` detects the paste boundary sequences and sets `input->paste` to one of three
phases:

| Phase | Value | Meaning |
|-------|-------|---------|
| Start | 1 | `\e[200~` detected; first data chunk |
| Continue | 2 | Key buffer filled; more data follows |
| End | 3 | `\e[201~` detected; paste complete |

The phase value is forwarded with each chunk via the RPC API call `nvim_paste(chunk, crlf, phase)`.

### Redraw Batching

Neovim maintains a 65,535-byte output buffer (`OUTBUF_SIZE = 0xffff`). Escape sequences for cursor
movement and cell updates accumulate in this buffer. During paste, the buffer fills and flushes
automatically but does not trigger a full redraw per character. Quoting the implementation
documentation: "paste data itself doesn't directly trigger rendering; instead, the accumulated
changes queue redraw operations that are processed after the paste completes." Neovim shows a
throbber (`...`) during long pastes rather than rendering each intermediate state.

### 10x Speedup: PR #4448

The 2018 paste redesign (PR #4448, "paste: redesign, fixes, 10x speedup") replaced an
autocmd-based approach with a direct callback through `vim.paste()`. The old approach fired
`PastePre`/`PastePost` autocmds for every chunk, causing excessive overhead. The new design:

- Introduced `nvim_paste()` API allowing UIs and clients to inject paste at a lower level, bypassing
  Vim's traditional insert-mode key machinery.
- Used the `phase` parameter to batch chunks; rendering is done "at intervals and when paste
  terminates," not per keystroke.
- Exposed `vim.paste` as an overridable Lua function, allowing users to customize paste behavior.

---

## tcell

tcell (used by gocui, tview, and other Go TUI libraries) implements bracketed paste with two event
types:

### Event Types

```go
// EventPaste marks the start (Start() == true) or end (Start() == false) of a bracketed paste.
// Between these events, regular EventKey events carry the pasted characters.
type EventPaste struct { ... }

// EventClipboard carries clipboard content in response to clipboard requests.
type EventClipboard struct { ... }
```

The event channel buffer size is **128 events** (`eventQ = make(chan Event, 128)`).

### Critical Issue: Per-Character Key Events

Unlike Neovim's nvim_paste() which receives the paste as a complete chunk via RPC, tcell emits
**individual `EventKey` events** for each character between `EventPaste(start)` and
`EventPaste(end)`. The application is responsible for detecting the paste boundary events and
buffering the intermediate key events itself.

This means that with a 10,000-character paste:
1. tcell emits `EventPaste{start=true}`
2. tcell emits 10,000 individual `EventKey` events
3. tcell emits `EventPaste{start=false}`

If the application processes each `EventKey` by re-rendering the full screen (as gocui does by
default), this causes 10,000 full renders — freeze behavior.

**The event channel buffer of 128** means that if the application's Update() loop cannot drain
events fast enough, the channel fills and the input goroutine blocks, which stalls further reads
from the PTY. This manifests as the terminal appearing to accept no more input.

### Opt-in Requirement

Applications must call `EnablePaste()` to receive `EventPaste` events. Without this, the terminal
still sends `\e[200~` / `\e[201~` if it has been enabled, but tcell does not emit `EventPaste`
events and the raw bytes appear as garbage characters in the input stream.

---

## bubbletea

bubbletea v2 introduced first-class paste support with dedicated message types:

```go
case tea.PasteMsg:
    // Contains pasted content as a single string: msg.Content
case tea.PasteStartMsg:
    // Paste started (rarely needed)
case tea.PasteEndMsg:
    // Paste ended (rarely needed)
```

In v1, paste arrived as `tea.KeyMsg` with a confusing `msg.Paste bool` flag. v2 consolidates the
entire paste content into a single `PasteMsg` message, meaning the application receives one message
for the entire paste rather than per-character events.

bubbletea does not document explicit render throttling during paste; the EVA (Elm Virtual DOM
approach) means the renderer diffs the new View() against the last, which naturally reduces output
for intermediate states if the model coalesces input before returning.

---

## tview

tview provides `app.EnablePaste(true)` which opts in to bracketed paste events. The library
documents that "paste events are typically only used to insert a block of text into an InputField
or a TextArea." tview sits on top of tcell, so the per-character EventKey issue applies unless
tview explicitly buffers between `EventPaste` start/end events internally.

---

## Comparison: Paste Handling Strategies

| Application | Input Library | Paste Delivery | Render During Paste |
|-------------|--------------|----------------|---------------------|
| Neovim | libtermkey + libuv | Chunked RPC (`nvim_paste`, phases 1/2/3) | Throttled; redraws at intervals + end |
| bubbletea v2 | own (x/input) | Single `PasteMsg` with full content | Normal; one Update+View cycle per paste |
| tcell-based (gocui, tview) | tcell | Per-char `EventKey` between `EventPaste` boundary events | Depends on app; default gocui redraws per key |
| bubbletea v1 | own | Per-char `KeyMsg{Paste: true}` | Depends on app |

---

## Root Cause Pattern: Paste Freeze in gocui

The freeze pattern in gocui-based applications follows from the above:

1. Application does not call `EnablePaste()` (or gocui does not propagate it to tcell).
2. Outer terminal sends `\e[200~` + 10,000 chars + `\e[201~`.
3. tmux (if present) checks if inner app has enabled mode; if not, strips markers, forwards raw.
4. tcell receives 10,000 raw `EventKey` events.
5. gocui processes each one synchronously: `Manager.Layout()` is called after every key.
6. Each `Layout()` call triggers a full screen redraw.
7. 10,000 screen redraws saturate the output buffer and stall the goroutine.

**The fix requires at least one of:**
- Enable bracketed paste mode (`\e[?2004h`) so markers reach the application.
- Detect `EventPaste` start/end events and suppress intermediate renders.
- Buffer pasted characters into a single string, then process as one event.

---

## Key References

- Conrad Irwin. "Bracketed Paste Mode." 2013. https://cirw.in/blog/bracketed-paste
- Neovim commit 59fb8f8: "tui: Add support bracketed paste."
  https://github.com/neovim/neovim/commit/59fb8f81723f88935c3b4c7a1c0df4d6db1c2ff8
- Neovim PR #4448: "paste: redesign, fixes, 10x speedup."
  https://github.com/neovim/neovim/pull/4448
- Neovim DeepWiki — Terminal UI and Input Processing.
  https://deepwiki.com/neovim/neovim/3.1-terminal-ui-and-input-processing
- tcell paste.go source.
  https://github.com/gdamore/tcell/blob/main/paste.go
- tcell issue #120: "Support for bracketed paste mode."
  https://github.com/gdamore/tcell/issues/120
- bubbletea v2 discussion #1374: "Bubble Tea v2: What's New."
  https://github.com/charmbracelet/bubbletea/discussions/1374
- tmux issue #280: "Bracketed Paste mode should be set independently per pane."
  https://github.com/tmux/tmux/issues/280
- tmux commit f4fdddc: "Support bracketed paste mode."
  https://github.com/tmux/tmux/commit/f4fdddc9306886e3ab5257f40003f6db83ac926b
- jdhao. "Bracketed Paste Mode in Terminal." 2021.
  https://jdhao.github.io/2021/02/01/bracketed_paste_mode/
