package keymap_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui/keymap"
	"github.com/jesseduffield/gocui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_Register_And_AllActions(t *testing.T) {
	t.Parallel()
	r := keymap.NewRegistry()
	r.Register(keymap.ActionDef{
		Action:   keymap.ActionQuit,
		Bindings: []keymap.KeyBinding{{Rune: 'q'}},
		Scope:    keymap.ScopeGlobal,
	})

	defs := r.AllActions()
	require.Len(t, defs, 1)
	assert.Equal(t, keymap.ActionQuit, defs[0].Action)
}

func TestRegistry_Match_RuneKey(t *testing.T) {
	t.Parallel()
	r := keymap.NewRegistry()
	r.Register(keymap.ActionDef{
		Action:   keymap.ActionQuit,
		Bindings: []keymap.KeyBinding{{Rune: 'q'}},
		Scope:    keymap.ScopeGlobal,
	})

	def, ok := r.Match('q', 0, gocui.ModNone, keymap.ScopeGlobal)
	require.True(t, ok)
	assert.Equal(t, keymap.ActionQuit, def.Action)
}

func TestRegistry_Match_WrongScope_NoMatch(t *testing.T) {
	t.Parallel()
	r := keymap.NewRegistry()
	r.Register(keymap.ActionDef{
		Action:   keymap.ActionQuit,
		Bindings: []keymap.KeyBinding{{Rune: 'q'}},
		Scope:    keymap.ScopeGlobal,
	})

	_, ok := r.Match('q', 0, gocui.ModNone, keymap.ScopeSession)
	assert.False(t, ok)
}

func TestRegistry_Match_MultipleBindings(t *testing.T) {
	t.Parallel()
	r := keymap.NewRegistry()
	r.Register(keymap.ActionDef{
		Action:   keymap.ActionCursorUp,
		Bindings: []keymap.KeyBinding{{Rune: 'k'}, {Key: gocui.KeyArrowUp}},
		Scope:    keymap.ScopeSession,
	})

	def, ok := r.Match('k', 0, gocui.ModNone, keymap.ScopeSession)
	require.True(t, ok)
	assert.Equal(t, keymap.ActionCursorUp, def.Action)

	def, ok = r.Match(0, gocui.KeyArrowUp, gocui.ModNone, keymap.ScopeSession)
	require.True(t, ok)
	assert.Equal(t, keymap.ActionCursorUp, def.Action)
}

func TestRegistry_Match_SpecialKey(t *testing.T) {
	t.Parallel()
	r := keymap.NewRegistry()
	r.Register(keymap.ActionDef{
		Action:   keymap.ActionExitFull,
		Bindings: []keymap.KeyBinding{{Key: gocui.KeyCtrlD}},
		Scope:    keymap.ScopeFullScreen,
	})

	_, ok := r.Match(0, gocui.KeyCtrlD, gocui.ModNone, keymap.ScopeFullScreen)
	assert.True(t, ok)
	_, ok = r.Match(0, gocui.KeyCtrlD, gocui.ModNone, keymap.ScopeGlobal)
	assert.False(t, ok)
}

func TestRegistry_AllActions_Order(t *testing.T) {
	t.Parallel()
	r := keymap.NewRegistry()
	r.Register(keymap.ActionDef{Action: keymap.ActionQuit, Bindings: []keymap.KeyBinding{{Rune: 'q'}}, Scope: keymap.ScopeGlobal})
	r.Register(keymap.ActionDef{Action: keymap.ActionEnterFull, Bindings: []keymap.KeyBinding{{Key: gocui.KeyEnter}}, Scope: keymap.ScopeSession})

	defs := r.AllActions()
	require.Len(t, defs, 2)
	assert.Equal(t, keymap.ActionQuit, defs[0].Action)
	assert.Equal(t, keymap.ActionEnterFull, defs[1].Action)
}

