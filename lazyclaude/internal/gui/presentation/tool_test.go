package presentation_test

import (
	"strings"
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Benchmarks ---

func BenchmarkParseToolInput_Bash(b *testing.B) {
	for b.Loop() {
		presentation.ParseToolInput("Bash", `{"command":"npm test -- --coverage"}`, "/home/user/app")
	}
}

func BenchmarkFormatToolLines(b *testing.B) {
	td := presentation.ToolDisplay{
		Name:  "Bash",
		CWD:   "/home/user/app",
		Lines: []string{"npm test -- --coverage", "echo done"},
	}
	for b.Loop() {
		presentation.FormatToolLines(td)
	}
}

func TestParseToolInput_Bash(t *testing.T) {
	t.Parallel()
	td := presentation.ParseToolInput("Bash", `{"command":"npm test -- --coverage"}`, "/home/user/app")

	assert.Equal(t, "Bash", td.Name)
	assert.Equal(t, "/home/user/app", td.CWD)
	require.Len(t, td.Lines, 1)
	assert.Equal(t, "npm test -- --coverage", td.Lines[0])
}

func TestParseToolInput_Bash_Multiline(t *testing.T) {
	t.Parallel()
	td := presentation.ParseToolInput("Bash", `{"command":"cd /tmp\nls -la"}`, "")

	require.Len(t, td.Lines, 2)
	assert.Equal(t, "cd /tmp", td.Lines[0])
	assert.Equal(t, "ls -la", td.Lines[1])
}

func TestParseToolInput_Read(t *testing.T) {
	t.Parallel()
	td := presentation.ParseToolInput("Read", `{"file_path":"/home/user/main.go"}`, "")

	require.Len(t, td.Lines, 1)
	assert.Contains(t, td.Lines[0], "/home/user/main.go")
}

func TestParseToolInput_Edit(t *testing.T) {
	t.Parallel()
	td := presentation.ParseToolInput("Edit", `{"file_path":"main.go","old_string":"hello","new_string":"world"}`, "")

	assert.Contains(t, strings.Join(td.Lines, "\n"), "main.go")
	assert.Contains(t, strings.Join(td.Lines, "\n"), "hello")
	assert.Contains(t, strings.Join(td.Lines, "\n"), "world")
}

func TestParseToolInput_Agent(t *testing.T) {
	t.Parallel()
	td := presentation.ParseToolInput("Agent", `{"description":"Research the codebase"}`, "")

	require.Len(t, td.Lines, 1)
	assert.Equal(t, "Research the codebase", td.Lines[0])
}

func TestParseToolInput_InvalidJSON(t *testing.T) {
	t.Parallel()
	td := presentation.ParseToolInput("Unknown", `{invalid`, "")

	require.Len(t, td.Lines, 1)
	assert.Equal(t, "{invalid", td.Lines[0])
}

func TestParseToolInput_Generic(t *testing.T) {
	t.Parallel()
	td := presentation.ParseToolInput("CustomTool", `{"key":"value","num":42}`, "")

	joined := strings.Join(td.Lines, "\n")
	assert.Contains(t, joined, "key")
	assert.Contains(t, joined, "value")
}

func TestFormatToolLines_Bash(t *testing.T) {
	t.Parallel()
	td := presentation.ToolDisplay{
		Name:  "Bash",
		CWD:   "/home/user/app",
		Lines: []string{"npm test"},
	}

	lines := presentation.FormatToolLines(td)

	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "Command:")
	assert.Contains(t, joined, "npm test") // content present (may have ANSI around $)
	assert.Contains(t, joined, "/home/user/app")
}

func TestFormatToolLines_NoCWD(t *testing.T) {
	t.Parallel()
	td := presentation.ToolDisplay{
		Name:  "Read",
		Lines: []string{"File: main.go"},
	}

	lines := presentation.FormatToolLines(td)
	joined := strings.Join(lines, "\n")
	assert.NotContains(t, joined, "/home") // no CWD path when not set
}

func TestFormatToolLines_NonBash(t *testing.T) {
	t.Parallel()
	td := presentation.ToolDisplay{
		Name:  "Edit",
		Lines: []string{"File: main.go", "Old:", "  - hello"},
	}

	lines := presentation.FormatToolLines(td)
	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "File: main.go")
	// Non-bash lines get "  " prefix, not "  $ "
	assert.NotContains(t, joined, "$ File")
}
