package gui_test

import (
	"testing"
	"time"

	"github.com/KEMSHlM/lazyclaude/internal/core/model"
	"github.com/KEMSHlM/lazyclaude/internal/gui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockInputForwarder_RecordsKeys(t *testing.T) {
	t.Parallel()
	f := &gui.MockInputForwarder{}

	require.NoError(t, f.ForwardKey("@0", "h"))
	require.NoError(t, f.ForwardKey("@0", "e"))
	require.NoError(t, f.ForwardKey("@0", "l"))

	assert.Equal(t, []string{"h", "e", "l"}, f.Keys())
}

func TestMockInputForwarder_RecordsTarget(t *testing.T) {
	t.Parallel()
	f := &gui.MockInputForwarder{}

	f.ForwardKey("@1", "x")
	assert.Equal(t, "@1", f.LastTarget())
}

func TestKeyMapping_PrintableRune(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "a", gui.RuneToLiteral('a'))
	assert.Equal(t, "Z", gui.RuneToLiteral('Z'))
	assert.Equal(t, "1", gui.RuneToLiteral('1'))
	assert.Equal(t, " ", gui.RuneToLiteral(' '))
}

func TestFullScreen_ForwardsKeys(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)

	app.EnterFullScreenForTest("s1")

	app.ForwardKeyForTest('h')

	require.Eventually(t, func() bool { return len(fwd.Keys()) == 1 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, []string{"h"}, fwd.Keys())
}

func TestFullScreen_ForwardsSpecialKey(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	app.EnterFullScreenForTest("s1")

	app.ForwardSpecialKeyForTest("Enter")
	require.Eventually(t, func() bool { return len(fwd.Keys()) == 1 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, []string{"Enter"}, fwd.Keys())
}

func TestFullScreen_ExistingKeysForwardInFullMode(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	app.EnterFullScreenForTest("s1")

	// j in full mode should forward, not move cursor
	cursorBefore := app.CursorForTest()
	app.ForwardKeyForTest('j')
	assert.Equal(t, cursorBefore, app.CursorForTest(), "cursor should not change in full mode")
	require.Eventually(t, func() bool { return len(fwd.Keys()) == 1 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, []string{"j"}, fwd.Keys())
}

func TestFullScreen_KeyOrderPreserved(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	app.EnterFullScreenForTest("s1")

	// Simulate rapid IME-like input: あいうえお mapped to keys a,i,u,e,o
	keys := []rune{'a', 'i', 'u', 'e', 'o'}
	for _, ch := range keys {
		app.ForwardKeyForTest(ch)
	}

	expected := []string{"a", "i", "u", "e", "o"}
	assert.Equal(t, expected, fwd.Keys(), "keys must arrive in order (IME input)")
}

func TestFullScreen_RuneKeysSentAsLiteral(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	app.EnterFullScreenForTest("s1")

	// Rune characters (including tmux metacharacters) must use literal mode
	for _, ch := range []rune{';', '&', '|', '$', '(', ')', 'あ', 'A'} {
		app.ForwardKeyForTest(ch)
	}

	expected := []string{";", "&", "|", "$", "(", ")", "あ", "A"}
	assert.Equal(t, expected, fwd.Literals(), "rune chars must be sent via ForwardLiteral")
}

func TestFullScreen_SpecialKeysSentAsKeyName(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	app.EnterFullScreenForTest("s1")

	// Special keys must NOT be sent as literal
	app.ForwardSpecialKeyForTest("Enter")
	app.ForwardSpecialKeyForTest("Space")

	assert.Equal(t, []string{"Enter", "Space"}, fwd.Keys())
	assert.Empty(t, fwd.Literals(), "special keys must NOT use ForwardLiteral")
}

// --- Paste content callback tests ---
// These test the OnPasteContent → forwardPaste path, which is how paste
// works in the new architecture (gocui layer accumulates, App layer forwards).

func TestPaste_OnPasteContentForwardsToPane(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	app.EnterFullScreenForTest("s1")

	// Simulate what gocui's handleEvent does for eventPasteContent.
	app.HandlePasteContentForTest("hello world\nline two")

	assert.Equal(t, []string{"hello world\nline two"}, fwd.Pastes(),
		"OnPasteContent should forward text via ForwardPaste")
}

func TestPaste_OnPasteContentIgnoredWithoutFullscreen(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	// NOT entering fullscreen mode

	app.HandlePasteContentForTest("should be ignored")

	assert.Empty(t, fwd.Pastes(), "paste should not be forwarded when not in fullscreen")
}

func TestPaste_OnPasteContentBlockedByPopup(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	app.EnterFullScreenForTest("s1")

	// Show popup
	app.ShowToolPopupForTest(&model.ToolNotification{
		ToolName: "Write",
		Window:   "@0",
	})

	app.HandlePasteContentForTest("should be blocked")

	assert.Empty(t, fwd.Pastes(), "paste should not be forwarded when popup is showing")
}

func TestPaste_EmptyTextIgnored(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	app.EnterFullScreenForTest("s1")

	app.HandlePasteContentForTest("")

	assert.Empty(t, fwd.Pastes(), "empty paste should be ignored")
}

func TestFullScreen_PopupBlocksForwarding(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)

	app.EnterFullScreenForTest("s1")

	// Show popup — forwarding should be blocked
	app.ShowToolPopupForTest(&model.ToolNotification{
		ToolName: "Write",
		Window:   "@0",
	})

	app.ForwardKeyForTest('h')
	assert.Empty(t, fwd.Keys(), "keys should not be forwarded when popup is showing")
}

// --- inputEditor.Edit tests (individual key forwarding only) ---

func TestEdit_EscForwardedAsEscape(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	app.EnterFullScreenForTest("s1")
	app.InitEditorForTest()

	// Esc should be forwarded immediately as "Escape" — no buffering.
	app.EditForTest(gui.KeyEscForTest, 0, 0)
	app.DrainQueueForTest()

	assert.Equal(t, []string{"Escape"}, fwd.Keys(),
		"Esc should be forwarded as 'Escape' key name")
}

func TestEdit_RuneForwarded(t *testing.T) {
	app, err := gui.NewAppHeadless(gui.ModeMain, 80, 24)
	require.NoError(t, err)

	mock := &mockSessionProvider{
		sessions: []gui.SessionItem{
			{ID: "s1", Name: "test", Status: "Running", TmuxWindow: "@0"},
		},
	}
	app.SetSessions(mock)

	fwd := &gui.MockInputForwarder{}
	app.SetInputForwarder(fwd)
	app.EnterFullScreenForTest("s1")
	app.InitEditorForTest()

	app.EditForTest(0, 'a', 0)
	app.DrainQueueForTest()

	assert.Equal(t, []string{"a"}, fwd.Literals())
}
