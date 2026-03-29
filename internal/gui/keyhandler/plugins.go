package keyhandler

import (
	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/jesseduffield/gocui"
)

// PluginsPanel handles keys for the plugins view (middle-left).
// Stateless: all state is managed by AppActions (via App).
// Supports two tabs within the panel: Installed and Marketplace.
type PluginsPanel struct{}

func (p *PluginsPanel) Name() string  { return "plugins" }
func (p *PluginsPanel) Label() string { return "Plugins" }

func (p *PluginsPanel) HandleKey(ev KeyEvent, actions AppActions) HandlerResult {
	switch {
	case ev.Rune == 'j' || ev.Key == gocui.KeyArrowDown:
		actions.PluginCursorDown()
		return Handled
	case ev.Rune == 'k' || ev.Key == gocui.KeyArrowUp:
		actions.PluginCursorUp()
		return Handled
	case ev.Rune == ']':
		actions.PluginNextTab()
		return Handled
	case ev.Rune == '[':
		actions.PluginPrevTab()
		return Handled
	case ev.Rune == 'i':
		actions.PluginInstall()
		return Handled
	case ev.Rune == 'd':
		actions.PluginUninstall()
		return Handled
	case ev.Rune == 'e':
		actions.PluginToggleEnabled()
		return Handled
	case ev.Rune == 'u':
		actions.PluginUpdate()
		return Handled
	case ev.Rune == 'r':
		actions.PluginRefresh()
		return Handled
	}
	return Unhandled
}

// InstalledOptionsBar returns the options bar when Installed tab is active.
func (p *PluginsPanel) InstalledOptionsBar() string {
	return " " +
		presentation.StyledKey("e", "toggle") + "  " +
		presentation.StyledKey("d", "uninstall") + "  " +
		presentation.StyledKey("u", "update") + "  " +
		presentation.StyledKey("r", "refresh") + "  " +
		presentation.StyledKey("[/]", "tab") + "  " +
		presentation.StyledKey("q", "quit")
}

// MarketplaceOptionsBar returns the options bar when Marketplace tab is active.
func (p *PluginsPanel) MarketplaceOptionsBar() string {
	return " " +
		presentation.StyledKey("i", "install") + "  " +
		presentation.StyledKey("r", "refresh") + "  " +
		presentation.StyledKey("[/]", "tab") + "  " +
		presentation.StyledKey("q", "quit")
}

// OptionsBar returns the default (Installed tab) options bar.
func (p *PluginsPanel) OptionsBar() string {
	return p.InstalledOptionsBar()
}

func (p *PluginsPanel) TabCount() int       { return 2 }
func (p *PluginsPanel) TabIndex() int       { return 0 }
func (p *PluginsPanel) TabLabels() []string { return []string{"Installed", "Marketplace"} }
