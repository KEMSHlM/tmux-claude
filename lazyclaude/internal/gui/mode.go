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
	if !a.fullScreen || a.inputMode != ModeInsert || a.inputForwarder == nil || a.hasPopup() || a.sessions == nil {
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
// Also resets scroll to bottom so the user sees the latest output.
func (a *App) triggerRefreshAfterInput() {
	a.fullScreenScrollY = 0
	a.previewMu.Lock()
	if !a.previewBusy {
		a.previewTime = time.Time{}
	}
	a.previewMu.Unlock()
}

// cursorDown moves the normal-mode cursor down.
func (a *App) normalCursorDown() {
	a.fullScreenCursorY++
}

// cursorUp moves the normal-mode cursor up.
func (a *App) normalCursorUp() {
	if a.fullScreenCursorY > 0 {
		a.fullScreenCursorY--
	}
}

// cursorLeft moves the normal-mode cursor left.
func (a *App) normalCursorLeft() {
	if a.fullScreenCursorX > 0 {
		a.fullScreenCursorX--
	}
}

// cursorRight moves the normal-mode cursor right.
func (a *App) normalCursorRight() {
	a.fullScreenCursorX++
}

func (a *App) enterFullScreen(sessionID string) {
	a.fullScreen = true
	a.fullScreenTarget = sessionID
	a.inputMode = ModeInsert
	a.fullScreenScrollY = 0
	a.fullScreenCursorX = 0
	a.fullScreenCursorY = 0
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
	a.fullScreen = false
	a.fullScreenTarget = ""
	a.inputMode = ModeInsert
	a.previewCache = ""
}
