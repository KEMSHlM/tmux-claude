# Implementation Plan: Popup System Redesign (gocui-Only + Normal/Insert Mode)

## Overview

Full-screen mode renders Claude Code output in a gocui view using capture-pane.
Vim-like normal/insert mode controls whether keystrokes go to lazyclaude or
Claude Code. Tool/diff popups work identically in both preview and full-screen
modes as gocui overlays.

## Status — ALL PHASES COMPLETE

- Phase 1: DONE (full-screen rendering)
- Phase 2: DONE (InputForwarder)
- Phase 3: DONE (normal/insert mode)
- Phase 4: DONE (dead code removal)
- Phase 5: DONE (dead code cleanup: Binding, theme, options)
- Phase 6: DONE (control mode for event-driven refresh)

## Architecture Summary

### File Structure (internal/gui/)

| File | Lines | Purpose |
|------|-------|---------|
| app.go | ~230 | App struct, lifecycle, Run, accessors |
| mode.go | ~120 | InputMode, enter/exit full-screen, forwardKey |
| keymap.go | ~130 | KeyAction, KeyBinding, KeyMap, DefaultKeyMap |
| keybindings.go | ~380 | setupGlobalKeybindings using KeyMap |
| layout.go | ~320 | layoutMain, layoutFullScreen, layoutPopup, render* |
| input_editor.go | ~120 | Editor catch-all for insert/normal mode key dispatch |
| input_forwarder.go | ~65 | InputForwarder interface, TmuxInputForwarder, Mock |
| popup.go | ~210 | gocui overlay for tool/diff popups |

### Key Design Decisions

**Ctrl+\\ for mode switch** (not Esc or Ctrl+[):
- Ctrl+[ = Esc at terminal level (same byte 0x1B), verified via gocui/tcell
- Esc is used by Claude Code in 10+ contexts (chat:cancel, autocomplete, etc.)
- Ctrl+\\ (KeyCtrlBackslash) is a distinct byte, not used by Claude Code
- Source: https://code.claude.com/docs/en/keybindings

**Editor catch-all** (not per-key registration):
- gocui View.Editor.Edit() receives ALL unmatched keys when Editable=true
- Only KeyMap action keys (~15) are registered as gocui keybindings
- Editor forwards remaining keys to Claude Code (insert) or no-ops (normal)
- Eliminates 100+ for-loop keybinding registrations

**capture-pane limitation**:
- capture-pane returns text content only, no cursor/highlight/overlay
- tmux copy-mode cursor is invisible in capture-pane output
- Normal mode navigation requires alternative approach (see issue)
- Source: tmux issues #1949, #3787

**Rate-limited refresh**:
- triggerRefreshAfterInput: 50ms minimum between captures (insert mode)
- Prevents capture-per-keystroke during fast typing
- Control mode %output events for real-time updates when available

### Input Dispatch Flow

```
Key press
  ↓
gocui dispatch:
  1. View-specific bindings (popup view: y/a/n/1/2/3)
  2. Editor.Edit() (Editable=true in full-screen)
     - Insert mode: forward all to Claude Code
     - Normal mode: q exits, i returns to insert, rest no-op
  3. Global bindings (special keys: Ctrl+\, Ctrl+D, Ctrl+C, Esc, Enter, arrows)
```

## Current Normal Mode Capabilities

| Key | Action |
|-----|--------|
| q | Exit full-screen, return to split panel |
| i | Switch to insert mode |
| Ctrl+D | Exit full-screen (same as q) |
| Other keys | No-op (future: see issue-normal-mode-navigation.md) |

## Known Limitations

- Normal mode j/k/h/l navigation not implemented (capture-pane can't show cursor)
- Mouse scroll only works in insert mode
- Visual mode (V) planned but not started
- See: `docs/dev/issue-normal-mode-navigation.md`

## Testing

### Unit Tests (42 tests, headless gocui)
- [x] EnterFullScreen / ExitFullScreen toggle
- [x] layoutFullScreen creates correct views
- [x] MockInputForwarder records keys
- [x] forwardKey sends to correct target
- [x] Popup blocks forwarding
- [x] Default insert mode on enter
- [x] Ctrl+\\ switches to normal mode
- [x] i returns to insert mode
- [x] q exits full-screen
- [x] Mode preserved across popup show/dismiss
- [x] Normal mode keys are no-op
- [x] Insert mode blocks forwarding during popup
- [x] Ctrl+D exits from normal mode

### E2E Tests (4 tests, Docker + tmux)
- [x] TUI startup: shows session panel and options bar
- [x] Tool popup auto-display: notification file triggers popup overlay
- [x] Full-screen mode: Enter enters, Ctrl+D exits, INSERT shown
- [x] Normal mode: Ctrl+\\ switches to NORMAL, q exits to split panel

## Success Criteria — ALL MET

- [x] Full mode renders Claude Code in full-screen gocui view
- [x] Insert mode: all keys reach Claude Code prompt
- [x] Normal mode: q exits, i returns to insert
- [x] Mode indicator visible in status bar (INSERT/NORMAL)
- [x] Tool/diff popup identical in both modes
- [x] No tmux display-popup in codebase
- [x] No gocui Suspend/Resume in codebase
- [x] All tests pass (42 unit + 4 E2E)
- [x] Dead code removed (Binding, theme, options, copy-mode)
