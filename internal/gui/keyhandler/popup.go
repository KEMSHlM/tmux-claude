package keyhandler

import (
	"github.com/KEMSHlM/lazyclaude/internal/core/choice"
	"github.com/jesseduffield/gocui"
)

// PopupHandler handles keys when a popup is visible.
// Highest priority: consumes ALL keys to prevent leaking to panels.
type PopupHandler struct{}

func (h *PopupHandler) HandleKey(ev KeyEvent, actions AppActions) HandlerResult {
	if !actions.HasPopup() {
		return Unhandled
	}

	switch {
	case ev.Rune == 'y' || ev.Rune == '1':
		actions.DismissPopup(choice.Accept)
	case ev.Rune == 'a' || ev.Rune == '2':
		actions.DismissPopup(choice.Allow)
	case ev.Rune == 'n' || ev.Rune == '3':
		actions.DismissPopup(choice.Reject)
	case ev.Rune == 'Y':
		actions.DismissAllPopups(choice.Accept)
	case ev.Key == gocui.KeyEsc:
		actions.SuspendPopups()
	case ev.Key == gocui.KeyArrowDown:
		actions.PopupFocusNext()
	case ev.Key == gocui.KeyArrowUp:
		actions.PopupFocusPrev()
	case ev.Rune == 'j':
		actions.PopupScrollDown()
	case ev.Rune == 'k':
		actions.PopupScrollUp()
	}

	// Consume ALL keys when popup is visible — prevent leaking to panels.
	return Handled
}
