package config_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/core/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteHooksSettingsFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path, err := config.WriteHooksSettingsFile(tmp)
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(data)

	// Must not contain Go's HTML-safe unicode escapes — node can't parse them.
	assert.False(t, strings.Contains(content, `\u003e`), "must not contain \\u003e (escaped >)")
	assert.False(t, strings.Contains(content, `\u0026`), "must not contain \\u0026 (escaped &)")

	// Must contain literal JS operators that Go would normally escape.
	assert.True(t, strings.Contains(content, `=>`), "must contain literal => (arrow functions)")

	// Valid JSON
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	hooks, ok := parsed["hooks"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, hooks, "PreToolUse")
	assert.Contains(t, hooks, "Notification")
	assert.Contains(t, hooks, "Stop")
	assert.Contains(t, hooks, "SessionStart")
	assert.Contains(t, hooks, "UserPromptSubmit")
}
