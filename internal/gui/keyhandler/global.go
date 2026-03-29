package keyhandler

import (
	"github.com/KEMSHlM/lazyclaude/internal/gui/keymap"
	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/jesseduffield/gocui"
)

// GlobalHandler handles keys that apply regardless of focused panel.
type GlobalHandler struct {
	panels *PanelManager
	reg    *keymap.Registry
}

// NewGlobalHandler creates a GlobalHandler with injected registry.
func NewGlobalHandler(pm *PanelManager, reg *keymap.Registry) *GlobalHandler {
	return &GlobalHandler{panels: pm, reg: reg}
}

func (h *GlobalHandler) HandleKey(ev KeyEvent, actions AppActions) HandlerResult {
	def, ok := h.reg.Match(ev.Rune, ev.Key, ev.Mod, keymap.ScopeGlobal)
	if !ok {
		// Esc quits in non-main modes (Diff/Tool) — not in registry because
		// Esc has different semantics per mode (popup suspend vs quit).
		if actions.Mode() != 0 && ev.Key == gocui.KeyEsc {
			actions.Quit()
			return Handled
		}
		return Unhandled
	}

	// Skip most global keys in non-main modes
	if actions.Mode() != 0 {
		switch def.Action {
		case keymap.ActionQuitCtrlC:
			actions.Quit()
			return Handled
		default:
			return Unhandled
		}
	}

	switch def.Action {
	case keymap.ActionQuit, keymap.ActionQuitCtrlC, keymap.ActionQuitCtrlBackslash:
		actions.Quit()
	case keymap.ActionFocusNextPanel:
		h.panels.FocusNext()
	case keymap.ActionFocusPrevPanel:
		h.panels.FocusPrev()
	case keymap.ActionUnsuspendPopups:
		actions.UnsuspendPopups()
	case keymap.ActionPanelNextTab:
		actions.PanelNextTab()
	case keymap.ActionPanelPrevTab:
		actions.PanelPrevTab()
	default:
		return Unhandled
	}
	return Handled
}

// OptionsBar returns the global hint bar from registry.
func (h *GlobalHandler) OptionsBar() string {
	hints := h.reg.HintsForScope(keymap.ScopeGlobal)
	defs := make([]presentation.HintDef, 0, len(hints))
	for _, d := range hints {
		defs = append(defs, presentation.HintDef{
			Key:   d.HintKeyLabel(),
			Label: d.HintLabel,
		})
	}
	return presentation.BuildOptionsBar(defs)
}
