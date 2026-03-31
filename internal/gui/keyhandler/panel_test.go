package keyhandler_test

import (
	"testing"

	"github.com/any-context/lazyclaude/internal/gui/keyhandler"
	"github.com/any-context/lazyclaude/internal/gui/keymap"
)

func TestPanelManager_FocusNext_Wraps(t *testing.T) {
	reg := keymap.Default()
	pm := keyhandler.NewPanelManager(keyhandler.NewSessionsPanel(reg), keyhandler.NewLogsPanel(reg))

	if pm.FocusIdx() != 0 || pm.ActivePanel().Name() != "sessions" {
		t.Fatal("initial state wrong")
	}
	pm.FocusNext()
	if pm.FocusIdx() != 1 || pm.ActivePanel().Name() != "logs" {
		t.Fatal("after FocusNext should be logs")
	}
	pm.FocusNext()
	if pm.FocusIdx() != 0 || pm.ActivePanel().Name() != "sessions" {
		t.Fatal("should wrap to sessions")
	}
}

func TestPanelManager_FocusPrev_Wraps(t *testing.T) {
	reg := keymap.Default()
	pm := keyhandler.NewPanelManager(keyhandler.NewSessionsPanel(reg), keyhandler.NewLogsPanel(reg))

	pm.FocusPrev()
	if pm.FocusIdx() != 1 {
		t.Fatalf("FocusPrev from 0 should wrap to 1, got %d", pm.FocusIdx())
	}
	pm.FocusPrev()
	if pm.FocusIdx() != 0 {
		t.Fatalf("FocusPrev from 1 should be 0, got %d", pm.FocusIdx())
	}
}

func TestPanelManager_PanelCount(t *testing.T) {
	reg := keymap.Default()
	pm := keyhandler.NewPanelManager(keyhandler.NewSessionsPanel(reg), keyhandler.NewLogsPanel(reg))
	if pm.PanelCount() != 2 {
		t.Fatalf("PanelCount = %d, want 2", pm.PanelCount())
	}
}

func TestPanel_TabSupport(t *testing.T) {
	reg := keymap.Default()
	entry := keyhandler.NewSessionsPanel(reg)
	if entry.TabCount() != 1 {
		t.Errorf("SessionsPanel TabCount = %d, want 1", entry.TabCount())
	}
	labels := entry.TabLabels()
	if len(labels) != 1 || labels[0] != "Sessions" {
		t.Errorf("SessionsPanel TabLabels = %v", labels)
	}
	// OptionsBarForTab ignores tabIdx for single-tab panels
	if entry.OptionsBarForTab(0) == "" {
		t.Error("OptionsBarForTab(0) should not be empty")
	}
}
