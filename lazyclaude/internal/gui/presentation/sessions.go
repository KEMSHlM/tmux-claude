package presentation

import (
	"fmt"
	"strings"

	"github.com/KEMSHlM/lazyclaude/internal/session"
)

// FormatSessionLine renders a single session line for the list view.
// Format: "name          status_indicator flags"
func FormatSessionLine(s session.Session, maxWidth int) string {
	status := statusIndicator(s.Status)
	flags := formatFlags(s.Flags)

	// Build right-side indicators
	right := strings.TrimSpace(status + " " + flags)

	// Calculate padding
	nameWidth := maxWidth - len(right) - 2 // 2 for spacing
	if nameWidth < 5 {
		nameWidth = 5
	}

	name := s.Name
	if s.Host != "" {
		name = s.Host + ":" + name
	}

	// Truncate name if needed
	if len(name) > nameWidth {
		name = name[:nameWidth-1] + "~"
	}

	if right == "" {
		return name
	}
	padding := nameWidth - len(name)
	if padding < 1 {
		padding = 1
	}
	return name + strings.Repeat(" ", padding) + right
}

// FormatSessionLines renders all sessions for the list view.
func FormatSessionLines(sessions []session.Session, maxWidth int) []string {
	lines := make([]string, len(sessions))
	for i, s := range sessions {
		lines[i] = FormatSessionLine(s, maxWidth)
	}
	return lines
}

func statusIndicator(s session.Status) string {
	switch s {
	case session.StatusRunning:
		return IconRunning
	case session.StatusDead:
		return IconDead
	case session.StatusOrphan:
		return IconOrphan
	case session.StatusDetached:
		return IconDetached
	default:
		return IconUnknown
	}
}

func formatFlags(flags []string) string {
	var parts []string
	for _, f := range flags {
		switch f {
		case "--resume":
			parts = append(parts, "R")
		default:
			// skip unknown flags
		}
	}
	return strings.Join(parts, "")
}

// ServerStatusLine formats a server status line.
func ServerStatusLine(port int, connCount int, uptime string) string {
	return fmt.Sprintf("MCP: listening :%d  |  Connections: %d  |  %s", port, connCount, uptime)
}
