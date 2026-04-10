package main

import (
	"sync"
)

// RemoteHostManager manages lazy connections to remote SSH hosts.
// Each host is connected at most once via sync.Once. The actual
// connection pipeline (SSH tunnel, daemon registration, SSE, mirror
// windows) is handled by the connectFn closure wired from root.go.
type RemoteHostManager struct {
	connectFn func(string) error // the root.go connectRemoteHost closure
	mu        sync.Mutex
	conns     map[string]*lazyConn
}

// lazyConn ensures a remote host is connected exactly once.
// If the initial connect fails, subsequent callers see the cached error
// without retrying (connectRemoteHost leaves no side effects on failure).
type lazyConn struct {
	once sync.Once
	err  error
}

// NewRemoteHostManager creates a RemoteHostManager with the given connect function.
func NewRemoteHostManager(connectFn func(string) error) *RemoteHostManager {
	return &RemoteHostManager{
		connectFn: connectFn,
		conns:     make(map[string]*lazyConn),
	}
}

// EnsureConnected lazily establishes a remote connection on first use.
// Returns nil if host is empty (local operation) or already connected.
// Uses sync.Once per host to guarantee exactly one connectFn call.
func (m *RemoteHostManager) EnsureConnected(host string) error {
	debugLog("RemoteHostManager.EnsureConnected: host=%q connectFn=%v", host, m.connectFn != nil)
	if host == "" || m.connectFn == nil {
		return nil
	}

	m.mu.Lock()
	lc, ok := m.conns[host]
	if !ok {
		lc = &lazyConn{}
		m.conns[host] = lc
	}
	m.mu.Unlock()

	lc.once.Do(func() {
		debugLog("RemoteHostManager.EnsureConnected: calling connectFn for host=%q", host)
		lc.err = m.connectFn(host)
		debugLog("RemoteHostManager.EnsureConnected: connectFn result: %v", lc.err)
	})
	return lc.err
}

// MarkConnected records that a host has been successfully connected via an
// external path (e.g. the connect dialog). This populates the lazyConn cache
// so that EnsureConnected skips the redundant connectFn call.
func (m *RemoteHostManager) MarkConnected(host string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	lc := &lazyConn{}
	lc.once.Do(func() {}) // mark as completed with nil error
	m.conns[host] = lc
	debugLog("RemoteHostManager.MarkConnected: host=%q cached in lazyConn", host)
}
