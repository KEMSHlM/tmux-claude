package daemon

import (
	"fmt"
	"sync"
)

// SessionProvider abstracts a session backend for the CompositeProvider.
// Both the local adapter and remote daemon clients implement this interface.
// This mirrors gui.SessionProvider but lives in daemon to avoid import cycles.
type SessionProvider interface {
	// HasSession returns true if this provider manages the given session ID.
	HasSession(sessionID string) bool
	// Host returns the hostname for this provider ("" for local).
	Host() string
	// Sessions returns all sessions from this provider.
	Sessions() ([]SessionInfo, error)
	// Create creates a session.
	Create(path string) error
	// Delete deletes a session by ID.
	Delete(id string) error
	// Rename renames a session.
	Rename(id, newName string) error
	// PurgeOrphans removes orphaned sessions.
	PurgeOrphans() (int, error)
	// CapturePreview captures pane content.
	CapturePreview(id string, width, height int) (*PreviewResponse, error)
	// CaptureScrollback captures scrollback lines.
	CaptureScrollback(id string, width, startLine, endLine int) (*ScrollbackResponse, error)
	// HistorySize returns scrollback line count.
	HistorySize(id string) (int, error)
	// SendChoice sends a permission choice.
	SendChoice(window string, choice int) error
	// AttachSession attaches to a session interactively.
	AttachSession(id string) error
	// LaunchLazygit launches lazygit for a path.
	LaunchLazygit(path string) error
	// CreateWorktree creates a git worktree and session.
	CreateWorktree(name, prompt, projectRoot string) error
	// ResumeWorktree resumes a worktree session.
	ResumeWorktree(worktreePath, prompt, projectRoot string) error
	// ListWorktrees lists worktrees for a project root.
	ListWorktrees(projectRoot string) ([]WorktreeInfo, error)
	// CreatePMSession creates a PM session.
	CreatePMSession(projectRoot string) error
	// CreateWorkerSession creates a worker session.
	CreateWorkerSession(name, prompt, projectRoot string) error
	// ConnectionState returns the connection state. Local always returns Connected.
	ConnectionState() ConnectionState
}

// CompositeProvider merges local and remote session providers.
// The TUI interacts with this provider transparently; routing to the correct
// backend is handled internally based on host or session ID.
type CompositeProvider struct {
	mu      sync.RWMutex
	local   SessionProvider
	remotes map[string]SessionProvider // host -> provider
	router  *MessageRouter

	// staleCache holds the last known sessions from disconnected remotes.
	staleCache map[string][]SessionInfo
}

// NewCompositeProvider creates a CompositeProvider with the given local backend.
func NewCompositeProvider(local SessionProvider, router *MessageRouter) *CompositeProvider {
	return &CompositeProvider{
		local:      local,
		remotes:    make(map[string]SessionProvider),
		router:     router,
		staleCache: make(map[string][]SessionInfo),
	}
}

// AddRemote registers a remote provider.
func (c *CompositeProvider) AddRemote(host string, rp SessionProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remotes[host] = rp
}

// RemoveRemote unregisters a remote provider.
func (c *CompositeProvider) RemoveRemote(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.remotes, host)
	delete(c.staleCache, host)
}

// Sessions returns all sessions from local and remote providers merged.
// Disconnected remotes return stale cached data.
func (c *CompositeProvider) Sessions() ([]SessionInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	local, err := c.local.Sessions()
	if err != nil {
		return nil, fmt.Errorf("local sessions: %w", err)
	}
	items := make([]SessionInfo, len(local))
	copy(items, local)

	for host, rp := range c.remotes {
		if rp.ConnectionState() == Connected {
			remote, rerr := rp.Sessions()
			if rerr == nil {
				items = append(items, remote...)
				c.updateStaleCacheLocked(host, remote)
			} else {
				items = append(items, c.staleCache[host]...)
			}
		} else {
			items = append(items, c.staleCache[host]...)
		}
	}
	return items, nil
}

// Create creates a session, routing to the provider for the given host.
func (c *CompositeProvider) Create(path, host string) error {
	p, err := c.providerForHost(host)
	if err != nil {
		return err
	}
	return p.Create(path)
}

// Delete deletes a session, routing to the correct provider.
func (c *CompositeProvider) Delete(id string) error {
	p := c.providerForSession(id)
	if p == nil {
		return fmt.Errorf("no provider found for session %q", id)
	}
	return p.Delete(id)
}

// Rename renames a session, routing to the correct provider.
func (c *CompositeProvider) Rename(id, newName string) error {
	p := c.providerForSession(id)
	if p == nil {
		return fmt.Errorf("no provider found for session %q", id)
	}
	return p.Rename(id, newName)
}

