package keyhandler

import "github.com/KEMSHlM/lazyclaude/internal/gui/keymap"

// FullScreenHandler handles special keys in full-screen mode.
// Rune keys are NOT handled here — inputEditor.Edit() handles those.
type FullScreenHandler struct {
	reg *keymap.Registry
}

// NewFullScreenHandler creates a FullScreenHandler with injected registry.
func NewFullScreenHandler(reg *keymap.Registry) *FullScreenHandler {
	return &FullScreenHandler{reg: reg}
}

func (h *FullScreenHandler) HandleKey(ev KeyEvent, actions AppActions) HandlerResult {
	if !actions.IsFullScreen() {
		return Unhandled
	}

	def, ok := h.reg.Match(ev.Rune, ev.Key, ev.Mod, keymap.ScopeFullScreen)
	if !ok {
		return Unhandled
	}

	switch def.Action {
	case keymap.ActionExitFull:
		actions.ExitFullScreen()
	case keymap.ActionForwardEnter:
		actions.ForwardSpecialKey("Enter")
	case keymap.ActionForwardEsc:
		actions.ForwardSpecialKey("Escape")
	case keymap.ActionForwardDown:
		actions.ForwardSpecialKey("Down")
	case keymap.ActionForwardUp:
		actions.ForwardSpecialKey("Up")
	default:
		return Unhandled
	}
	return Handled
}
