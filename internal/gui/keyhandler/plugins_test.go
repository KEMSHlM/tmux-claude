package keyhandler_test

import (
	"testing"

	"github.com/any-context/lazyclaude/internal/gui/keyhandler"
	"github.com/any-context/lazyclaude/internal/gui/keymap"
	"github.com/jesseduffield/gocui"
)

func TestPluginsPanel_MCPTab_Navigation(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewPluginsPanel(reg)
	p := entry.Panel.(*keyhandler.PluginsPanel)
	tests := []struct {
		ev   keyhandler.KeyEvent
		want string
	}{
		{keyhandler.KeyEvent{Rune: 'j'}, "MCPCursorDown"},
		{keyhandler.KeyEvent{Key: gocui.KeyArrowDown}, "MCPCursorDown"},
		{keyhandler.KeyEvent{Rune: 'k'}, "MCPCursorUp"},
		{keyhandler.KeyEvent{Key: gocui.KeyArrowUp}, "MCPCursorUp"},
	}
	for _, tt := range tests {
		a := &mockPluginsPanelActions{tabIndex: 0} // MCP tab
		r := p.HandleKey(tt.ev, a)
		if r != keyhandler.Handled {
			t.Errorf("key %v: want Handled", tt.ev)
		}
		if a.lastCall() != tt.want {
			t.Errorf("key %v: got %q, want %q", tt.ev, a.lastCall(), tt.want)
		}
	}
}

func TestPluginsPanel_MCPTab_Operations(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewPluginsPanel(reg)
	p := entry.Panel.(*keyhandler.PluginsPanel)
	tests := []struct {
		ev   keyhandler.KeyEvent
		want string
	}{
		{keyhandler.KeyEvent{Rune: 'e'}, "MCPToggleDenied"},
		{keyhandler.KeyEvent{Rune: 'r'}, "MCPRefresh"},
	}
	for _, tt := range tests {
		a := &mockPluginsPanelActions{tabIndex: 0} // MCP tab
		r := p.HandleKey(tt.ev, a)
		if r != keyhandler.Handled {
			t.Errorf("key %v: want Handled", tt.ev)
		}
		if a.lastCall() != tt.want {
			t.Errorf("key %v: got %q, want %q", tt.ev, a.lastCall(), tt.want)
		}
	}
}

func TestPluginsPanel_PluginsTab_Navigation(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewPluginsPanel(reg)
	p := entry.Panel.(*keyhandler.PluginsPanel)
	tests := []struct {
		ev   keyhandler.KeyEvent
		want string
	}{
		{keyhandler.KeyEvent{Rune: 'j'}, "PluginCursorDown"},
		{keyhandler.KeyEvent{Key: gocui.KeyArrowDown}, "PluginCursorDown"},
		{keyhandler.KeyEvent{Rune: 'k'}, "PluginCursorUp"},
		{keyhandler.KeyEvent{Key: gocui.KeyArrowUp}, "PluginCursorUp"},
	}
	for _, tt := range tests {
		a := &mockPluginsPanelActions{tabIndex: 1} // Plugins tab
		r := p.HandleKey(tt.ev, a)
		if r != keyhandler.Handled {
			t.Errorf("key %v: want Handled", tt.ev)
		}
		if a.lastCall() != tt.want {
			t.Errorf("key %v: got %q, want %q", tt.ev, a.lastCall(), tt.want)
		}
	}
}

func TestPluginsPanel_PluginsTab_Operations(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewPluginsPanel(reg)
	p := entry.Panel.(*keyhandler.PluginsPanel)
	tests := []struct {
		ev   keyhandler.KeyEvent
		want string
	}{
		{keyhandler.KeyEvent{Rune: 'd'}, "PluginUninstall"},
		{keyhandler.KeyEvent{Rune: 'e'}, "PluginToggleEnabled"},
		{keyhandler.KeyEvent{Rune: 'u'}, "PluginUpdate"},
		{keyhandler.KeyEvent{Rune: 'r'}, "PluginRefresh"},
	}
	for _, tt := range tests {
		a := &mockPluginsPanelActions{tabIndex: 1} // Plugins tab
		r := p.HandleKey(tt.ev, a)
		if r != keyhandler.Handled {
			t.Errorf("key %v: want Handled", tt.ev)
		}
		if a.lastCall() != tt.want {
			t.Errorf("key %v: got %q, want %q", tt.ev, a.lastCall(), tt.want)
		}
	}
}

func TestPluginsPanel_MarketplaceTab_Operations(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewPluginsPanel(reg)
	p := entry.Panel.(*keyhandler.PluginsPanel)
	tests := []struct {
		ev   keyhandler.KeyEvent
		want string
	}{
		{keyhandler.KeyEvent{Rune: 'i'}, "PluginInstall"},
		{keyhandler.KeyEvent{Rune: 'r'}, "PluginRefresh"},
	}
	for _, tt := range tests {
		a := &mockPluginsPanelActions{tabIndex: 2} // Marketplace tab
		r := p.HandleKey(tt.ev, a)
		if r != keyhandler.Handled {
			t.Errorf("key %v: want Handled", tt.ev)
		}
		if a.lastCall() != tt.want {
			t.Errorf("key %v: got %q, want %q", tt.ev, a.lastCall(), tt.want)
		}
	}
}

func TestPluginsPanel_PluginsTab_RejectsMarketplaceKeys(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewPluginsPanel(reg)
	p := entry.Panel.(*keyhandler.PluginsPanel)
	a := &mockPluginsPanelActions{tabIndex: 1} // Plugins tab
	if p.HandleKey(keyhandler.KeyEvent{Rune: 'i'}, a) != keyhandler.Unhandled {
		t.Error("'i' should be Unhandled on Plugins tab")
	}
}

func TestPluginsPanel_TabSwitchingHandledByGlobal(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewPluginsPanel(reg)
	p := entry.Panel.(*keyhandler.PluginsPanel)
	a := &mockPluginsPanelActions{}

	if p.HandleKey(keyhandler.KeyEvent{Rune: '['}, a) != keyhandler.Unhandled {
		t.Error("[ should be Unhandled by PluginsPanel")
	}
	if p.HandleKey(keyhandler.KeyEvent{Rune: ']'}, a) != keyhandler.Unhandled {
		t.Error("] should be Unhandled by PluginsPanel")
	}
}

func TestPluginsPanel_Unhandled(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewPluginsPanel(reg)
	p := entry.Panel.(*keyhandler.PluginsPanel)
	a := &mockPluginsPanelActions{}
	if p.HandleKey(keyhandler.KeyEvent{Rune: 'x'}, a) != keyhandler.Unhandled {
		t.Error("'x' should be Unhandled")
	}
}

func TestPluginsPanel_OptionsBarForTab(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewPluginsPanel(reg)
	mcpBar := entry.OptionsBarForTab(0)
	pluginsBar := entry.OptionsBarForTab(1)
	marketBar := entry.OptionsBarForTab(2)

	if mcpBar == pluginsBar {
		t.Error("MCP and plugins options bars should differ")
	}
	if pluginsBar == marketBar {
		t.Error("plugins and marketplace options bars should differ")
	}
}

func TestPluginsPanel_Name(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewPluginsPanel(reg)
	if entry.Name() != "plugins" {
		t.Errorf("Name = %q", entry.Name())
	}
	if entry.TabCount() != 3 {
		t.Errorf("TabCount = %d", entry.TabCount())
	}
	labels := entry.TabLabels()
	if len(labels) != 3 || labels[0] != "MCP" || labels[1] != "Plugins" || labels[2] != "Marketplace" {
		t.Errorf("TabLabels = %v", labels)
	}
}
