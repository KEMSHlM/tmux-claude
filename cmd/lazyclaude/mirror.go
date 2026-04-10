package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"

	"github.com/any-context/lazyclaude/internal/core/shell"
	"github.com/any-context/lazyclaude/internal/core/tmux"
	"github.com/any-context/lazyclaude/internal/daemon"
	"github.com/any-context/lazyclaude/internal/session"
)

// MirrorManager creates and removes local tmux mirror windows for remote
// sessions. A mirror is a local tmux window running "ssh -t <host> tmux
// attach" that lets the TUI capture-pane and send-keys through local tmux.
type MirrorManager struct {
	tmux  tmux.Client
	store *session.Store

	// onError reports errors to the GUI (wired from App).
	onError func(msg string)
	// guiUpdateFn triggers a GUI refresh from background goroutines.
	guiUpdateFn func()
}

// mirrorParams carries the fields required to create a local mirror window
// and store entry for a remote session.
type mirrorParams struct {
	ID         string
	Name       string
	Path       string // session display path (e.g. worktree path)
	TmuxWindow string // remote tmux window identifier
	Host       string
	Role       session.Role
}

// CreateMirror creates a local tmux mirror window and adds the session to
// the local store after a remote daemon API call succeeds.
// groupPath is the project root used for Store.Add grouping.
// Implements MirrorCreator.
func (m *MirrorManager) CreateMirror(host, groupPath string, resp *daemon.SessionCreateResponse) error {
	// Skip if already in local store (guards against double-click / retry).
	if m.store.FindByID(resp.ID) != nil {
		debugLog("MirrorManager.CreateMirror: session %q already in store, skipping", resp.ID)
		return nil
	}

	// Use the daemon's response path (accurate session path, e.g. worktree
	// path for [W] display). Falls back to groupPath.
	sessionPath := groupPath
	if resp.Path != "" {
		sessionPath = resp.Path
	}

	p := mirrorParams{
		ID:         resp.ID,
		Name:       resp.Name,
		Path:       sessionPath,
		TmuxWindow: resp.TmuxWindow,
		Host:       host,
		Role:       session.Role(resp.Role),
	}
	if err := m.addMirrorSession(p, groupPath); err != nil {
		return err
	}
	debugLog("MirrorManager.CreateMirror: mirror created for session %q path=%q role=%q respPath=%q", resp.ID, sessionPath, resp.Role, resp.Path)
	if m.guiUpdateFn != nil {
		m.guiUpdateFn()
	}
	return nil
}

// DeleteMirror kills the local tmux mirror window for a session.
// Implements MirrorCreator.
func (m *MirrorManager) DeleteMirror(sessionID string) error {
	mirrorName := session.MirrorWindowName(sessionID)
	return m.tmux.KillWindow(context.Background(), "lazyclaude:"+mirrorName)
}

// RestoreExisting creates mirror windows for existing remote sessions
// discovered during host connection. Skips sessions that already have a
// mirror in the local store. Returns the number of mirrors created.
func (m *MirrorManager) RestoreExisting(host string, sessions []daemon.SessionInfo) int {
	created := 0
	for _, s := range sessions {
		// Skip if already in local store (e.g. reconnection).
		if m.store.FindByID(s.ID) != nil {
			debugLog("MirrorManager.RestoreExisting: session %q already in store, skipping", s.ID)
			continue
		}

		p := mirrorParams{
			ID:         s.ID,
			Name:       s.Name,
			Path:       s.Path,
			TmuxWindow: s.TmuxWindow,
			Host:       host,
			Role:       session.Role(s.Role),
		}
		if err := m.addMirrorSession(p, s.Path); err != nil {
			debugLog("MirrorManager.RestoreExisting: failed for %q: %v", s.ID, err)
			continue
		}
		created++
	}
	return created
}

// addMirrorSession creates a local tmux mirror window for a remote session
// and adds the session to the local store under groupPath.
func (m *MirrorManager) addMirrorSession(p mirrorParams, groupPath string) error {
	mirrorName := session.MirrorWindowName(p.ID)
	if err := m.createMirrorWindow(p.Host, p.TmuxWindow, mirrorName); err != nil {
		return fmt.Errorf("create mirror window: %w", err)
	}

	sess := session.Session{
		ID:         p.ID,
		Name:       p.Name,
		Path:       p.Path,
		Host:       p.Host,
		Status:     session.StatusRunning,
		TmuxWindow: mirrorName,
		Role:       p.Role,
	}
	m.store.Add(sess, groupPath)
	if err := m.store.Save(); err != nil {
		debugLog("MirrorManager.addMirrorSession: save store failed: %v", err)
		if m.onError != nil {
			m.onError(fmt.Sprintf("save store: %v", err))
		}
	}
	debugLog("MirrorManager.addMirrorSession: mirror %q created for session %q path=%q role=%q", mirrorName, p.ID, p.Path, p.Role)
	return nil
}

// createMirrorWindow creates a local tmux window that SSH-attaches to a
// remote lazyclaude tmux session. Uses a grouped session (new-session -t)
// so that each mirror has independent window selection. The remote command
// is base64-encoded to prevent shell injection from user-controlled host strings.
func (m *MirrorManager) createMirrorWindow(host, remoteWindow, localWindowName string) error {
	debugLog("MirrorManager.createMirrorWindow: host=%q remoteWindow=%q localWindowName=%q", host, remoteWindow, localWindowName)
	remoteCmd := fmt.Sprintf(
		"tmux -L lazyclaude set-option -t lazyclaude window-size largest 2>/dev/null; "+
			"tmux -L lazyclaude new-session -t lazyclaude -s %s "+
			"\\; set-option destroy-unattached on "+
			"\\; select-window -t %s",
		shell.Quote(localWindowName),
		shell.Quote(remoteWindow),
	)

	encoded := base64.StdEncoding.EncodeToString([]byte(remoteCmd))

	sshHost, port := daemon.SplitHostPort(host)
	sshArgs := "ssh -t"
	if port != "" {
		sshArgs += " -p " + port
	}
	sshArgs += " " + sshHost
	command := fmt.Sprintf("exec %s eval \"$(echo %s | base64 -d)\"", sshArgs, encoded)

	abs, err := filepath.Abs(".")
	if err != nil {
		abs = "."
	}

	ctx := context.Background()

	// Ensure the lazyclaude tmux session exists. On a fresh start where the
	// first operation is remote (no local sessions yet), the session won't
	// exist and NewWindow would fail with "no server running".
	exists, err := m.tmux.HasSession(ctx, "lazyclaude")
	if err != nil {
		debugLog("MirrorManager.createMirrorWindow: HasSession error (non-fatal): %v", err)
	}
	if !exists {
		if err := m.tmux.NewSession(ctx, tmux.NewSessionOpts{
			Name:       "lazyclaude",
			WindowName: localWindowName,
			Command:    command,
			StartDir:   abs,
			Detached:   true,
		}); err != nil {
			return fmt.Errorf("new-session: %w", err)
		}
		return nil
	}

	return m.tmux.NewWindow(ctx, tmux.NewWindowOpts{
		Session:  "lazyclaude",
		Name:     localWindowName,
		Command:  command,
		StartDir: abs,
	})
}
