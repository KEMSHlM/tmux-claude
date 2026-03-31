package keyhandler

import "github.com/any-context/lazyclaude/internal/gui/keymap"

// Panel represents a focusable area in the TUI (metadata + tab lifecycle).
// Key handling is provided separately via PanelWithHandler.
type Panel interface {
	Name() string        // gocui view name ("sessions", "logs", "plugins")
	Label() string       // display label
	Scope() keymap.Scope // keybinding scope for this panel

	// OnTabChanged is called when the panel's active tab changes.
	// Panels that need side effects (e.g. resetting cursors) implement logic here.
	// Single-tab panels are no-ops.
	OnTabChanged(newTab int, actions AppActions)

	// OptionsBarForTab returns the options bar text for the given tab index.
	// Single-tab panels ignore tabIdx and return a fixed bar.
	OptionsBarForTab(tabIdx int) string

	// Tab support. TabCount returns 1 for single-tab panels.
	TabCount() int
	TabLabels() []string
}

// PanelWithHandler pairs Panel metadata with a key dispatch function.
// The HandleKey function accepts AppActions (the composite interface)
// at the dispatch boundary, while the underlying panel handler depends
// only on its narrow interface.
type PanelWithHandler struct {
	Panel
	HandleKey func(ev KeyEvent, actions AppActions) HandlerResult
}

// PanelManager tracks focus across registered panels.
// Tab/Shift+Tab cycles focusIdx.
type PanelManager struct {
	panels   []PanelWithHandler
	focusIdx int
}

// NewPanelManager creates a PanelManager with the given panel entries.
func NewPanelManager(panels ...PanelWithHandler) *PanelManager {
	return &PanelManager{panels: panels}
}

// ActivePanel returns the currently focused panel entry.
func (pm *PanelManager) ActivePanel() *PanelWithHandler {
	if len(pm.panels) == 0 {
		return nil
	}
	return &pm.panels[pm.focusIdx]
}

// FocusNext advances focus to the next panel (wrapping).
func (pm *PanelManager) FocusNext() {
	if len(pm.panels) == 0 {
		return
	}
	pm.focusIdx = (pm.focusIdx + 1) % len(pm.panels)
}

// FocusPrev moves focus to the previous panel (wrapping).
func (pm *PanelManager) FocusPrev() {
	if len(pm.panels) == 0 {
		return
	}
	pm.focusIdx = (pm.focusIdx - 1 + len(pm.panels)) % len(pm.panels)
}

// FocusIdx returns the current focus index.
func (pm *PanelManager) FocusIdx() int {
	return pm.focusIdx
}

// PanelCount returns the number of registered panels.
func (pm *PanelManager) PanelCount() int {
	return len(pm.panels)
}