// PurgeOrphans purges orphaned sessions from the local provider.
func (c *CompositeProvider) PurgeOrphans() (int, error) {
	return c.local.PurgeOrphans()
}

// CapturePreview captures pane content, routing to the correct provider.
func (c *CompositeProvider) CapturePreview(id string, width, height int) (*PreviewResponse, error) {
	p := c.providerForSession(id)
	if p == nil {
		return nil, fmt.Errorf("no provider found for session %q", id)
	}
	return p.CapturePreview(id, width, height)
}

// CaptureScrollback captures scrollback, routing to the correct provider.
func (c *CompositeProvider) CaptureScrollback(id string, width, startLine, endLine int) (*ScrollbackResponse, error) {
	p := c.providerForSession(id)
	if p == nil {
		return nil, fmt.Errorf("no provider found for session %q", id)
	}
	return p.CaptureScrollback(id, width, startLine, endLine)
}

// HistorySize returns scrollback size, routing to the correct provider.
func (c *CompositeProvider) HistorySize(id string) (int, error) {
	p := c.providerForSession(id)
	if p == nil {
		return 0, fmt.Errorf("no provider found for session %q", id)
	}
	return p.HistorySize(id)
}

// SendChoice sends a permission choice. Tries local first, then remotes.
func (c *CompositeProvider) SendChoice(window string, choice int) error {
	err := c.local.SendChoice(window, choice)
	if err == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, rp := range c.remotes {
		if rp.ConnectionState() == Connected {
			if rerr := rp.SendChoice(window, choice); rerr == nil {
				return nil
			}
		}
	}
	return err
}

// AttachSession attaches to a session, routing to the correct provider.
func (c *CompositeProvider) AttachSession(id string) error {
	p := c.providerForSession(id)
	if p == nil {
		return fmt.Errorf("no provider found for session %q", id)
	}
	return p.AttachSession(id)
}

// LaunchLazygit launches lazygit, routing by host.
func (c *CompositeProvider) LaunchLazygit(path, host string) error {
	p, err := c.providerForHost(host)
	if err != nil {
		return err
	}
	return p.LaunchLazygit(path)
}

// CreateWorktree creates a worktree, routing by host.
func (c *CompositeProvider) CreateWorktree(name, prompt, projectRoot, host string) error {
	p, err := c.providerForHost(host)
	if err != nil {
		return err
	}
	return p.CreateWorktree(name, prompt, projectRoot)
}

// ResumeWorktree resumes a worktree, routing by host.
func (c *CompositeProvider) ResumeWorktree(worktreePath, prompt, projectRoot, host string) error {
	p, err := c.providerForHost(host)
	if err != nil {
		return err
	}
	return p.ResumeWorktree(worktreePath, prompt, projectRoot)
}

// ListWorktrees lists worktrees, routing by host.
func (c *CompositeProvider) ListWorktrees(projectRoot, host string) ([]WorktreeInfo, error) {
	p, err := c.providerForHost(host)
	if err != nil {
		return nil, err
	}
	return p.ListWorktrees(projectRoot)
}

// CreatePMSession creates a PM session, routing by host.
func (c *CompositeProvider) CreatePMSession(projectRoot, host string) error {
	p, err := c.providerForHost(host)
	if err != nil {
		return err
	}
	return p.CreatePMSession(projectRoot)
}

// CreateWorkerSession creates a worker session, routing by host.
func (c *CompositeProvider) CreateWorkerSession(name, prompt, projectRoot, host string) error {
	p, err := c.providerForHost(host)
	if err != nil {
		return err
	}
	return p.CreateWorkerSession(name, prompt, projectRoot)
}

// providerForHost returns the provider for the given host.
func (c *CompositeProvider) providerForHost(host string) (SessionProvider, error) {
	if host == "" {
		return c.local, nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	rp, ok := c.remotes[host]
	if !ok {
		return nil, fmt.Errorf("no remote provider for host %q", host)
	}
	return rp, nil
}

// providerForSession returns the provider that manages the given session.
func (c *CompositeProvider) providerForSession(sessionID string) SessionProvider {
	if c.local.HasSession(sessionID) {
		return c.local
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, rp := range c.remotes {
		if rp.HasSession(sessionID) {
			return rp
		}
	}
	return nil
}

// updateStaleCacheLocked stores the latest session snapshot for a host.
// Must be called with at least a read lock held on c.mu.
func (c *CompositeProvider) updateStaleCacheLocked(host string, sessions []SessionInfo) {
	cached := make([]SessionInfo, len(sessions))
	copy(cached, sessions)
	c.staleCache[host] = cached
}
