package gui_test

import (
	"strings"
	"testing"

	"github.com/any-context/lazyclaude/internal/core/model"
	"github.com/any-context/lazyclaude/internal/gui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSessionList_NeedsInput_ShowsMagentaBang(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "worker", Status: "Running", Activity: model.ActivityNeedsInput},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "!", "needs-input session should show bang icon")
	assert.Contains(t, out, "\x1b[35m", "needs-input icon should be magenta")
	assert.NotContains(t, out, "\x1b[32m\xe2\x9f\xb3", "needs-input session should NOT show green running icon")
}

func TestRenderSessionList_Running_ShowsGreenSpinner(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "app", Status: "Running", Activity: model.ActivityRunning},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "\xe2\x9f\xb3", "running session should show spinner")
	assert.Contains(t, out, "\x1b[32m", "running icon should be green")
}

func TestRenderSessionList_Dead_ActivityIgnored(t *testing.T) {
	// Even if Activity is set, Dead status takes priority
	items := []gui.SessionItem{
		{ID: "s1", Name: "dead-session", Status: "Dead", Activity: model.ActivityNeedsInput},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "\xc3\x97", "dead session should show dim cross icon")
	assert.NotContains(t, out, "\x1b[35m", "dead session should NOT show magenta needs-input icon")
}

func TestRenderSessionList_Orphan_ActivityIgnored(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "orphan-session", Status: "Orphan", Activity: model.ActivityNeedsInput},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "\xe2\x97\x8b", "orphan session should show empty circle icon")
	assert.NotContains(t, out, "\x1b[35m", "orphan session should NOT show magenta needs-input icon")
}

func TestRenderSessionList_UnknownActivity_FallsBackToStatus(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "app", Status: "Running", Activity: model.ActivityUnknown},
	}
	out := gui.RenderSessionListForTest(items, 0)

	// Should show unknown icon as fallback (not running spinner)
	assert.Contains(t, out, "?", "unknown activity should fall back to unknown icon")
}

func TestRenderSessionList_MultipleSessions_OnlyNeedsInputGetsBang(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "normal", Status: "Running", Activity: model.ActivityRunning},
		{ID: "s2", Name: "blocked", Status: "Running", Activity: model.ActivityNeedsInput},
		{ID: "s3", Name: "dead-one", Status: "Dead"},
	}
	out := gui.RenderSessionListForTest(items, 0)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 3)

	assert.Contains(t, lines[0], "\xe2\x9f\xb3", "first session (running) should have spinner")
	assert.NotContains(t, lines[0], "!", "first session should not have bang")

	assert.Contains(t, lines[1], "!", "second session (needs-input) should have bang")
	assert.Contains(t, lines[1], "\x1b[35m", "second session bang should be magenta")

	assert.Contains(t, lines[2], "\xc3\x97", "third session (dead) should have dim cross")
}

func TestRenderSessionList_NoSessions_ActivityZeroValue(t *testing.T) {
	item := gui.SessionItem{ID: "s1", Name: "app", Status: "Running"}
	assert.Equal(t, model.ActivityUnknown, item.Activity, "Activity zero value should be ActivityUnknown")
}

func TestRenderSessionList_Idle_ShowsCyanCheckmark(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "done", Status: "Running", Activity: model.ActivityIdle},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "\xe2\x9c\x93", "idle session should show checkmark")
	assert.Contains(t, out, "\x1b[36m", "idle icon should be cyan")
}

func TestRenderSessionList_Error_ShowsRedCross(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "failed", Status: "Running", Activity: model.ActivityError},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "\xe2\x9c\x97", "error session should show cross")
	assert.Contains(t, out, "\x1b[31m", "error icon should be red")
}

func TestRenderSessionList_PMRole_ShowsPMBadge(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "pm-session", Status: "Running", Role: "pm"},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "[PM]", "PM role session should show [PM] badge")
}

func TestRenderSessionList_PMRole_BadgeIsPurple(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "pm-session", Status: "Running", Role: "pm"},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "\x1b[38;5;141m", "PM badge should use purple color")
}

func TestRenderSessionList_NonPMRole_NoPMBadge(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "regular", Status: "Running", Role: ""},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.NotContains(t, out, "[PM]", "non-PM session should not show [PM] badge")
}

func TestRenderSessionList_WorkerRole_NoPMBadge(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "worker-session", Status: "Running", Role: "worker"},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.NotContains(t, out, "[PM]", "worker role session should not show [PM] badge")
}

func TestSessionItem_RoleField_IsString(t *testing.T) {
	t.Parallel()
	item := gui.SessionItem{Role: "pm"}
	assert.Equal(t, "pm", item.Role)

	item2 := gui.SessionItem{}
	assert.Equal(t, "", item2.Role)
}
