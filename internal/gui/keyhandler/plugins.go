package keyhandler

import (
	"github.com/KEMSHlM/lazyclaude/internal/gui/keymap"
	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
)

// PluginsPanel handles keys for the plugins view (middle-left).
// Stateless: all state (including tab index) is managed by App.
// Tab switching ([/]) is handled by GlobalHandler as a generic panel operation.
type PluginsPanel struct {
	reg *keymap.Registry
}

// NewPluginsPanel creates a PluginsPanel with injected registry.
func NewPluginsPanel(reg *keymap.Registry) *PluginsPanel {
	return &PluginsPanel{reg: reg}
}

func (p *PluginsPanel) Name() string  { return "plugins" }
func (p *PluginsPanel) Label() string { return "Plugins" }

func (p *PluginsPanel) HandleKey(ev KeyEvent, actions AppActions) HandlerResult {
	scope := keymap.ScopePlugins
	if actions.ActivePanelTabIndex() == 1 {
		scope = keymap.ScopeMarketplace
	}
	def, ok := p.reg.Match(ev.Rune, ev.Key, ev.Mod, scope)
	if !ok {
		return Unhandled
	}

	switch def.Action {
	case keymap.ActionPluginCursorDown:
		actions.PluginCursorDown()
	case keymap.ActionPluginCursorUp:
		actions.PluginCursorUp()
	case keymap.ActionPluginInstall:
		actions.PluginInstall()
	case keymap.ActionPluginUninstall:
		actions.PluginUninstall()
	case keymap.ActionPluginToggleEnabled:
		actions.PluginToggleEnabled()
	case keymap.ActionPluginUpdate:
		actions.PluginUpdate()
	case keymap.ActionPluginRefresh:
		actions.PluginRefresh()
	default:
		return Unhandled
	}
	return Handled
}

// OptionsBarForTab returns the options bar for the given tab.
// Tab 0 = Installed (ScopePlugins), Tab 1 = Marketplace (ScopeMarketplace).
func (p *PluginsPanel) OptionsBarForTab(tabIdx int) string {
	scope := keymap.ScopePlugins
	if tabIdx == 1 {
		scope = keymap.ScopeMarketplace
	}
	hints := p.reg.HintsForScope(scope)
	defs := make([]presentation.HintDef, 0, len(hints))
	for _, d := range hints {
		defs = append(defs, presentation.HintDef{
			Key:   d.HintKeyLabel(),
			Label: d.HintLabel,
		})
	}
	return presentation.BuildOptionsBar(defs)
}

func (p *PluginsPanel) TabCount() int       { return 2 }
func (p *PluginsPanel) TabLabels() []string { return []string{"Installed", "Marketplace"} }
