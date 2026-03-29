package keyhandler

import (
	"github.com/KEMSHlM/lazyclaude/internal/core/choice"
	"github.com/KEMSHlM/lazyclaude/internal/gui/keymap"
)

// PopupHandler handles keys when a popup is visible.
// Highest priority: consumes ALL keys to prevent leaking to panels.
type PopupHandler struct {
	reg *keymap.Registry
}

// NewPopupHandler creates a PopupHandler with injected registry.
func NewPopupHandler(reg *keymap.Registry) *PopupHandler {
	return &PopupHandler{reg: reg}
}

func (h *PopupHandler) HandleKey(ev KeyEvent, actions AppActions) HandlerResult {
	if !actions.HasPopup() {
		return Unhandled
	}

	if def, ok := h.reg.Match(ev.Rune, ev.Key, ev.Mod, keymap.ScopePopup); ok {
		switch def.Action {
		case keymap.ActionPopupAccept:
			actions.DismissPopup(choice.Accept)
		case keymap.ActionPopupAllow:
			actions.DismissPopup(choice.Allow)
		case keymap.ActionPopupReject:
			actions.DismissPopup(choice.Reject)
		case keymap.ActionPopupAcceptAll:
			actions.DismissAllPopups(choice.Accept)
		case keymap.ActionPopupSuspend:
			actions.SuspendPopups()
		case keymap.ActionPopupFocusNext:
			actions.PopupFocusNext()
		case keymap.ActionPopupFocusPrev:
			actions.PopupFocusPrev()
		case keymap.ActionPopupScrollDown:
			actions.PopupScrollDown()
		case keymap.ActionPopupScrollUp:
			actions.PopupScrollUp()
		}
	}

	// Consume ALL keys when popup is visible — prevent leaking to panels.
	return Handled
}
