package keyhandler

import (
	"github.com/any-context/lazyclaude/internal/gui/keymap"
	"github.com/any-context/lazyclaude/internal/gui/presentation"
)

// LogsPanel handles keys for the logs view (lower-left).
type LogsPanel struct {
	reg *keymap.Registry
}

// NewLogsPanel creates a LogsPanel and returns it wrapped as
// a PanelWithHandler for use with PanelManager.
func NewLogsPanel(reg *keymap.Registry) PanelWithHandler {
	p := &LogsPanel{reg: reg}
	return PanelWithHandler{
		Panel: p,
		HandleKey: func(ev KeyEvent, actions AppActions) HandlerResult {
			return p.HandleKey(ev, actions)
		},
	}
}

func (p *LogsPanel) Name() string        { return "logs" }
func (p *LogsPanel) Label() string       { return "Logs" }
func (p *LogsPanel) Scope() keymap.Scope { return keymap.ScopeLog }

func (p *LogsPanel) OnTabChanged(_ int, _ AppActions) {} // single-tab: no-op

// HandleKey dispatches logs-scoped key events.
// Depends only on LogsActions.
func (p *LogsPanel) HandleKey(ev KeyEvent, actions LogsActions) HandlerResult {
	def, ok := p.reg.Match(ev.Rune, ev.Key, ev.Mod, keymap.ScopeLog)
	if !ok {
		return Unhandled
	}

	switch def.Action {
	case keymap.ActionLogsCursorDown:
		actions.LogsCursorDown()
	case keymap.ActionLogsCursorUp:
		actions.LogsCursorUp()
	case keymap.ActionLogsCursorToEnd:
		actions.LogsCursorToEnd()
	case keymap.ActionLogsCursorToTop:
		actions.LogsCursorToTop()
	case keymap.ActionLogsToggleSelect:
		actions.LogsToggleSelect()
	case keymap.ActionLogsCopySelection:
		actions.LogsCopySelection()
	case keymap.ActionStartSearch:
		actions.StartSearch()
	default:
		return Unhandled
	}
	return Handled
}

func (p *LogsPanel) OptionsBarForTab(_ int) string {
	hints := p.reg.HintsForScope(keymap.ScopeLog)
	defs := make([]presentation.HintDef, 0, len(hints))
	for _, d := range hints {
		defs = append(defs, presentation.HintDef{
			Key:   d.HintKeyLabel(),
			Label: d.HintLabel,
		})
	}
	return presentation.BuildOptionsBar(defs)
}

func (p *LogsPanel) TabCount() int       { return 1 }
func (p *LogsPanel) TabLabels() []string { return []string{"Logs"} }
