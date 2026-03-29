package keyhandler_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui/keyhandler"
	"github.com/jesseduffield/gocui"
)

func TestPluginsPanel_Navigation(t *testing.T) {
	p := &keyhandler.PluginsPanel{}
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
		a := newMockActions()
		r := p.HandleKey(tt.ev, a)
		if r != keyhandler.Handled {
			t.Errorf("key %v: want Handled", tt.ev)
		}
		if a.lastCall() != tt.want {
			t.Errorf("key %v: got %q, want %q", tt.ev, a.lastCall(), tt.want)
		}
	}
}

func TestPluginsPanel_TabSwitching(t *testing.T) {
	p := &keyhandler.PluginsPanel{}

	a := newMockActions()
	p.HandleKey(keyhandler.KeyEvent{Rune: ']'}, a)
	if a.lastCall() != "PluginNextTab" {
		t.Errorf("]: got %q, want PluginNextTab", a.lastCall())
	}

	a2 := newMockActions()
	p.HandleKey(keyhandler.KeyEvent{Rune: '['}, a2)
	if a2.lastCall() != "PluginPrevTab" {
		t.Errorf("[: got %q, want PluginPrevTab", a2.lastCall())
	}
}

func TestPluginsPanel_Operations(t *testing.T) {
	p := &keyhandler.PluginsPanel{}
	tests := []struct {
		ev   keyhandler.KeyEvent
		want string
	}{
		{keyhandler.KeyEvent{Rune: 'i'}, "PluginInstall"},
		{keyhandler.KeyEvent{Rune: 'd'}, "PluginUninstall"},
		{keyhandler.KeyEvent{Rune: 'e'}, "PluginToggleEnabled"},
		{keyhandler.KeyEvent{Rune: 'u'}, "PluginUpdate"},
		{keyhandler.KeyEvent{Rune: 'r'}, "PluginRefresh"},
	}
	for _, tt := range tests {
		a := newMockActions()
		r := p.HandleKey(tt.ev, a)
		if r != keyhandler.Handled {
			t.Errorf("key %v: want Handled", tt.ev)
		}
		if a.lastCall() != tt.want {
			t.Errorf("key %v: got %q, want %q", tt.ev, a.lastCall(), tt.want)
		}
	}
}

func TestPluginsPanel_Unhandled(t *testing.T) {
	p := &keyhandler.PluginsPanel{}
	a := newMockActions()
	if p.HandleKey(keyhandler.KeyEvent{Rune: 'x'}, a) != keyhandler.Unhandled {
		t.Error("'x' should be Unhandled")
	}
}

func TestPluginsPanel_OptionsBars(t *testing.T) {
	p := &keyhandler.PluginsPanel{}
	if p.InstalledOptionsBar() == p.MarketplaceOptionsBar() {
		t.Error("installed and marketplace options bars should differ")
	}
}

func TestPluginsPanel_Name(t *testing.T) {
	p := &keyhandler.PluginsPanel{}
	if p.Name() != "plugins" {
		t.Errorf("Name = %q", p.Name())
	}
	if p.TabCount() != 2 {
		t.Errorf("TabCount = %d", p.TabCount())
	}
}
