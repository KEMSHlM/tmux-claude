package tmux_test

import (
	"context"
	"errors"
	"testing"

	"github.com/any-context/lazyclaude/internal/core/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindWindowForPidWithPanes_DirectMatch(t *testing.T) {
	t.Parallel()
	m := tmux.NewMockClient()
	m.Sessions["claude"] = []tmux.WindowInfo{
		{ID: "@1", Index: 0, Name: "lc-abc", Session: "claude"},
		{ID: "@2", Index: 1, Name: "lc-def", Session: "claude"},
	}
	m.Messages["@1"] = "claude"
	m.Messages["@2"] = "claude"

	panes := []tmux.PaneInfo{
		{ID: "%1", Window: "@1", PID: 1001},
		{ID: "%2", Window: "@2", PID: 1002},
	}

	// PID 1001 directly matches pane %1 -> window @1
	getParent := func(pid int) (int, error) { return 0, errors.New("should not be called") }

	w, err := tmux.FindWindowForPidWithPanes(context.Background(), m, 1001, panes, getParent)
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.Equal(t, "@1", w.ID)
	assert.Equal(t, "lc-abc", w.Name)
}

func TestFindWindowForPidWithPanes_ParentMatch(t *testing.T) {
	t.Parallel()
	m := tmux.NewMockClient()
	m.Sessions["claude"] = []tmux.WindowInfo{
		{ID: "@1", Index: 0, Name: "lc-abc", Session: "claude"},
	}
	m.Messages["@1"] = "claude"

	panes := []tmux.PaneInfo{
		{ID: "%1", Window: "@1", PID: 1001},
	}

	// Process tree: 3333 -> 2222 -> 1001 (pane pid)
	parentMap := map[int]int{
		3333: 2222,
		2222: 1001,
	}
	getParent := func(pid int) (int, error) {
		if p, ok := parentMap[pid]; ok {
			return p, nil
		}
		return 0, errors.New("not found")
	}

	w, err := tmux.FindWindowForPidWithPanes(context.Background(), m, 3333, panes, getParent)
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.Equal(t, "@1", w.ID)
}

func TestFindWindowForPidWithPanes_NoMatch(t *testing.T) {
	t.Parallel()
	m := tmux.NewMockClient()
	panes := []tmux.PaneInfo{
		{ID: "%1", Window: "@1", PID: 1001},
	}

	// Process tree: 9999 -> 9998 -> 1 (init, stops)
	parentMap := map[int]int{
		9999: 9998,
		9998: 1,
	}
	getParent := func(pid int) (int, error) {
		if p, ok := parentMap[pid]; ok {
			return p, nil
		}
		return 0, errors.New("not found")
	}

	w, err := tmux.FindWindowForPidWithPanes(context.Background(), m, 9999, panes, getParent)
	require.NoError(t, err)
	assert.Nil(t, w)
}

func TestFindWindowForPidWithPanes_EmptyPanes(t *testing.T) {
	t.Parallel()
	m := tmux.NewMockClient()

	getParent := func(pid int) (int, error) { return 0, errors.New("none") }

	w, err := tmux.FindWindowForPidWithPanes(context.Background(), m, 1234, nil, getParent)
	require.NoError(t, err)
	assert.Nil(t, w)
}

func TestFindWindowForPidWithPanes_SafetyLimit(t *testing.T) {
	t.Parallel()
	m := tmux.NewMockClient()
	panes := []tmux.PaneInfo{
		{ID: "%1", Window: "@1", PID: 1},
	}

	// Infinite loop: each pid returns pid+1
	calls := 0
	getParent := func(pid int) (int, error) {
		calls++
		return pid + 1, nil
	}

	w, err := tmux.FindWindowForPidWithPanes(context.Background(), m, 100, panes, getParent)
	require.NoError(t, err)
	assert.Nil(t, w)
	assert.LessOrEqual(t, calls, 20) // safety limit
}