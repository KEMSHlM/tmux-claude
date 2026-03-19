package notify_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/KEMSHlM/lazyclaude/internal/notify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite_Read_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	n := notify.ToolNotification{
		ToolName:  "Bash",
		Input:     `{"command":"rm -rf /"}`,
		CWD:       "/home/user",
		Window:    "lc-abc12345",
		Timestamp: time.Now().Truncate(time.Second),
	}

	err := notify.Write(dir, n)
	require.NoError(t, err)

	got, err := notify.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, n.ToolName, got.ToolName)
	assert.Equal(t, n.Input, got.Input)
	assert.Equal(t, n.CWD, got.CWD)
	assert.Equal(t, n.Window, got.Window)
}

func TestRead_NoFile_ReturnsNil(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	got, err := notify.Read(dir)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRead_DeletesFileAfterRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	n := notify.ToolNotification{ToolName: "Write", Window: "lc-123"}
	require.NoError(t, notify.Write(dir, n))

	// First read succeeds
	got, err := notify.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, got)

	// Second read returns nil (file deleted)
	got, err = notify.Read(dir)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestIsDiff_True(t *testing.T) {
	t.Parallel()
	n := notify.ToolNotification{ToolName: "Write", OldFilePath: "/tmp/test.go"}
	assert.True(t, n.IsDiff())
}

func TestIsDiff_False(t *testing.T) {
	t.Parallel()
	n := notify.ToolNotification{ToolName: "Bash"}
	assert.False(t, n.IsDiff())
}

func TestWrite_Read_DiffNotification(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	n := notify.ToolNotification{
		ToolName:    "Write",
		OldFilePath: "/home/user/main.go",
		NewContents: "package main\n\nfunc main() {}\n",
		Window:      "lc-abc",
	}
	require.NoError(t, notify.Write(dir, n))

	got, err := notify.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.IsDiff())
	assert.Equal(t, "/home/user/main.go", got.OldFilePath)
	assert.Equal(t, "package main\n\nfunc main() {}\n", got.NewContents)
}

func TestWrite_OverwritesPrevious(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	n1 := notify.ToolNotification{ToolName: "Bash", Window: "lc-111"}
	n2 := notify.ToolNotification{ToolName: "Write", Window: "lc-222"}

	require.NoError(t, notify.Write(dir, n1))
	require.NoError(t, notify.Write(dir, n2))

	got, err := notify.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Write", got.ToolName)
	assert.Equal(t, "lc-222", got.Window)
}

// --- Queue-based notification tests ---

func TestEnqueue_ReadAll_PreservesOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	n1 := notify.ToolNotification{ToolName: "Bash", Window: "lc-111"}
	n2 := notify.ToolNotification{ToolName: "Write", Window: "lc-222"}
	n3 := notify.ToolNotification{ToolName: "Edit", Window: "lc-333"}

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

	require.NoError(t, notify.Enqueue(dir, notify.ToolNotification{ToolName: "Bash", Window: "w"}))
	require.NoError(t, notify.Enqueue(dir, notify.ToolNotification{ToolName: "Write", Window: "w"}))

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
		require.NoError(t, notify.Enqueue(dir, notify.ToolNotification{
			ToolName: "Tool",
			Window:   fmt.Sprintf("w-%d", i),
		}))
	}

	got, err := notify.ReadAll(dir)
	require.NoError(t, err)
	assert.Len(t, got, 10, "all 10 notifications must be preserved")
}
