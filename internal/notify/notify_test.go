package notify_test

import (
	"fmt"
	"testing"

	"github.com/any-context/lazyclaude/internal/core/model"
	"github.com/any-context/lazyclaude/internal/notify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Queue-based notification tests ---

func TestEnqueue_ReadAll_PreservesOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	n1 := model.ToolNotification{ToolName: "Bash", Window: "lc-111"}
	n2 := model.ToolNotification{ToolName: "Write", Window: "lc-222"}
	n3 := model.ToolNotification{ToolName: "Edit", Window: "lc-333"}

	require.NoError(t, notify.Enqueue(dir, n1))
	require.NoError(t, notify.Enqueue(dir, n2))
	require.NoError(t, notify.Enqueue(dir, n3))

	got, err := notify.ReadAll(dir)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "Bash", got[0].ToolName)
	assert.Equal(t, "Write", got[1].ToolName)
	assert.Equal(t, "Edit", got[2].ToolName)
}

func TestEnqueue_ReadAll_DeletesAfterRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, notify.Enqueue(dir, model.ToolNotification{ToolName: "Bash", Window: "w"}))
	require.NoError(t, notify.Enqueue(dir, model.ToolNotification{ToolName: "Write", Window: "w"}))

	got, err := notify.ReadAll(dir)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Second ReadAll returns empty
	got, err = notify.ReadAll(dir)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestReadAll_Empty_ReturnsNil(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	got, err := notify.ReadAll(dir)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestEnqueue_NoLoss_RapidWrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Simulate rapid consecutive writes (the scenario that caused notification loss)
	for i := 0; i < 10; i++ {
		require.NoError(t, notify.Enqueue(dir, model.ToolNotification{
			ToolName: "Tool",
			Window:   fmt.Sprintf("w-%d", i),
		}))
	}

	got, err := notify.ReadAll(dir)
	require.NoError(t, err)
	assert.Len(t, got, 10, "all 10 notifications must be preserved")
}