func TestRegistry_HintsForScope(t *testing.T) {
	t.Parallel()
	r := keymap.NewRegistry()
	r.Register(keymap.ActionDef{
		Action:    keymap.ActionQuit,
		Bindings:  []keymap.KeyBinding{{Rune: 'q'}},
		Scope:     keymap.ScopeGlobal,
		HintLabel: "quit",
	})
	r.Register(keymap.ActionDef{
		Action:   keymap.ActionFocusNextPanel,
		Bindings: []keymap.KeyBinding{{Key: gocui.KeyTab}},
		Scope:    keymap.ScopeGlobal,
	})
	r.Register(keymap.ActionDef{
		Action:    keymap.ActionNewSession,
		Bindings:  []keymap.KeyBinding{{Rune: 'n'}},
		Scope:     keymap.ScopeSession,
		HintLabel: "new",
	})

	globalHints := r.HintsForScope(keymap.ScopeGlobal)
	require.Len(t, globalHints, 1)
	assert.Equal(t, "quit", globalHints[0].HintLabel)

	sessionHints := r.HintsForScope(keymap.ScopeSession)
	require.Len(t, sessionHints, 1)
	assert.Equal(t, "new", sessionHints[0].HintLabel)
}

func TestRegistry_BindingsForScope(t *testing.T) {
	t.Parallel()
	r := keymap.NewRegistry()
	r.Register(keymap.ActionDef{Action: keymap.ActionQuit, Bindings: []keymap.KeyBinding{{Rune: 'q'}}, Scope: keymap.ScopeGlobal})
	r.Register(keymap.ActionDef{Action: keymap.ActionNewSession, Bindings: []keymap.KeyBinding{{Rune: 'n'}}, Scope: keymap.ScopeSession})
	r.Register(keymap.ActionDef{Action: keymap.ActionDeleteSession, Bindings: []keymap.KeyBinding{{Rune: 'd'}}, Scope: keymap.ScopeSession})

	sessionDefs := r.BindingsForScope(keymap.ScopeSession)
	require.Len(t, sessionDefs, 2)
}

func TestDefault_HasAllScopes(t *testing.T) {
	t.Parallel()
	r := keymap.Default()
	defs := r.AllActions()
	assert.GreaterOrEqual(t, len(defs), 40, "default registry should have all actions")

	scopes := make(map[keymap.Scope]bool)
	for _, d := range defs {
		scopes[d.Scope] = true
	}
	assert.True(t, scopes[keymap.ScopeGlobal])
	assert.True(t, scopes[keymap.ScopeSession])
	assert.True(t, scopes[keymap.ScopePlugins])
	// ScopeMarketplace removed — plugins use Tab field instead
	assert.True(t, scopes[keymap.ScopeLog])
	assert.True(t, scopes[keymap.ScopePopup])
	assert.True(t, scopes[keymap.ScopeFullScreen])
}

func TestDefault_CtrlBackslash_ExitsFullScreen(t *testing.T) {
	t.Parallel()
	r := keymap.Default()
	def, ok := r.Match(0, gocui.KeyCtrlBackslash, gocui.ModNone, keymap.ScopeFullScreen)
	require.True(t, ok, "Ctrl+\\ should match in fullscreen scope")
	assert.Equal(t, keymap.ActionExitFull, def.Action)
}

func TestDefault_CtrlBackslash_NotInGlobal(t *testing.T) {
	t.Parallel()
	r := keymap.Default()
	// Ctrl+\ in global scope is ActionQuitCtrlBackslash, not ActionExitFull
	def, ok := r.Match(0, gocui.KeyCtrlBackslash, gocui.ModNone, keymap.ScopeGlobal)
	require.True(t, ok)
	assert.Equal(t, keymap.ActionQuitCtrlBackslash, def.Action)
}

func TestHintKeyLabel_WithHintKey(t *testing.T) {
	t.Parallel()
	def := keymap.ActionDef{
		Action:    keymap.ActionCollapseProject,
		Bindings:  []keymap.KeyBinding{{Rune: 'h'}},
		HintLabel: "fold",
		HintKey:   "h/l",
	}
	assert.Equal(t, "h/l", def.HintKeyLabel())
}

func TestHintKeyLabel_AutoFromBinding(t *testing.T) {
	t.Parallel()
	def := keymap.ActionDef{
		Action:    keymap.ActionQuit,
		Bindings:  []keymap.KeyBinding{{Rune: 'q'}},
		HintLabel: "quit",
	}
	assert.Equal(t, "q", def.HintKeyLabel())
}

func TestHintKeyLabel_SpecialKey(t *testing.T) {
	t.Parallel()
	def := keymap.ActionDef{
		Action:    keymap.ActionPopupSuspend,
		Bindings:  []keymap.KeyBinding{{Key: gocui.KeyEsc}},
		HintLabel: "hide",
	}
	assert.Equal(t, "Esc", def.HintKeyLabel())
}

