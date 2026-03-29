package gui

import "context"

// PluginItem is a read-only view of an installed plugin for display.
type PluginItem struct {
	ID          string
	Version     string
	Scope       string // "project"
	Enabled     bool
	InstalledAt string // ISO 8601
}

// AvailablePluginItem is a read-only view of a marketplace plugin for display.
type AvailablePluginItem struct {
	PluginID        string
	Name            string
	Description     string
	MarketplaceName string
	InstallCount    int
}

// PluginProvider abstracts plugin operations for the GUI layer.
type PluginProvider interface {
	Refresh(ctx context.Context) error
	Installed() []PluginItem
	Available() []AvailablePluginItem
	Install(ctx context.Context, pluginID string) error
	Uninstall(ctx context.Context, pluginID string) error
	ToggleEnabled(ctx context.Context, pluginID string) error
	Update(ctx context.Context, pluginID string) error
}

// PluginState holds the UI state for the plugins panel.
type PluginState struct {
	tabIdx          int // 0=Installed, 1=Marketplace
	installedCursor int
	marketCursor    int
	loading         bool
	errMsg          string
}

// NewPluginState creates a new PluginState.
func NewPluginState() *PluginState {
	return &PluginState{}
}

// Cursor returns the cursor for the active tab.
func (ps *PluginState) Cursor() int {
	if ps.tabIdx == 1 {
		return ps.marketCursor
	}
	return ps.installedCursor
}

// SetCursor sets the cursor for the active tab.
func (ps *PluginState) SetCursor(n int) {
	if ps.tabIdx == 1 {
		ps.marketCursor = n
	} else {
		ps.installedCursor = n
	}
}
