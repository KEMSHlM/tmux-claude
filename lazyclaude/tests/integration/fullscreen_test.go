package integration_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// e2eBinary returns the lazyclaude binary path.
func e2eBinary(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat("/usr/local/bin/lazyclaude"); err == nil {
		return "/usr/local/bin/lazyclaude"
	}
	return ensureBinary(t)
}

// cleanLazyClaudeState kills any shared lazyclaude tmux server and removes port file.
// Required between E2E tests that start lazyclaude (shared internal socket).
func cleanLazyClaudeState(t *testing.T) {
	t.Helper()
	exec.Command("tmux", "-L", "lazyclaude", "kill-server").Run()
	os.Remove(filepath.Join(os.TempDir(), "lazyclaude-mcp.port"))
	// Remove stale state file so lazyclaude starts fresh
	os.Remove(filepath.Join(os.Getenv("HOME"), ".local", "share", "lazyclaude", "state.json"))
	time.Sleep(500 * time.Millisecond)
}

// TestE2E_LazyClaudeTUI_StartsAndShowsSessions verifies basic TUI startup.
func TestE2E_LazyClaudeTUI_StartsAndShowsSessions(t *testing.T) {
	bin := e2eBinary(t)
	h := newTmuxHelper(t)
	h.startSession("tui-test", 80, 24)

	h.sendKeys("tui-test", fmt.Sprintf("%s; sleep 999", bin), "Enter")

	found := h.waitForText("tui-test", "no sessions", 5*time.Second)
	if !found {
		t.Logf("capture:\n%s", h.capturePane("tui-test"))
	}
	require.True(t, found, "TUI should show Sessions panel")

	content := h.capturePane("tui-test")
	assert.Contains(t, content, "n: new")
	assert.Contains(t, content, "q: quit")
}

// TestE2E_ToolPopup_AutoDisplay verifies that a notification file
// triggers a popup overlay in the TUI.
func TestE2E_ToolPopup_AutoDisplay(t *testing.T) {
	bin := e2eBinary(t)
	h := newTmuxHelper(t)
	h.startSession("popup-e2e", 80, 24)

	h.sendKeys("popup-e2e", fmt.Sprintf("%s; sleep 999", bin), "Enter")

	found := h.waitForText("popup-e2e", "no sessions", 5*time.Second)
	require.True(t, found, "TUI should start")

	// Write a notification file to trigger popup
	runtimeDir := os.TempDir()
	notifPath := filepath.Join(runtimeDir, "lazyclaude-pending.json")
	notif := map[string]any{
		"tool_name": "Write",
		"input":     `{"file_path":"/tmp/test.txt"}`,
		"window":    "@0",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(notif)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(notifPath, data, 0o600))
	t.Cleanup(func() { os.Remove(notifPath) })

	// Wait for popup to appear (ticker polls every 500ms)
	found = h.waitForText("popup-e2e", "Write", 3*time.Second)
	assert.True(t, found, "tool popup should appear with tool name")

	if found {
		content := h.capturePane("popup-e2e")
		assert.Contains(t, content, "yes")
		assert.Contains(t, content, "cancel")
	}
}

// TestE2E_FullScreenMode_EnterAndExit verifies Enter/Ctrl+D toggle.
func TestE2E_FullScreenMode_EnterAndExit(t *testing.T) {
	cleanLazyClaudeState(t)
	bin := e2eBinary(t)
	h := newTmuxHelper(t)
	h.startSession("fs-test", 80, 24)

	// Start lazyclaude, create a session, enter full-screen
	h.sendKeys("fs-test", fmt.Sprintf("%s; sleep 999", bin), "Enter")

	found := h.waitForText("fs-test", "no sessions", 5*time.Second)
	require.True(t, found, "TUI should start")

	// Create a session
	h.sendKeys("fs-test", "n")
	time.Sleep(2 * time.Second) // wait for session creation

	// Enter full-screen
	h.sendKeys("fs-test", "Enter")
	time.Sleep(1 * time.Second)

	// Should show INSERT status bar
	found = h.waitForText("fs-test", "INSERT", 3*time.Second)
	assert.True(t, found, "full-screen should show INSERT mode")

	// Ctrl+D to exit
	h.sendKeys("fs-test", "C-d")
	time.Sleep(500 * time.Millisecond)

	// Should be back to split panel (has session now, so check for options bar)
	found = h.waitForText("fs-test", "n: new", 3*time.Second)
	assert.True(t, found, "should return to split panel after Ctrl+D")
}

// TestE2E_NormalMode_SwitchAndExit verifies Ctrl+\ and q.
func TestE2E_NormalMode_SwitchAndExit(t *testing.T) {
	cleanLazyClaudeState(t)
	bin := e2eBinary(t)
	h := newTmuxHelper(t)
	h.startSession("mode-test", 80, 24)

	h.sendKeys("mode-test", fmt.Sprintf("%s; sleep 999", bin), "Enter")

	found := h.waitForText("mode-test", "no sessions", 5*time.Second)
	require.True(t, found)

	// Create session and enter full-screen
	h.sendKeys("mode-test", "n")
	time.Sleep(2 * time.Second)
	h.sendKeys("mode-test", "Enter")

	found = h.waitForText("mode-test", "INSERT", 3*time.Second)
	if !found {
		t.Logf("expected INSERT, got:\n%s", h.capturePane("mode-test"))
	}
	require.True(t, found, "should show INSERT after Enter")

	// Ctrl+\ to normal mode
	// tmux send-keys interprets C-\ as Ctrl+backslash
	_, err := h.run("send-keys", "-t", "mode-test", "C-\\")
	require.NoError(t, err)

	found = h.waitForText("mode-test", "NORMAL", 3*time.Second)
	content := h.capturePane("mode-test")
	t.Logf("after Ctrl+\\:\n%s", content)
	require.True(t, found, "should switch to NORMAL mode")

	// q to exit full-screen
	h.sendKeys("mode-test", "q")

	found = h.waitForText("mode-test", "n: new", 3*time.Second)
	assert.True(t, found, "q should exit full-screen back to split panel")
}
