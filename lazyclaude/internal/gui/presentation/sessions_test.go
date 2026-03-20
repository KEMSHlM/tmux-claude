package presentation_test

import (
	"testing"
	"time"

	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/KEMSHlM/lazyclaude/internal/session"
	"github.com/stretchr/testify/assert"
)

func sess(name string, status session.Status, host string, flags ...string) session.Session {
	return session.Session{
		ID:        "test-id",
		Name:      name,
		Host:      host,
		Status:    status,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Flags:     flags,
	}
}

func TestFormatSessionLine_Running(t *testing.T) {
	t.Parallel()
	s := sess("my-app", session.StatusRunning, "")
	line := presentation.FormatSessionLine(s, 40)

	assert.Contains(t, line, "my-app")
	assert.Contains(t, line, "●") // green filled circle
}

func TestFormatSessionLine_Dead(t *testing.T) {
	t.Parallel()
	s := sess("my-app", session.StatusDead, "")
	line := presentation.FormatSessionLine(s, 40)

	assert.Contains(t, line, "×") // red cross
}

func TestFormatSessionLine_Orphan(t *testing.T) {
	t.Parallel()
	s := sess("orphaned", session.StatusOrphan, "")
	line := presentation.FormatSessionLine(s, 40)

	assert.Contains(t, line, "○") // yellow empty circle
}

func TestFormatSessionLine_Detached(t *testing.T) {
	t.Parallel()
	s := sess("idle", session.StatusDetached, "")
	line := presentation.FormatSessionLine(s, 40)

	assert.Contains(t, line, "◆") // gray diamond
}

func TestFormatSessionLine_Unknown(t *testing.T) {
	t.Parallel()
	s := sess("new", session.StatusUnknown, "")
	line := presentation.FormatSessionLine(s, 40)

	assert.Contains(t, line, "?")
}

func TestFormatSessionLine_Remote(t *testing.T) {
	t.Parallel()
	s := sess("work", session.StatusRunning, "srv1")
	line := presentation.FormatSessionLine(s, 40)

	assert.Contains(t, line, "srv1:work")
	assert.Contains(t, line, "●")
}

func TestFormatSessionLine_WithFlags(t *testing.T) {
	t.Parallel()
	s := sess("my-app", session.StatusRunning, "", "--resume")
	line := presentation.FormatSessionLine(s, 40)

	assert.Contains(t, line, "R")
	assert.Contains(t, line, "●")
}

func TestFormatSessionLine_TruncateLongName(t *testing.T) {
	t.Parallel()
	s := sess("very-long-project-name-that-exceeds-width", session.StatusRunning, "")
	line := presentation.FormatSessionLine(s, 30)

	// Status icons contain ANSI escapes, so byte length exceeds display width.
	// Just verify truncation marker is present and name is shortened.
	assert.Contains(t, line, "~") // truncation marker
	assert.NotContains(t, line, "very-long-project-name-that-exceeds-width")
}

func TestFormatSessionLines(t *testing.T) {
	t.Parallel()
	sessions := []session.Session{
		sess("app", session.StatusRunning, ""),
		sess("lib", session.StatusDetached, ""),
		sess("work", session.StatusRunning, "srv1"),
	}

	lines := presentation.FormatSessionLines(sessions, 40)
	assert.Len(t, lines, 3)
	assert.Contains(t, lines[0], "app")
	assert.Contains(t, lines[1], "lib")
	assert.Contains(t, lines[2], "srv1:work")
}

func TestFormatSessionLine_NarrowWidth(t *testing.T) {
	t.Parallel()
	s := sess("app", session.StatusRunning, "")
	line := presentation.FormatSessionLine(s, 10)

	// Should not panic, should produce some output
	assert.NotEmpty(t, line)
}

func TestServerStatusLine(t *testing.T) {
	t.Parallel()
	line := presentation.ServerStatusLine(7860, 2, "3h 24m")

	assert.Contains(t, line, ":7860")
	assert.Contains(t, line, "2")
	assert.Contains(t, line, "3h 24m")
}
