# Issue: Normal Mode Navigation (scrollback + cursor)

## Fundamental Limitation: capture-pane Cannot Show Copy-Mode

`capture-pane` returns only the text content of a pane. It does NOT include:
- Copy-mode cursor position (highlighted character)
- Copy-mode selection (visual highlight)
- Copy-mode status line (`[0/100]`)
- Scroll position within copy-mode

These are all tmux overlays rendered by the terminal, not pane content.

**Evidence:**
- tmux man page: capture-pane "Capture the contents of a pane" — text only
- [tmux Issue #1949](https://github.com/tmux/tmux/issues/1949): maintainer added
  `copy_cursor_x`/`copy_cursor_y` format variables because capture-pane cannot
  provide cursor position
- [tmux Issue #3787](https://github.com/tmux/tmux/issues/3787): no way to
  correlate capture-pane output with copy-mode cursor position
- Verified in this project: capture-pane returns identical output before and
  after j/k navigation in copy-mode

## Failed Approach: tmux copy-mode Integration

Attempted: enter `tmux copy-mode` on normal mode switch, forward j/k via
`send-keys`, capture-pane to show updated view.

Result: send-keys succeeds, pane is in copy-mode (`#{pane_in_mode}=1`),
but capture-pane returns the same content regardless of cursor position.
User sees no visual change.

Also attempted: `capture-pane -S -` (full scrollback). Returns thousands of
lines per frame, making gocui rendering extremely slow.

## Current State

- Normal mode exists (Ctrl+\ to enter, i/q to exit)
- j/k/h/l are no-op in normal mode (keys forwarded but invisible)
- Mouse scroll works in insert mode (SetOrigin on captured content)
- copy-mode infrastructure exists but is visually non-functional

## Proposed Solutions

### Option A: On-demand scrollback with self-rendered cursor (recommended)

1. On normal mode entry: `capture-pane -S -1000` ONCE → cache full content
2. Render in gocui with self-managed cursor (SetCursor + Highlight)
3. j/k move our cursor within cached content (no tmux interaction)
4. On normal mode exit: discard cache, return to live capture
5. No per-frame subprocess calls in normal mode

Advantage: completely decoupled from tmux copy-mode. Full control over
cursor rendering. One-time capture cost.

### Option B: `copy_cursor_y` format variable

1. Enter tmux copy-mode
2. Forward j/k via send-keys
3. Read `#{copy_cursor_y}` via `display-message -p`
4. Use the cursor position to render our own highlight in gocui
5. capture-pane for content + display-message for position = 2 calls per frame

Advantage: uses tmux's copy-mode for actual navigation (search, etc.)
Disadvantage: 2 subprocess calls per keypress, cursor-only (no selection highlight)

### Option C: Virtual terminal emulator

Parse raw pane output through a Go VT100 library. Maintain our own
scrollback buffer. No capture-pane needed.

Advantage: complete control, fastest rendering
Disadvantage: complex implementation, must handle all ANSI sequences

## Related

- `docs/dev/popup-redesign-plan.md`
- `memory/project_visual_mode.md` (future V mode)
