package gui

import "time"

// InputMode controls key handling in full-screen mode (vim-like).
type InputMode int

const (
	ModeInsert InputMode = iota // all keys forwarded to Claude Code
	ModeNormal                  // lazyclaude handles keys (scroll, quit, popup)
)

// resolveForwardTarget returns the tmux target for key forwarding.
// Returns empty string if forwarding should be skipped.
func (a *App) resolveForwardTarget() string {
	if !a.fullScreen || a.inputForwarder == nil || a.hasPopup() || a.sessions == nil {
		return ""
	}
	items := a.sessions.Sessions()
	if a.cursor < 0 || a.cursor >= len(items) {
		return ""
	}
	t := items[a.cursor].TmuxWindow
	if t == "" {
		// TmuxWindow not yet synced (between Create and first GC Sync).
		// Construct name-based target from session ID as fallback.
		id := items[a.cursor].ID
		if id == "" {
			return ""
		}
		windowName := "lc-" + id
		if len(id) > 8 {
			windowName = "lc-" + id[:8]
		}
		return "lazyclaude:" + windowName
	}
	return "lazyclaude:" + t
}

// forwardKey sends a rune key to the Claude Code pane.
func (a *App) forwardKey(ch rune) {
	target := a.resolveForwardTarget()
	if target == "" {
		return
	}
	a.inputForwarder.ForwardKey(target, RuneToTmuxKey(ch))
	a.triggerRefreshAfterInput()
}

func (a *App) forwardSpecialKey(tmuxKey string) {
	target := a.resolveForwardTarget()
	if target == "" {
		return
	}
	a.inputForwarder.ForwardKey(target, tmuxKey)
	a.triggerRefreshAfterInput()
}

// triggerRefreshAfterInput marks preview as stale after sending a key.
// Rate-limited: only marks stale if last capture was >50ms ago.
// This prevents capture-per-keystroke during fast typing while still
// providing responsive display updates.
func (a *App) triggerRefreshAfterInput() {
	if a.inputMode != ModeInsert {
		return
	}
	a.fullScreenScrollY = 0
	a.previewMu.Lock()
	if !a.previewBusy && time.Since(a.previewTime) > 50*time.Millisecond {
		a.previewTime = time.Time{}
	}
	a.previewMu.Unlock()
}


// setInputMode switches between insert and normal mode.
// Normal mode enters tmux copy-mode; insert mode exits it.
func (a *App) setInputMode(mode InputMode) {
	if a.inputMode == mode {
		return
	}
	a.inputMode = mode

	if a.sessions == nil || !a.fullScreen {
		return
	}
	items := a.sessions.Sessions()
	if a.cursor < 0 || a.cursor >= len(items) {
		return
	}
	id := items[a.cursor].ID
	_ = a.sessions.SetCopyMode(id, mode == ModeNormal) // best-effort
}

func (a *App) enterFullScreen(sessionID string) {
	a.fullScreen = true
	a.fullScreenTarget = sessionID
	a.inputMode = ModeInsert
	a.fullScreenScrollY = 0
	a.previewCache = ""
	// Set cursor to the target session once at entry (not in layout)
	if a.sessions != nil {
		for i, item := range a.sessions.Sessions() {
			if item.ID == sessionID {
				a.cursor = i
				break
			}
		}
	}
}

func (a *App) exitFullScreen() {
	if a.inputMode == ModeNormal {
		a.setInputMode(ModeInsert) // exits copy-mode
	}
	a.fullScreen = false
	a.fullScreenTarget = ""
	a.previewCache = ""
}
