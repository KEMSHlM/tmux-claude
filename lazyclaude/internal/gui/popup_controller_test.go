package gui

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/core/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestNotif(tool, window string) *model.ToolNotification {
	return &model.ToolNotification{ToolName: tool, Window: window}
}

func TestPopupController_PushAndCount(t *testing.T) {
	t.Parallel()
	pc := NewPopupController()
	assert.Equal(t, 0, pc.Count())
	assert.False(t, pc.HasVisible())

	pc.PushPopup(NewToolPopup(makeTestNotif("Bash", "@0")))
	assert.Equal(t, 1, pc.Count())
	assert.True(t, pc.HasVisible())
}

func TestPopupController_ActiveEntry(t *testing.T) {
	t.Parallel()
	pc := NewPopupController()
	pc.PushPopup(NewToolPopup(makeTestNotif("Bash", "@0")))
	pc.PushPopup(NewToolPopup(makeTestNotif("Write", "@1")))

	active := pc.ActiveNotification()
	require.NotNil(t, active)
	assert.Equal(t, "Write", active.ToolName)
}

func TestPopupController_Dismiss(t *testing.T) {
	t.Parallel()
	pc := NewPopupController()
	pc.PushPopup(NewToolPopup(makeTestNotif("Bash", "@0")))
	pc.PushPopup(NewToolPopup(makeTestNotif("Write", "@1")))

	window := pc.DismissActive(ChoiceAccept)
	assert.Equal(t, "@1", window)
	assert.Equal(t, 1, pc.Count())
	assert.Equal(t, "Bash", pc.ActiveNotification().ToolName)
}

func TestPopupController_DismissAll(t *testing.T) {
	t.Parallel()
	pc := NewPopupController()
	pc.PushPopup(NewToolPopup(makeTestNotif("Bash", "@0")))
	pc.PushPopup(NewToolPopup(makeTestNotif("Write", "@1")))

	windows := pc.DismissAll(ChoiceAccept)
	assert.Equal(t, 0, pc.Count())
	assert.False(t, pc.HasVisible())
	assert.Len(t, windows, 2)
	assert.Equal(t, "@0", windows[0])
	assert.Equal(t, "@1", windows[1])
}

func TestPopupController_SuspendAndUnsuspend(t *testing.T) {
	t.Parallel()
	pc := NewPopupController()
	pc.PushPopup(NewToolPopup(makeTestNotif("Bash", "@0")))
	pc.PushPopup(NewToolPopup(makeTestNotif("Write", "@1")))

	pc.SuspendAll()
	assert.False(t, pc.HasVisible())
	assert.Equal(t, 2, pc.Count())

	pc.UnsuspendAll()
	assert.True(t, pc.HasVisible())
	assert.Equal(t, "Write", pc.ActiveNotification().ToolName)
}

func TestPopupController_FocusCycle(t *testing.T) {
	t.Parallel()
	pc := NewPopupController()
	pc.PushPopup(NewToolPopup(makeTestNotif("A", "@0")))
	pc.PushPopup(NewToolPopup(makeTestNotif("B", "@1")))
	pc.PushPopup(NewToolPopup(makeTestNotif("C", "@2")))

	assert.Equal(t, "C", pc.ActiveNotification().ToolName)

	pc.FocusPrev()
	assert.Equal(t, "B", pc.ActiveNotification().ToolName)

	pc.FocusNext()
	assert.Equal(t, "C", pc.ActiveNotification().ToolName)
}

func TestPopupController_DismissOnEmpty(t *testing.T) {
	t.Parallel()
	pc := NewPopupController()
	window := pc.DismissActive(ChoiceAccept) // should not panic
	assert.Equal(t, "", window)
	assert.Equal(t, 0, pc.Count())
}

func TestPopupController_VisibleCount(t *testing.T) {
	t.Parallel()
	pc := NewPopupController()
	pc.PushPopup(NewToolPopup(makeTestNotif("A", "@0")))
	pc.PushPopup(NewToolPopup(makeTestNotif("B", "@1")))
	assert.Equal(t, 2, pc.VisibleCount())

	pc.SuspendAll()
	assert.Equal(t, 0, pc.VisibleCount())
}

func TestPopupController_ActiveEntry_Scroll(t *testing.T) {
	t.Parallel()
	pc := NewPopupController()
	pc.PushPopup(NewToolPopup(makeTestNotif("Bash", "@0")))

	entry := pc.ActiveEntry()
	require.NotNil(t, entry)
	entry.popup.SetScrollY(5)
	assert.Equal(t, 5, pc.ActiveEntry().popup.ScrollY())
}
