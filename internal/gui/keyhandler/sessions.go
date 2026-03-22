package keyhandler

import (
	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/jesseduffield/gocui"
)

// SessionsPanel handles keys for the sessions list (upper-left).
type SessionsPanel struct{}

func (p *SessionsPanel) Name() string  { return "sessions" }
func (p *SessionsPanel) Label() string { return "Sessions" }

func (p *SessionsPanel) HandleKey(ev KeyEvent, actions AppActions) HandlerResult {
	switch {
	case ev.Rune == 'j' || ev.Key == gocui.KeyArrowDown:
		actions.MoveCursorDown()
		return Handled
	case ev.Rune == 'k' || ev.Key == gocui.KeyArrowUp:
		actions.MoveCursorUp()
		return Handled
	case ev.Rune == 'n':
		actions.CreateSession()
		return Handled
	case ev.Rune == 'd':
		actions.DeleteSession()
		return Handled
	case ev.Key == gocui.KeyEnter && ev.Mod == gocui.ModAlt:
		actions.AttachSession()
		return Handled
	case ev.Key == gocui.KeyEnter:
		actions.EnterFullScreen()
		return Handled
	case ev.Rune == 'r':
		actions.EnterFullScreen()
		return Handled
	case ev.Rune == 'R':
		actions.StartRename()
		return Handled
	case ev.Rune == 'D':
		actions.PurgeOrphans()
		return Handled
	}
	return Unhandled
}

func (p *SessionsPanel) OptionsBar() string {
	return " " +
		presentation.StyledKey("n", "new") + "  " +
		presentation.StyledKey("d", "del") + "  " +
		presentation.StyledKey("enter", "full") + "  " +
		presentation.StyledKey("M-enter", "attach") + "  " +
		presentation.StyledKey("R", "rename") + "  " +
		presentation.StyledKey("q", "quit")
}

func (p *SessionsPanel) TabCount() int      { return 1 }
func (p *SessionsPanel) TabIndex() int      { return 0 }
func (p *SessionsPanel) TabLabels() []string { return []string{"Sessions"} }