func TestKeyBinding_HintKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		binding keymap.KeyBinding
		want    string
	}{
		{keymap.KeyBinding{Rune: 'q'}, "q"},
		{keymap.KeyBinding{Key: gocui.KeyEnter}, "Enter"},
		{keymap.KeyBinding{Key: gocui.KeyEsc}, "Esc"},
		{keymap.KeyBinding{Key: gocui.KeyTab}, "Tab"},
		{keymap.KeyBinding{Key: gocui.KeyCtrlY}, "C-y"},
		{keymap.KeyBinding{Key: gocui.KeyCtrlBackslash}, "C-\\"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.binding.HintKey())
	}
}

func TestMatchTab_FiltersCorrectly(t *testing.T) {
	t.Parallel()
	r := keymap.NewRegistry()
	r.Register(keymap.ActionDef{
		Action:   keymap.ActionPluginUninstall,
		Bindings: []keymap.KeyBinding{{Rune: 'd'}},
		Scope:    keymap.ScopePlugins,
		Tab:      0,
	})
	r.Register(keymap.ActionDef{
		Action:   keymap.ActionPluginInstall,
		Bindings: []keymap.KeyBinding{{Rune: 'i'}},
		Scope:    keymap.ScopePlugins,
		Tab:      1,
	})
	r.Register(keymap.ActionDef{
		Action:   keymap.ActionPluginRefresh,
		Bindings: []keymap.KeyBinding{{Rune: 'r'}},
		Scope:    keymap.ScopePlugins,
		Tab:      keymap.TabAll,
	})

	// 'd' matches on tab 0 (Installed) but not on tab 1
	_, ok := r.MatchTab('d', 0, gocui.ModNone, keymap.ScopePlugins, 0)
	assert.True(t, ok)
	_, ok = r.MatchTab('d', 0, gocui.ModNone, keymap.ScopePlugins, 1)
	assert.False(t, ok)

	// 'i' matches on tab 1 (Marketplace) but not on tab 0
	_, ok = r.MatchTab('i', 0, gocui.ModNone, keymap.ScopePlugins, 1)
	assert.True(t, ok)
	_, ok = r.MatchTab('i', 0, gocui.ModNone, keymap.ScopePlugins, 0)
	assert.False(t, ok)

	// 'r' matches on both tabs (TabAll)
	_, ok = r.MatchTab('r', 0, gocui.ModNone, keymap.ScopePlugins, 0)
	assert.True(t, ok)
	_, ok = r.MatchTab('r', 0, gocui.ModNone, keymap.ScopePlugins, 1)
	assert.True(t, ok)
}

func TestHintsForScopeTab(t *testing.T) {
	t.Parallel()
	r := keymap.NewRegistry()
	r.Register(keymap.ActionDef{
		Action: keymap.ActionPluginToggleEnabled, Bindings: []keymap.KeyBinding{{Rune: 'e'}},
		Scope: keymap.ScopePlugins, Tab: 0, HintLabel: "toggle",
	})
	r.Register(keymap.ActionDef{
		Action: keymap.ActionPluginInstall, Bindings: []keymap.KeyBinding{{Rune: 'i'}},
		Scope: keymap.ScopePlugins, Tab: 1, HintLabel: "install",
	})
	r.Register(keymap.ActionDef{
		Action: keymap.ActionPluginRefresh, Bindings: []keymap.KeyBinding{{Rune: 'r'}},
		Scope: keymap.ScopePlugins, Tab: keymap.TabAll, HintLabel: "refresh",
	})

	// Tab 0: toggle + refresh
	hints0 := r.HintsForScopeTab(keymap.ScopePlugins, 0)
	require.Len(t, hints0, 2)
	assert.Equal(t, "toggle", hints0[0].HintLabel)
	assert.Equal(t, "refresh", hints0[1].HintLabel)

	// Tab 1: install + refresh
	hints1 := r.HintsForScopeTab(keymap.ScopePlugins, 1)
	require.Len(t, hints1, 2)
	assert.Equal(t, "install", hints1[0].HintLabel)
	assert.Equal(t, "refresh", hints1[1].HintLabel)
}
