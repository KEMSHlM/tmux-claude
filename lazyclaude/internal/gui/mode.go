package gui

import (
	"fmt"

	"github.com/jesseduffield/gocui"
)

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
		return ""
	}
	return "lazyclaude:" + t
}

// forwardKey sends a rune key to the Claude Code pane in full-screen mode.
// Called synchronously from gocui event loop — tmux send-keys is fast (~5ms).
func (a *App) forwardKey(ch rune) {
	if target := a.resolveForwardTarget(); target != "" {
		if err := a.inputForwarder.ForwardKey(target, RuneToTmuxKey(ch)); err != nil {
			a.setStatusAsync(fmt.Sprintf("forward key: %v", err))
		}
	}
}

// forwardSpecialKey sends a named special key (Enter, Tab, Up, Down, etc.).
func (a *App) forwardSpecialKey(tmuxKey string) {
	if target := a.resolveForwardTarget(); target != "" {
		if err := a.inputForwarder.ForwardKey(target, tmuxKey); err != nil {
			a.setStatusAsync(fmt.Sprintf("forward key: %v", err))
		}
	}
}

func (a *App) setStatusAsync(msg string) {
	if a.gui == nil {
		return
	}
	a.gui.Update(func(g *gocui.Gui) error {
		a.setStatus(g, msg)
		return nil
	})
}

func (a *App) enterFullScreen(sessionID string) {
	a.fullScreen = true
	a.fullScreenTarget = sessionID
	a.inputMode = ModeInsert
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
