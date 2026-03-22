package keyhandler

// Panel represents a focusable area in the TUI.
// Each panel manages its own key handling and options bar.
// Panels optionally support tabs (sub-content switching within a panel).
type Panel interface {
	Name() string  // gocui view name ("sessions", "logs")
	Label() string // active tab label (or panel name if no tabs)
	HandleKey(ev KeyEvent, actions AppActions) HandlerResult
	OptionsBar() string

	// Tab support. Panels with a single tab return fixed values.
	TabCount() int
	TabIndex() int
	TabLabels() []string
}

// PanelManager tracks focus across registered panels.
// Tab/Shift+Tab cycles focusIdx.
type PanelManager struct {
	panels   []Panel
	focusIdx int
}

// NewPanelManager creates a PanelManager with the given panels.
func NewPanelManager(panels ...Panel) *PanelManager {
	return &PanelManager{panels: panels}
}

// ActivePanel returns the currently focused panel.
func (pm *PanelManager) ActivePanel() Panel {
	if len(pm.panels) == 0 {
		return nil
	}
	return pm.panels[pm.focusIdx]
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

// Panels returns all registered panels.
func (pm *PanelManager) Panels() []Panel {
	return pm.panels
}

// FocusIdx returns the current focus index.
func (pm *PanelManager) FocusIdx() int {
	return pm.focusIdx
}

// PanelCount returns the number of registered panels.
func (pm *PanelManager) PanelCount() int {
	return len(pm.panels)
}
