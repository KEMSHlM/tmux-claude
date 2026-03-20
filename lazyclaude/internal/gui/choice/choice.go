// Package choice has moved to internal/core/choice.
// This file re-exports everything for backward compatibility during migration.
package choice

import (
	"context"

	corechoice "github.com/KEMSHlM/lazyclaude/internal/core/choice"
	"github.com/KEMSHlM/lazyclaude/internal/core/config"
	"github.com/KEMSHlM/lazyclaude/internal/core/tmux"
)

// Choice re-exported from core/choice.
type Choice = corechoice.Choice

const (
	Accept = corechoice.Accept
	Allow  = corechoice.Allow
	Reject = corechoice.Reject
	Cancel = corechoice.Cancel
)

// WriteFile delegates to core/choice.WriteFile.
func WriteFile(paths config.Paths, window string, c Choice) error {
	return corechoice.WriteFile(paths, window, c)
}

// ReadFile delegates to core/choice.ReadFile.
func ReadFile(paths config.Paths, window string) (Choice, error) {
	return corechoice.ReadFile(paths, window)
}

// DetectMaxOption delegates to core/choice.DetectMaxOption.
func DetectMaxOption(paneContent string) int {
	return corechoice.DetectMaxOption(paneContent)
}

// SendToPane delegates to core/choice.SendToPane.
func SendToPane(ctx context.Context, client tmux.Client, window string, c Choice) error {
	return corechoice.SendToPane(ctx, client, window, c)
}
