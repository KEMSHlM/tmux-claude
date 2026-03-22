package keyhandler

import "github.com/jesseduffield/gocui"

// FullScreenHandler handles special keys in full-screen mode.
// Rune keys are NOT handled here — inputEditor.Edit() handles those.
type FullScreenHandler struct{}

func (h *FullScreenHandler) HandleKey(ev KeyEvent, actions AppActions) HandlerResult {
	if !actions.IsFullScreen() {
		return Unhandled
	}

	switch {
	case ev.Key == gocui.KeyCtrlBackslash:
		actions.ExitFullScreen()
		return Handled
	case ev.Key == gocui.KeyCtrlD:
		actions.ExitFullScreen()
		return Handled
	case ev.Key == gocui.KeyEnter:
		actions.ForwardSpecialKey("Enter")
		return Handled
	case ev.Key == gocui.KeyEsc:
		actions.ForwardSpecialKey("Escape")
		return Handled
	case ev.Key == gocui.KeyArrowDown:
		actions.ForwardSpecialKey("Down")
		return Handled
	case ev.Key == gocui.KeyArrowUp:
		actions.ForwardSpecialKey("Up")
		return Handled
	}
	return Unhandled
}
