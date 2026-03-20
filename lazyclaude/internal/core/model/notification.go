package model

import "time"

// Event is published on the event broker when a tool permission request arrives.
// It supplements the file-based queue for fast local in-process delivery.
type Event struct {
	Notification *ToolNotification
}

// ToolNotification represents a pending tool permission request from Claude Code.
type ToolNotification struct {
	ToolName    string    `json:"tool_name"`
	Input       string    `json:"input"`
	CWD         string    `json:"cwd,omitempty"`
	Window      string    `json:"window"`
	Timestamp   time.Time `json:"timestamp"`
	OldFilePath string    `json:"old_file_path,omitempty"` // set for Edit/Write diff
	NewContents string    `json:"new_contents,omitempty"`  // set for Edit/Write diff
	MaxOption   int       `json:"max_option,omitempty"`    // 2 or 3 (0 = unset, treat as 3)
}

// IsDiff returns true if this notification contains diff information.
func (n *ToolNotification) IsDiff() bool {
	return n.OldFilePath != ""
}
